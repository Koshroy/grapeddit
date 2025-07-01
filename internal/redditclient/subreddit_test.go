package redditclient

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetSubreddit_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true
	client.loid = "test-loid"
	client.session = "test-session"

	listing := SubredditListing{
		Kind: "Listing",
		Data: struct {
			Children []struct {
				Kind string `json:"kind"`
				Data Post   `json:"data"`
			} `json:"children"`
			After  string `json:"after"`
			Before string `json:"before"`
		}{
			Children: []struct {
				Kind string `json:"kind"`
				Data Post   `json:"data"`
			}{
				{
					Kind: "t3",
					Data: Post{
						ID:        "test123",
						Title:     "Test Post",
						Author:    "testuser",
						Subreddit: "golang",
						Score:     42,
					},
				},
			},
		},
	}
	responseBody, _ := json.Marshal(listing)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "/r/golang/hot.json") &&
			req.Header.Get("Authorization") == "Bearer test-token" &&
			req.Header.Get("x-reddit-loid") == "test-loid" &&
			req.Header.Get("x-reddit-session") == "test-session"
	})).Return(createHTTPResponse(200, string(responseBody), map[string]string{
		"x-ratelimit-remaining": "50",
	}), nil)

	result, err := client.GetSubreddit(t.Context(), "golang", "hot")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Listing", result.Kind)
	assert.Len(t, result.Data.Children, 1)
	assert.Equal(t, "Test Post", result.Data.Children[0].Data.Title)
	mockHTTP.AssertExpectations(t)
}

func TestGetSubreddit_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.GetSubreddit(t.Context(), "golang", "hot")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}

func TestHandleRestrictedContent_Gated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// First response with gated content
	gatedResponse := `{"reason": "gated"}`
	// Second response with actual content
	actualContent := `{"kind": "Listing", "data": {"children": []}}`

	// Mock the initial request that returns gated content
	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return !strings.Contains(req.Header.Get("Cookie"), "pref_gated_sr_optin")
	})).Return(createHTTPResponse(200, gatedResponse, nil), nil).Once()

	// Mock the retry request with cookie
	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.Header.Get("Cookie"), "pref_gated_sr_optin")
	})).Return(createHTTPResponse(200, actualContent, nil), nil).Once()

	result, err := client.GetSubreddit(t.Context(), "gatedsubreddit", "hot")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockHTTP.AssertExpectations(t)
}

func TestHandleRestrictedContent_Private(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	privateResponse := `{"reason": "private"}`

	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(200, privateResponse, nil), nil)

	result, err := client.GetSubreddit(t.Context(), "privatesubreddit", "hot")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "content is private")
	mockHTTP.AssertExpectations(t)
}
