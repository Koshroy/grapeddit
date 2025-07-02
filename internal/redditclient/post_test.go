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

func TestGetPost_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	postResponse := PostResponse{
		Kind: "Listing",
		Data: struct {
			Children []struct {
				Kind string `json:"kind"`
				Data any    `json:"data"`
			} `json:"children"`
		}{
			Children: []struct {
				Kind string `json:"kind"`
				Data any    `json:"data"`
			}{
				{
					Kind: "t3",
					Data: map[string]any{
						"id":    "abc123",
						"title": "Test Post",
					},
				},
			},
		},
	}
	responseBody, _ := json.Marshal(postResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "/r/golang/comments/abc123.json")
	})).Return(createHTTPResponse(200, string(responseBody), nil), nil)

	result, err := client.GetPost(t.Context(), "golang", "abc123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Listing", result.Kind)
	mockHTTP.AssertExpectations(t)
}

func TestGetPost_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.GetPost(t.Context(), "golang", "abc123")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}
