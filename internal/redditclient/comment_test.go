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

func TestGetComments_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Reddit comments endpoint returns a two-element array: [post_listing, comment_listing]
	commentsResponse := [2]interface{}{
		// Element 0: Post listing
		map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{
					map[string]interface{}{
						"kind": "t3",
						"data": map[string]interface{}{
							"id":           "abc123",
							"title":        "Test Post",
							"author":       "postauthor",
							"selftext":     "This is the post body",
							"score":        42,
							"num_comments": 5,
						},
					},
				},
			},
		},
		// Element 1: Comment listing
		map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{
					map[string]interface{}{
						"kind": "t1",
						"data": map[string]interface{}{
							"id":        "comment1",
							"author":    "commenter",
							"body":      "Great post!",
							"score":     5,
							"parent_id": "t3_abc123",
							"replies": map[string]any{
								"kind": "Listing",
								"data": map[string]interface{}{
									"children": []interface{}{
										map[string]interface{}{
											"kind": "t1",
											"data": map[string]interface{}{
												"id":        "comment2",
												"author":    "commenter2",
												"body":      "Greater post!",
												"score":     5,
												"parent_id": "t1_comment1",
												"replies":   map[string][]any{},
											},
										},
									},
								},
							},
						},
					},
					map[string]interface{}{
						"kind": "more",
						"data": map[string]interface{}{
							"count":     10,
							"id":        "more123",
							"parent_id": "t3_abc123",
							"children":  []string{"child1", "child2"},
						},
					},
				},
			},
		},
	}
	responseBody, _ := json.Marshal(commentsResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// Check for sort parameter
		u, _ := url.Parse(req.URL.String())
		params := u.Query()
		return strings.Contains(req.URL.String(), "/r/golang/comments/abc123.json") &&
			params.Get("sort") == "top"
	})).Return(createHTTPResponse(200, string(responseBody), nil), nil)

	result, err := client.GetComments(t.Context(), "golang", "abc123", "top")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, *result, 2) // Post data + comments data
	mockHTTP.AssertExpectations(t)
}

func TestGetComments_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.GetComments(t.Context(), "golang", "abc123", "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}

func TestGetMoreComments_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Response from /api/morechildren endpoint
	moreResponse := map[string]interface{}{
		"json": map[string]interface{}{
			"errors": []interface{}{},
			"data": map[string]interface{}{
				"things": []interface{}{
					map[string]interface{}{
						"kind": "t1",
						"data": map[string]interface{}{
							"id":        "extracomment1",
							"author":    "anotheruser",
							"body":      "Additional comment loaded",
							"score":     3,
							"parent_id": "t3_abc123",
						},
					},
				},
			},
		},
	}
	responseBody, _ := json.Marshal(moreResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		u, _ := url.Parse(req.URL.String())
		params := u.Query()
		return strings.Contains(req.URL.String(), "/api/morechildren.json") &&
			params.Get("link_id") == "t3_abc123" &&
			params.Get("api_type") == "json"
	})).Return(createHTTPResponse(200, string(responseBody), nil), nil)

	result, err := client.GetMoreComments(t.Context(), "t3_abc123", []string{"child1", "child2"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.JSON.Errors)
	assert.Len(t, result.JSON.Data.Things, 1)
	mockHTTP.AssertExpectations(t)
}

func TestGetMoreComments_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.GetMoreComments(t.Context(), "t3_abc123", []string{"child1"})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}
