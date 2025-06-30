package redditclient

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
)

// Authenticate performs OAuth authentication
func (c *Client) Authenticate() error {
	// OAuth Client ID for Reddit Android app
	auth := base64.StdEncoding.EncodeToString([]byte(ANDROID_CLIENT_ID + ":"))

	body := map[string]interface{}{
		"scopes": []string{"*", "email", "pii"},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to unmarshal json response body: %w", err)
	}

	req, err := http.NewRequest("POST", "https://www.reddit.com/auth/v2/oauth/access-token/loid", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Required headers for Android app spoofing
	headers := map[string]string{
		"Authorization":         "Basic " + auth,
		"User-Agent":            c.userAgent,
		"X-Reddit-Device-Id":    c.deviceID,
		"client-vendor-id":      c.deviceID,
		"Content-Type":          "application/json; charset=UTF-8",
		"x-reddit-retry":        "algo=no-retries",
		"x-reddit-compression":  "1",
		"x-reddit-qos":          fmt.Sprintf("%.3f", rand.Float64()*100),
		"x-reddit-media-codecs": "available-codecs=video/avc, video/hevc, video/x-vnd.on2.vp9",
	}

	c.shuffleHeaders(req, headers)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	var oauthResp OAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&oauthResp); err != nil {
		return fmt.Errorf("failed to decode OAuth response: %w", err)
	}

	c.accessToken = oauthResp.AccessToken
	c.loid = resp.Header.Get("x-reddit-loid")
	c.session = resp.Header.Get("x-reddit-session")

	return nil
}