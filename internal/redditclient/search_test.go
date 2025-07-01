package redditclient

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSearch_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	searchResponse := SearchResponse{
		Kind: "Listing",
		Data: struct {
			Children []struct {
				Kind string `json:"kind"`
				Data Post   `json:"data"`
			} `json:"children"`
		}{
			Children: []struct {
				Kind string `json:"kind"`
				Data Post   `json:"data"`
			}{
				{
					Kind: "t3",
					Data: Post{
						ID:    "search123",
						Title: "Search Result",
						Score: 25,
					},
				},
			},
		},
	}
	responseBody, _ := json.Marshal(searchResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		u, _ := url.Parse(req.URL.String())
		params := u.Query()
		return strings.Contains(req.URL.String(), "/search.json") &&
			params.Get("q") == "golang" &&
			params.Get("sort") == "top" &&
			params.Get("t") == "week"
	})).Return(createHTTPResponse(200, string(responseBody), nil), nil)

	result, err := client.Search(t.Context(), "golang", "top", "week")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Data.Children, 1)
	assert.Equal(t, "Search Result", result.Data.Children[0].Data.Title)
	mockHTTP.AssertExpectations(t)
}

func TestSearch_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.Search(t.Context(), "golang", "top", "week")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}