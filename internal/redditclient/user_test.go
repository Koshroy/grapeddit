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

func TestGetUser_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	userResponse := UserResponse{
		Kind: "t2",
		Data: struct {
			Name         string  `json:"name"`
			LinkKarma    int     `json:"link_karma"`
			CommentKarma int     `json:"comment_karma"`
			Created      float64 `json:"created_utc"`
		}{
			Name:         "testuser",
			LinkKarma:    100,
			CommentKarma: 200,
			Created:      1640995200.0,
		},
	}
	responseBody, _ := json.Marshal(userResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.String(), "/user/testuser/about.json")
	})).Return(createHTTPResponse(200, string(responseBody), nil), nil)

	result, err := client.GetUser(t.Context(), "testuser")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "testuser", result.Data.Name)
	assert.Equal(t, 100, result.Data.LinkKarma)
	assert.Equal(t, 200, result.Data.CommentKarma)
	mockHTTP.AssertExpectations(t)
}

func TestGetUser_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.GetUser(t.Context(), "testuser")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}
