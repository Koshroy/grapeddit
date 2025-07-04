package redditclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockHTTPClient for testing
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

// Helper function to create HTTP response
func createHTTPResponse(statusCode int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	for k, v := range headers {
		resp.Header.Set(k, v)
	}

	return resp
}

func TestNewClient(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, mockHTTP, client.httpClient)
	assert.NotEmpty(t, client.deviceID)
	assert.NotEmpty(t, client.userAgent)
	assert.Equal(t, 100, client.rateLimit)
}

func TestNewClientWithNilHTTPClient(t *testing.T) {
	client, err := NewClient(nil)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.IsType(t, &http.Client{}, client.httpClient)
}

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
}

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

func TestGetPost_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.GetPost(t.Context(), "golang", "abc123")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
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

func TestSearch_NotAuthenticated(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)

	result, err := client.Search(t.Context(), "golang", "top", "week")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}

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
				Kind string      `json:"kind"`
				Data interface{} `json:"data"`
			} `json:"children"`
		}{
			Children: []struct {
				Kind string      `json:"kind"`
				Data interface{} `json:"data"`
			}{
				{
					Kind: "t3",
					Data: map[string]interface{}{
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

func TestShuffleHeaders(t *testing.T) {
	client, err := NewClient(nil)
	require.NoError(t, err)
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	headers := map[string]string{
		"Header1": "Value1",
		"Header2": "Value2",
		"Header3": "Value3",
	}

	client.shuffleHeaders(req, headers)

	// Verify all headers are set
	for k, v := range headers {
		assert.Equal(t, v, req.Header.Get(k))
	}
}

func TestUpdateRateLimit(t *testing.T) {
	client, err := NewClient(nil)
	require.NoError(t, err)
	initialLimit := client.rateLimit

	client.updateRateLimit("50")

	// In our simplified implementation, rate limit decreases by 1
	assert.Equal(t, initialLimit-1, client.rateLimit)
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
	mockHTTP.AssertExpectations(t)
}

func TestNewClient_Success(t *testing.T) {
	// Test that NewClient creates a client successfully
	client, err := NewClient(nil)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.NotEmpty(t, client.deviceID)
	assert.NotEmpty(t, client.userAgent)
}

func TestGzipDecompression(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Create gzipped response content
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	testContent := `{"kind": "Listing", "data": {"children": [{"kind": "t3", "data": {"id": "test123", "title": "Test Gzip Post", "score": 42}}]}}`
	_, err = gzipWriter.Write([]byte(testContent))
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)

	// Mock gzipped response
	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(200, buf.String(), map[string]string{
			"Content-Encoding": "gzip",
		}), nil).Once()

	// Request should properly decompress gzipped content
	result, err := client.GetSubreddit(t.Context(), "test", "hot")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Listing", result.Kind)
	assert.Len(t, result.Data.Children, 1)
	assert.Equal(t, "Test Gzip Post", result.Data.Children[0].Data.Title)
	assert.Equal(t, 42, result.Data.Children[0].Data.Score)

	// Mock second gzipped response to test pool reuse
	var buf2 bytes.Buffer
	gzipWriter2 := gzip.NewWriter(&buf2)
	testContent2 := `{"kind": "Listing", "data": {"children": [{"kind": "t3", "data": {"id": "test456", "title": "Another Gzip Post", "score": 84}}]}}`
	_, err = gzipWriter2.Write([]byte(testContent2))
	require.NoError(t, err)
	err = gzipWriter2.Close()
	require.NoError(t, err)

	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(200, buf2.String(), map[string]string{
			"Content-Encoding": "gzip",
		}), nil).Once()

	// Second request should also work with pooled gzip readers
	result2, err := client.GetSubreddit(t.Context(), "test2", "hot")
	require.NoError(t, err)
	assert.NotNil(t, result2)
	assert.Equal(t, "Listing", result2.Kind)
	assert.Len(t, result2.Data.Children, 1)
	assert.Equal(t, "Another Gzip Post", result2.Data.Children[0].Data.Title)
	assert.Equal(t, 84, result2.Data.Children[0].Data.Score)

	mockHTTP.AssertExpectations(t)
}

func TestMultipleGzipRequests(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Helper function to create gzipped response content
	createGzippedResponse := func(title string) string {
		var buf bytes.Buffer
		gzipWriter := gzip.NewWriter(&buf)
		testContent := fmt.Sprintf(`{"kind": "Listing", "data": {"children": [{"kind": "t3", "data": {"id": "test", "title": "%s", "score": 100}}]}}`, title)
		gzipWriter.Write([]byte(testContent))
		gzipWriter.Close()
		return buf.String()
	}

	// Mock each response individually with fresh gzipped content
	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(200, createGzippedResponse("Request 1"), map[string]string{
			"Content-Encoding": "gzip",
		}), nil).Once()

	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(200, createGzippedResponse("Request 2"), map[string]string{
			"Content-Encoding": "gzip",
		}), nil).Once()

	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return(createHTTPResponse(200, createGzippedResponse("Request 3"), map[string]string{
			"Content-Encoding": "gzip",
		}), nil).Once()

	// Test sequential requests to verify pool reuse works
	for i := 1; i <= 3; i++ {
		result, err := client.GetSubreddit(t.Context(), fmt.Sprintf("test%d", i), "hot")
		require.NoError(t, err, "Request %d should succeed", i)
		require.NotNil(t, result, "Result %d should not be nil", i)
		require.Len(t, result.Data.Children, 1, "Result %d should have one child", i)
		expectedTitle := fmt.Sprintf("Request %d", i)
		assert.Equal(t, expectedTitle, result.Data.Children[0].Data.Title, "Request %d should have correct title", i)
	}

	mockHTTP.AssertExpectations(t)
}

func TestContextCancellation(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Create a cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	// Mock the HTTP client to return context canceled error
	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return((*http.Response)(nil), context.Canceled)

	// Test that API call respects cancelled context
	_, err = client.GetSubreddit(ctx, "golang", "hot")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")

	// Test that authentication respects cancelled context
	err = client.Authenticate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")

	mockHTTP.AssertExpectations(t)
}

func TestContextTimeout(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Millisecond)
	defer cancel()

	// Sleep to ensure timeout occurs
	time.Sleep(5 * time.Millisecond)

	// Mock the HTTP client to return context deadline exceeded error
	mockHTTP.On("Do", mock.AnythingOfType("*http.Request")).
		Return((*http.Response)(nil), context.DeadlineExceeded)

	// Test that API call respects timeout
	_, err = client.GetSubreddit(ctx, "golang", "hot")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	mockHTTP.AssertExpectations(t)
}

func TestContextPropagation(t *testing.T) {
	testCtx := t.Context() // Store test context in variable to avoid conflicts
	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

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

	// Verify that the context is properly propagated to the HTTP request
	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// Check that the request context is derived from our test context
		reqCtx := req.Context()
		assert.NotNil(t, reqCtx)

		// The request context should have the same deadline as our test context (if any)
		if deadline, ok := testCtx.Deadline(); ok {
			reqDeadline, reqOk := reqCtx.Deadline()
			assert.True(t, reqOk)
			assert.Equal(t, deadline, reqDeadline)
		}

		return strings.Contains(req.URL.String(), "/r/golang/hot.json")
	})).Return(createHTTPResponse(200, string(responseBody), map[string]string{
		"x-ratelimit-remaining": "50",
	}), nil)

	result, err := client.GetSubreddit(testCtx, "golang", "hot")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Listing", result.Kind)
	mockHTTP.AssertExpectations(t)
}

// TestTestContextTimeout demonstrates how test timeouts work with t.Context()
// Run with: go test -v -run TestTestContextTimeout -timeout 100ms
func TestTestContextTimeout(t *testing.T) {
	// Skip this test during normal runs to avoid timeout failures
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	mockHTTP := &MockHTTPClient{}
	client, err := NewClient(mockHTTP)
	require.NoError(t, err)
	client.accessToken = "test-token"
	client.authenticated = true

	// Mock a slow HTTP response that would exceed the test timeout
	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// Check if the context has a deadline (it should from test timeout)
		deadline, hasDeadline := req.Context().Deadline()
		if hasDeadline {
			t.Logf("Request context has deadline: %v", deadline)
		} else {
			t.Log("Request context has no deadline")
		}
		return true
	})).Return(createHTTPResponse(200, `{"kind": "Listing", "data": {"children": []}}`, nil), nil)

	// This should work normally, but if you run with a very short timeout,
	// the test context will have a deadline that gets propagated to the request
	_, err = client.GetSubreddit(t.Context(), "golang", "hot")
	assert.NoError(t, err)

	mockHTTP.AssertExpectations(t)
}
