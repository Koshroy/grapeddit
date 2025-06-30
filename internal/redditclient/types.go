package redditclient

import (
	"compress/gzip"
	"context"
	"net/http"
	"sync"
)

// HTTPClient interface for dependency injection
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RedditClient interface for testability
type RedditClient interface {
	Authenticate(ctx context.Context) error
	GetSubreddit(ctx context.Context, subreddit, sort string) (*SubredditListing, error)
	GetPost(ctx context.Context, subreddit, postID string) (*PostResponse, error)
	GetUser(ctx context.Context, username string) (*UserResponse, error)
	Search(ctx context.Context, query, sort, timeframe string) (*SearchResponse, error)
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