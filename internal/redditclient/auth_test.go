package redditclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_Success(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	oauthResponse := OAuthResponse{
		AccessToken: "test-token",
		TokenType:   "bearer",
		ExpiresIn:   3600,
		Scope:       []string{"*", "email", "pii"},
	}
	responseBody, _ := json.Marshal(oauthResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// Verify the request has the test context
		assert.Equal(t, t.Context(), req.Context())
		return req.URL.String() == "https://www.reddit.com/auth/v2/oauth/access-token/loid" &&
			req.Method == "POST" &&
			req.Header.Get("Authorization") != "" &&
			req.Header.Get("User-Agent") != "" &&
			req.Header.Get("X-Reddit-Device-Id") != ""
	})).Return(createHTTPResponse(200, string(responseBody), map[string]string{
		"x-reddit-loid":    "test-loid",
		"x-reddit-session": "test-session",
	}), nil)

	err = client.Authenticate(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "test-token", client.accessToken)
	assert.Equal(t, "test-loid", client.loid)
	assert.Equal(t, "test-session", client.session)
	assert.True(t, client.authenticated)
	mockHTTP.AssertExpectations(t)
}

func TestAuthenticate_HTTPError(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return((*http.Response)(nil), fmt.Errorf("network error"))

	err = client.Authenticate(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication request failed")
	assert.Empty(t, client.accessToken)
	assert.False(t, client.authenticated)
}

func TestAuthenticate_BadStatusCode(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(401, "Unauthorized", nil), nil)

	err = client.Authenticate(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed with status: 401")
	assert.Empty(t, client.accessToken)
	assert.False(t, client.authenticated)
}

// Integration-style test for the complete authentication flow
func TestAuthenticationFlow_Integration(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	// Mock successful authentication
	oauthResponse := OAuthResponse{
		AccessToken: "integration-token",
		TokenType:   "bearer",
		ExpiresIn:   3600,
		Scope:       []string{"* email pii"},
	}
	responseBody, _ := json.Marshal(oauthResponse)

	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// Verify the request contains all required headers
		hasAuth := req.Header.Get("Authorization") != ""
		hasUserAgent := req.Header.Get("User-Agent") != ""
		hasDeviceID := req.Header.Get("X-Reddit-Device-Id") != ""
		hasContentType := req.Header.Get("Content-Type") == "application/json; charset=UTF-8"

		// Verify request body
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body)) // Reset body for actual use
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)
		hasCorrectScopes := len(reqBody["scopes"].([]interface{})) == 3

		return hasAuth && hasUserAgent && hasDeviceID && hasContentType && hasCorrectScopes
	})).Return(createHTTPResponse(200, string(responseBody), map[string]string{
		"x-reddit-loid":    "integration-loid",
		"x-reddit-session": "integration-session",
	}), nil)

	err = client.Authenticate(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "integration-token", client.accessToken)
	assert.Equal(t, "integration-loid", client.loid)
	assert.Equal(t, "integration-session", client.session)
	assert.True(t, client.authenticated)
	mockHTTP.AssertExpectations(t)
}
