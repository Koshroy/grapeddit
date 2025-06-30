package redditclient

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	ANDROID_CLIENT_ID             = "ohXpoqrZYub1kg"
	CONTENT_WARNING_ACCEPT_COOKIE = "_options=%7B%22pref_quarantine_optin%22%3A%20true%2C%20%22pref_gated_sr_optin%22%3A%20true%7D"
)

// HTTPClient interface for dependency injection
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RedditClient interface for testability
type RedditClient interface {
	Authenticate() error
	GetSubreddit(subreddit, sort string) (*SubredditListing, error)
	GetPost(subreddit, postID string) (*PostResponse, error)
	GetUser(username string) (*UserResponse, error)
	Search(query, sort, timeframe string) (*SearchResponse, error)
}

// Client implements RedditClient
type Client struct {
	httpClient    HTTPClient
	accessToken   string
	loid          string
	session       string
	deviceID      string
	userAgent     string
	rateLimitLock sync.RWMutex
	rateLimit     int
	gzipReader    *gzip.Reader
	gzipMutex     sync.Mutex
}

// OAuth response structures
type OAuthResponse struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ExpiresIn   int      `json:"expires_in"`
	Scope       []string `json:"scope"`
}

// API response structures
type SubredditListing struct {
	Kind string `json:"kind"`
	Data struct {
		Children []struct {
			Kind string `json:"kind"`
			Data Post   `json:"data"`
		} `json:"children"`
		After  string `json:"after"`
		Before string `json:"before"`
	} `json:"data"`
}

type Post struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	Score       int     `json:"score"`
	URL         string  `json:"url"`
	SelfText    string  `json:"selftext"`
	NumComments int     `json:"num_comments"`
	Created     float64 `json:"created_utc"`
}

type PostResponse struct {
	Kind string `json:"kind"`
	Data struct {
		Children []struct {
			Kind string      `json:"kind"`
			Data interface{} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type UserResponse struct {
	Kind string `json:"kind"`
	Data struct {
		Name         string  `json:"name"`
		LinkKarma    int     `json:"link_karma"`
		CommentKarma int     `json:"comment_karma"`
		Created      float64 `json:"created_utc"`
	} `json:"data"`
}

type SearchResponse struct {
	Kind string `json:"kind"`
	Data struct {
		Children []struct {
			Kind string `json:"kind"`
			Data Post   `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type ErrorResponse struct {
	Reason string `json:"reason"`
}

// Android app versions for User-Agent spoofing
var androidVersions = []string{
	"Reddit/2023.46.0/Android 12",
	"Reddit/2023.45.0/Android 11",
	"Reddit/2023.44.0/Android 13",
	"Reddit/2023.43.0/Android 12",
	"Reddit/2023.42.0/Android 11",
}

// NewClient creates a new Reddit client
func NewClient(httpClient HTTPClient) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	deviceID := uuid.New().String()
	userAgent := androidVersions[rand.Intn(len(androidVersions))]

	return &Client{
		httpClient: httpClient,
		deviceID:   deviceID,
		userAgent:  userAgent,
		rateLimit:  100, // Start with assumed full rate limit
	}, nil
}

// Authenticate performs OAuth authentication
func (c *Client) Authenticate() error {
	// OAuth Client ID for Reddit Android app
	auth := base64.StdEncoding.EncodeToString([]byte(ANDROID_CLIENT_ID + ":"))

	body := map[string]interface{}{
		"scopes": []string{"*", "email", "pii"},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to unmarshal json response body: %w", err)
	}

	req, err := http.NewRequest("POST", "https://www.reddit.com/auth/v2/oauth/access-token/loid", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Required headers for Android app spoofing
	headers := map[string]string{
		"Authorization":         "Basic " + auth,
		"User-Agent":            c.userAgent,
		"X-Reddit-Device-Id":    c.deviceID,
		"client-vendor-id":      c.deviceID,
		"Content-Type":          "application/json; charset=UTF-8",
		"x-reddit-retry":        "algo=no-retries",
		"x-reddit-compression":  "1",
		"x-reddit-qos":          fmt.Sprintf("%.3f", rand.Float64()*100),
		"x-reddit-media-codecs": "available-codecs=video/avc, video/hevc, video/x-vnd.on2.vp9",
	}

	c.shuffleHeaders(req, headers)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	var oauthResp OAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&oauthResp); err != nil {
		return fmt.Errorf("failed to decode OAuth response: %w", err)
	}

	c.accessToken = oauthResp.AccessToken
	c.loid = resp.Header.Get("x-reddit-loid")
	c.session = resp.Header.Get("x-reddit-session")

	return nil
}

// shuffleHeaders randomizes header order for anti-fingerprinting
func (c *Client) shuffleHeaders(req *http.Request, headers map[string]string) {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}

	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	for _, k := range keys {
		req.Header.Set(k, headers[k])
	}
}

// readResponseBody reads and decompresses response body if gzipped
func (c *Client) readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	// Check if response is gzip encoded
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		c.gzipMutex.Lock()
		defer c.gzipMutex.Unlock()

		// Lazy initialization: create gzip reader only when first needed
		if c.gzipReader == nil {
			var err error
			c.gzipReader, err = gzip.NewReader(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to create gzip reader: %w", err)
			}
		} else {
			// Reuse existing gzip reader by resetting it with new input
			if err := c.gzipReader.Reset(resp.Body); err != nil {
				return nil, fmt.Errorf("failed to reset gzip reader: %w", err)
			}
		}
		reader = c.gzipReader
	}

	return io.ReadAll(reader)
}

// makeAPIRequest handles common API request logic
func (c *Client) makeAPIRequest(endpoint string, params url.Values) ([]byte, error) {
	if c.accessToken == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	if params == nil {
		params = url.Values{}
	}
	params.Set("raw_json", "1")

	fullURL := "https://oauth.reddit.com" + endpoint
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	headers := map[string]string{
		"Authorization":    "Bearer " + c.accessToken,
		"User-Agent":       c.userAgent,
		"x-reddit-loid":    c.loid,
		"x-reddit-session": c.session,
		"Accept-Encoding":  "gzip",
	}

	c.shuffleHeaders(req, headers)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check rate limit
	if rateLimit := resp.Header.Get("x-ratelimit-remaining"); rateLimit != "" {
		c.updateRateLimit(rateLimit)
	}

	body, err := c.readResponseBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check for restricted content errors
	var errorResp ErrorResponse
	if json.Unmarshal(body, &errorResp) == nil && errorResp.Reason != "" {
		return c.handleRestrictedContent(req, errorResp.Reason)
	}

	return body, nil
}

// handleRestrictedContent handles gated/quarantined content
func (c *Client) handleRestrictedContent(originalReq *http.Request, reason string) ([]byte, error) {
	switch reason {
	case "gated", "quarantined":
		// Retry with cookie to accept content warning
		originalReq.Header.Set("Cookie", CONTENT_WARNING_ACCEPT_COOKIE)

		resp, err := c.httpClient.Do(originalReq)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := c.readResponseBody(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to read retry response: %w", err)
		}

		return body, nil

	case "private":
		return nil, fmt.Errorf("content is private and cannot be accessed")

	default:
		return nil, fmt.Errorf("unknown content restriction: %s", reason)
	}
}

// updateRateLimit updates the rate limit counter
func (c *Client) updateRateLimit(rateLimitStr string) {
	// Implementation would parse the rate limit and potentially refresh token
	// if below threshold (10 as mentioned in the analysis)
	c.rateLimitLock.Lock()
	defer c.rateLimitLock.Unlock()
	// Simplified implementation - in production would parse the actual value
	c.rateLimit--
	if c.rateLimit < 10 {
		// Would trigger background token refresh
		c.rateLimit = 100 // Reset for this example
	}
}

// GetSubreddit fetches subreddit listings
func (c *Client) GetSubreddit(subreddit, sort string) (*SubredditListing, error) {
	endpoint := fmt.Sprintf("/r/%s/%s.json", subreddit, sort)

	body, err := c.makeAPIRequest(endpoint, nil)
	if err != nil {
		return nil, err
	}

	var listing SubredditListing
	if err := json.Unmarshal(body, &listing); err != nil {
		log.Printf("failed to decode subreddit listing %s: %v", string(body), err)
		return nil, fmt.Errorf("failed to decode subreddit listing: %w", err)
	}

	return &listing, nil
}

// GetPost fetches a specific post and comments
func (c *Client) GetPost(subreddit, postID string) (*PostResponse, error) {
	endpoint := fmt.Sprintf("/r/%s/comments/%s.json", subreddit, postID)

	body, err := c.makeAPIRequest(endpoint, nil)
	if err != nil {
		return nil, err
	}

	var post PostResponse
	if err := json.Unmarshal(body, &post); err != nil {
		return nil, fmt.Errorf("failed to decode post: %w", err)
	}

	return &post, nil
}

// GetUser fetches user information
func (c *Client) GetUser(username string) (*UserResponse, error) {
	endpoint := fmt.Sprintf("/user/%s/about.json", username)

	body, err := c.makeAPIRequest(endpoint, nil)
	if err != nil {
		return nil, err
	}

	var user UserResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// Search performs a Reddit search
func (c *Client) Search(query, sort, timeframe string) (*SearchResponse, error) {
	params := url.Values{
		"q":    []string{query},
		"sort": []string{sort},
		"t":    []string{timeframe},
	}

	body, err := c.makeAPIRequest("/search.json", params)
	if err != nil {
		return nil, err
	}

	var search SearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return &search, nil
}
