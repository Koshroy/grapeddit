package redditclient

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// Error variables
var (
	ErrNotAuthenticated = errors.New("client is not authenticated - call Authenticate() first")
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
	GetComments(ctx context.Context, subreddit, postID string, sort string) (*PostAndCommentsResponse, error)
	GetMoreComments(ctx context.Context, linkID string, children []string) (*MoreCommentsResponse, error)
}

// Client implements RedditClient
type Client struct {
	httpClient     HTTPClient
	authenticated  bool
	accessToken    string
	loid           string
	session        string
	deviceID       string
	userAgent      string
	rateLimitLock  sync.RWMutex
	rateLimit      int
	gzipReaderPool sync.Pool
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

// PostResponse represents a single post response
type PostResponse struct {
	Kind string `json:"kind"`
	Data struct {
		Children []struct {
			Kind string      `json:"kind"`
			Data interface{} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// PostListing represents a listing containing posts
type PostListing struct {
	Kind string `json:"kind"`
	Data struct {
		Children []struct {
			Kind string `json:"kind"`
			Data Post   `json:"data"`
		} `json:"children"`
		After  *string `json:"after"`
		Before *string `json:"before"`
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

// Comment represents a Reddit comment (t1)
type Comment struct {
	ID       string      `json:"id"`
	Author   string      `json:"author"`
	Body     string      `json:"body"`
	Score    int         `json:"score"`
	Created  float64     `json:"created_utc"`
	ParentID string      `json:"parent_id"`
	Replies  interface{} `json:"replies"` // Can be empty string "" or CommentListing
}

// MoreComments represents a "more comments" placeholder (more)
type MoreComments struct {
	Count    int      `json:"count"`
	ID       string   `json:"id"`
	ParentID string   `json:"parent_id"`
	Children []string `json:"children"`
}

// CommentChild represents a child in a comment listing (can be t1 or more)
type CommentChild struct {
	Kind string      `json:"kind"`
	Data interface{} `json:"data"` // Comment for t1, MoreComments for more
}

// CommentListing represents a listing of comments
type CommentListing struct {
	Kind string `json:"kind"`
	Data struct {
		Children []CommentChild `json:"children"`
		After    *string        `json:"after"`
		Before   *string        `json:"before"`
	} `json:"data"`
}

// PostAndCommentsResponse represents the two-element array returned by Reddit
// Element 0: Post listing (contains the post)
// Element 1: Comment listing (contains comments)
type PostAndCommentsResponse [2]interface{}

// MoreCommentsResponse represents the response from /api/morechildren
type MoreCommentsResponse struct {
	JSON struct {
		Errors []interface{} `json:"errors"`
		Data   struct {
			Things []CommentChild `json:"things"`
		} `json:"data"`
	} `json:"json"`
}

type ErrorResponse struct {
	Reason string `json:"reason"`
}
