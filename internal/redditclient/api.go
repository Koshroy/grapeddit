package redditclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
)

// GetSubreddit fetches subreddit listings
func (c *Client) GetSubreddit(ctx context.Context, subreddit, sort string) (*SubredditListing, error) {
	if !c.authenticated {
		return nil, ErrNotAuthenticated
	}

	endpoint := fmt.Sprintf("/r/%s/%s.json", subreddit, sort)

	body, err := c.makeAPIRequest(ctx, endpoint, nil)
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
func (c *Client) GetPost(ctx context.Context, subreddit, postID string) (*PostResponse, error) {
	if !c.authenticated {
		return nil, ErrNotAuthenticated
	}

	endpoint := fmt.Sprintf("/r/%s/comments/%s.json", subreddit, postID)

	body, err := c.makeAPIRequest(ctx, endpoint, nil)
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
func (c *Client) GetUser(ctx context.Context, username string) (*UserResponse, error) {
	if !c.authenticated {
		return nil, ErrNotAuthenticated
	}

	endpoint := fmt.Sprintf("/user/%s/about.json", username)

	body, err := c.makeAPIRequest(ctx, endpoint, nil)
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
func (c *Client) Search(ctx context.Context, query, sort, timeframe string) (*SearchResponse, error) {
	if !c.authenticated {
		return nil, ErrNotAuthenticated
	}

	params := url.Values{
		"q":    []string{query},
		"sort": []string{sort},
		"t":    []string{timeframe},
	}

	body, err := c.makeAPIRequest(ctx, "/search.json", params)
	if err != nil {
		return nil, err
	}

	var search SearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return &search, nil
}

// GetComments fetches post and comments with optional sorting
// sort can be: confidence, top, new, controversial, old, qa
func (c *Client) GetComments(ctx context.Context, subreddit, postID string, sort string) (*PostAndCommentsResponse, error) {
	if !c.authenticated {
		return nil, ErrNotAuthenticated
	}

	endpoint := fmt.Sprintf("/r/%s/comments/%s.json", subreddit, postID)

	params := url.Values{}
	if sort != "" {
		params.Set("sort", sort)
	}

	body, err := c.makeAPIRequest(ctx, endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("error making GetComments API request: %w", err)
	}

	var response PostAndCommentsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode post and comments: %w", err)
	}

	return &response, nil
}

// GetMoreComments fetches additional comments using the morechildren API
func (c *Client) GetMoreComments(ctx context.Context, linkID string, children []string) (*MoreCommentsResponse, error) {
	if !c.authenticated {
		return nil, ErrNotAuthenticated
	}

	params := url.Values{
		"api_type": []string{"json"},
		"link_id":  []string{linkID},
	}

	// Add children IDs (Reddit expects comma-separated string)
	if len(children) > 0 {
		for _, child := range children {
			params.Add("children", child)
		}
	}

	body, err := c.makeAPIRequest(ctx, "/api/morechildren.json", params)
	if err != nil {
		return nil, err
	}

	var response MoreCommentsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode more comments: %w", err)
	}

	return &response, nil
}
