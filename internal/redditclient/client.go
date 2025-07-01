package redditclient

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewClient creates a new Reddit client
func NewClient(httpClient HTTPClient) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	deviceID := uuid.New().String()
	userAgent := androidVersions[rand.Intn(len(androidVersions))]

	return &Client{
		httpClient:    httpClient,
		authenticated: false,
		deviceID:      deviceID,
		userAgent:     userAgent,
		rateLimit:     100, // Start with assumed full rate limit
		gzipReaderPool: sync.Pool{
			New: func() interface{} {
				// Return nil - we'll create the gzip reader on first use
				// This works because sync.Pool.New may return nil at any time
				// Calling code must check for nil before using the reader anyway so
				// we lazily create the reader.
				return nil
			},
		},
	}, nil
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
		// Get a gzip reader from the pool
		poolItem := c.gzipReaderPool.Get()
		var gr *gzip.Reader

		if poolItem == nil {
			// Create a new gzip reader if pool is empty
			var err error
			gr, err = gzip.NewReader(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to create gzip reader: %w", err)
			}
		} else {
			// Reuse existing gzip reader from pool
			gr = poolItem.(*gzip.Reader)
			if err := gr.Reset(resp.Body); err != nil {
				return nil, fmt.Errorf("failed to reset gzip reader: %w", err)
			}
		}

		// Read all data
		data, err := io.ReadAll(gr)

		// Return the reader to the pool for reuse
		c.gzipReaderPool.Put(gr)

		return data, err
	}

	return io.ReadAll(reader)
}

// makeAPIRequest handles common API request logic
func (c *Client) makeAPIRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
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

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
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
		return c.handleRestrictedContent(ctx, req, errorResp.Reason)
	}

	return body, nil
}

// handleRestrictedContent handles gated/quarantined content
func (c *Client) handleRestrictedContent(ctx context.Context, originalReq *http.Request, reason string) ([]byte, error) {
	switch reason {
	case "gated", "quarantined":
		// Create a new request with the same context to avoid modifying the original
		retryReq, err := http.NewRequestWithContext(ctx, originalReq.Method, originalReq.URL.String(), originalReq.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create retry request: %w", err)
		}

		// Copy headers from original request
		for k, v := range originalReq.Header {
			retryReq.Header[k] = v
		}

		// Add cookie to accept content warning
		retryReq.Header.Set("Cookie", CONTENT_WARNING_ACCEPT_COOKIE)

		resp, err := c.httpClient.Do(retryReq)
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
