# Redlib Client Analysis

This document outlines the methods used by Redlib to communicate with Reddit's internal API. The primary strategy involves spoofing the official Reddit Android application to gain privileged access, but it also includes several other specialized techniques to ensure reliable communication.

*This analysis is current as of commit `628fbf86cc4edc41cfa54e710b2434edbed97b75`.*

## Part 1: The OAuth Authentication Flow

The core of the client's authentication strategy is to mimic a request from the Reddit Android app to a specific, non-public OAuth endpoint.

### Step 1: The Authentication Request

To obtain an access token, you must send a `POST` request to the following URL:

`https://www.reddit.com/auth/v2/oauth/access-token/loid`

This request must include a specific set of headers and a JSON body to be successful.

#### Required Headers

The headers are crucial for successfully spoofing the Android client.

1.  **`Authorization`**: This header uses HTTP Basic authentication. The username is the hardcoded OAuth Client ID for the Reddit Android app, and the password is left empty. The resulting string is then Base64-encoded.
    *   **Client ID**: `ohXpoqrZYub1kg`
    *   **Format**: `Basic <base64_encode("ohXpoqrZYub1kg:")>`
    *   **Example Value**: `Basic b2hYcG9xclpZdWIxa2c6`

2.  **`User-Agent`**: This should mimic a real Android app User-Agent. It is constructed from a randomized Android version and a known Reddit app version.
    *   **Format**: `Reddit/{app_version}/Android {android_version}`
    *   **Example Value**: `Reddit/2023.46.0/Android 12`

3.  **`X-Reddit-Device-Id` / `client-vendor-id`**: These headers should be set to the same unique identifier for the "device". The ID is a newly generated **Version 4 UUID (Universally Unique Identifier)**, which is a randomly generated ID. This should be unique for each authentication attempt.
    *   **Example Value**: `a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d`

4.  **`Content-Type`**: Must be set to `application/json; charset=UTF-8`.

5.  **Other Spoofed Headers**: To increase the authenticity of the request, several other headers are included:
    *   `x-reddit-retry`: `algo=no-retries`
    *   `x-reddit-compression`: `1`
    *   `x-reddit-qos`: A randomized float value (e.g., `42.123`)
    *   `x-reddit-media-codecs`: A string listing supported codecs, e.g., `available-codecs=video/avc, video/hevc, video/x-vnd.on2.vp9`

#### Request Body

The body of the `POST` request is a simple JSON object that specifies the desired scopes.

```json
{
  "scopes": ["*", "email", "pii"]
}
```

### Step 2: Handling the Response

If the request is successful, Reddit will respond with a `200 OK` status and a JSON body containing the access token.

#### Successful JSON Response

The response body will look like this:

```json
{
  "access_token": "your-new-access-token",
  "token_type": "bearer",
  "expires_in": 3600,
  "scope": "* email pii"
}
```

-   **`access_token`**: This is the bearer token you will use for subsequent API requests.
-   **`expires_in`**: The lifetime of the token in seconds (e.g., 3600 seconds = 1 hour). Your application should refresh the token before it expires.

#### Important Response Headers

The response also contains headers that should be captured and sent with subsequent requests to maintain the "session."

-   **`x-reddit-loid`**: A unique identifier for the logged-in user on this "device."
-   **`x-reddit-session`**: A session tracker.

### Step 3: Making Authenticated API Calls

Once you have the access token, you can make requests to Reddit's API (`https://oauth.reddit.com`). For every subsequent request, you must include the `Authorization` header, the spoofed `User-Agent`, and the `x-reddit-loid` and `x-reddit-session` headers from the token response.

## Part 2: Proactive and Defensive Client Behaviors

Beyond the core authentication flow, the client employs several other techniques to ensure robust and stealthy communication with the Reddit API.

### Reactive Error Handling for Restricted Content

The client first makes an optimistic request for content. If that content is restricted, Reddit's API returns a JSON object with a specific `"reason"` field instead of the expected data. The client uses this field to determine its next action.

#### "gated" and "quarantined" Content (Recoverable)

-   **Detection**: The initial API response contains `{"reason": "gated"}` or `{"reason": "quarantined"}`.
-   **Behavior**: This is treated as a recoverable error. The client immediately and automatically retries the request, but this time it adds a special `Cookie` header to programmatically accept the content warning.
-   **Cookie Value**: `_options=%7B%22pref_quarantine_optin%22%3A%20true%2C%20%22pref_gated_sr_optin%22%3A%20true%7D`
-   **Outcome**: The second request succeeds, and the user sees the content seamlessly.

#### "private" Content (Non-Recoverable)

-   **Detection**: The API response contains `{"reason": "private"}`.
-   **Behavior**: This is treated as a terminal, non-recoverable error. The client cannot bypass this restriction.
-   **Outcome**: The error is propagated up to the main server logic, which halts the request and serves a user-facing error page indicating the content is private.

### Proactive Rate Limit Management

The client actively monitors its remaining API rate limit via the `x-ratelimit-remaining` header. If the count drops below a threshold of 10, it triggers a background refresh of the OAuth token. This ensures a fresh token with a full rate limit is almost always available, preventing rate limit errors and ensuring a smooth user experience.

### Header Manipulation and Sanitization

#### Header Shuffling (Anti-Fingerprinting)

Before a request is sent, its HTTP headers are deliberately shuffled into a random order. This is an anti-fingerprinting technique used to prevent servers from identifying the client based on a consistent header order.

#### Response Header Stripping (Privacy)

When proxying media content, the client explicitly strips numerous headers from Reddit's response before passing it to the user. This includes headers related to CDNs (`x-cdn`), caching (`etag`), and internal tracking (`x-reddit-cdn`, `Nel`, `Report-To`). This is a privacy-preserving measure to avoid leaking Reddit's internal infrastructure details to the end-user's browser.

### URL Sanitization and Canonicalization

The client intelligently handles various URL formats and cleans them to ensure internal consistency. This is crucial in two main scenarios:

1.  **Resolving "Share" Links**: When a user provides a "Share" link (e.g., `/r/rust/s/kPgq8WNHRK`), the client follows the redirect. The final URL from Reddit includes tracking parameters (e.g., `?utm_source=share...`). The client strips these parameters to get the clean, canonical path, which is essential for caching and internal logic.
2.  **Normalizing Redirected API Paths**: If a request for a URL ending in `.json` is redirected, the `Location` header might also contain `.json`. The client strips this suffix to ensure it always works with a clean base path, preventing errors like requesting a URL that ends in `.json.json`.

### Automatic Gzip Decompression

The client requests gzip compression on `GET` requests to reduce bandwidth. It automatically decompresses gzipped response bodies before parsing them, which is a standard performance optimization.

## Part 3: Testing with `curl`

The following examples demonstrate how to replicate the client's behavior using `curl`.

### 1. Authentication

First, get an access token. You'll need a UUID for the device ID.

```bash
# Generate a random UUID for the device
DEVICE_ID=$(uuidgen)

# Make the authentication request and extract the token and session headers
# Note: We use -D - to dump headers to stdout, then grep/cut to extract them.
AUTH_RESPONSE=$(curl -s -X POST \
  -D - \
  'https://www.reddit.com/auth/v2/oauth/access-token/loid' \
  -H 'Authorization: Basic b2hYcG9xclpZdWIxa2c6' \
  -H 'User-Agent: Reddit/2023.46.0/Android 12' \
  -H "X-Reddit-Device-Id: $DEVICE_ID" \
  -H "client-vendor-id: $DEVICE_ID" \
  -H 'Content-Type: application/json; charset=UTF-8' \
  -d '{"scopes": ["*", "email", "pii"]}')

# Extract the token and headers from the response
ACCESS_TOKEN=$(echo "$AUTH_RESPONSE" | grep -o '"access_token": "[^"]*' | grep -o '[^"]*$')
REDDIT_LOID=$(echo "$AUTH_RESPONSE" | grep -i 'x-reddit-loid:' | cut -d' ' -f2 | tr -d '\r')
REDDIT_SESSION=$(echo "$AUTH_RESPONSE" | grep -i 'x-reddit-session:' | cut -d' ' -f2 | tr -d '\r')

echo "Access Token: $ACCESS_TOKEN"
echo "Loid: $REDDIT_LOID"
echo "Session: $REDDIT_SESSION"
```

### 2. Making a Standard API Call

Use the credentials from Step 1 to fetch a standard subreddit listing.

```bash
curl -s \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "User-Agent: Reddit/2023.46.0/Android 12" \
  -H "x-reddit-loid: $REDDIT_LOID" \
  -H "x-reddit-session: $REDDIT_SESSION" \
  -H "X-Reddit-Device-Id: $DEVICE_ID" \
  "https://oauth.reddit.com/r/rust/hot.json?raw_json=1" | jq .
```

### 3. Handling Gated/Quarantined Content

This example shows the two-step process for accessing a quarantined subreddit.

#### Step 3a: Initial (Failing) Request

This first request will fail with a `{"reason": "quarantined"}` error.

```bash
# Attempt to access a quarantined sub without the cookie
curl -s \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "User-Agent: Reddit/2023.46.0/Android 12" \
  -H "x-reddit-loid: $REDDIT_LOID" \
  -H "x-reddit-session: $REDDIT_SESSION" \
  "https://oauth.reddit.com/r/Quarantined/hot.json?raw_json=1" | jq .
```

#### Step 3b: Successful Request with Cookie

The client sees the error and retries, this time adding the special `Cookie` header.

```bash
# Retry the request with the quarantine opt-in cookie
curl -s \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "User-Agent: Reddit/2023.46.0/Android 12" \
  -H "x-reddit-loid: $REDDIT_LOID" \
  -H "x-reddit-session: $REDDIT_SESSION" \
  -b '_options=%7B%22pref_quarantine_optin%22%3A%20true%2C%20%22pref_gated_sr_optin%22%3A%20true%7D' \
  "https://oauth.reddit.com/r/Quarantined/hot.json?raw_json=1" | jq .
```

### 4. Resolving a Share Link

This demonstrates how the client resolves a share link to its canonical path.

```bash
# Use curl's -L flag to follow redirects and -w to get the final URL
# The -o /dev/null discards the body, similar to a HEAD request
curl -s -L -w '%{url_effective}\n' \
  -o /dev/null \
  "https://www.reddit.com/r/rust/s/kPgq8WNHRK"
# Expected Output (before client sanitization):
# https://www.reddit.com/r/rust/comments/18t5968/why_use_tuple_struct_over_standard_struct/kfbqlbc/?utm_source=share&utm_medium=web2x&context=3
```
The client would then internally strip the query parameters from this effective URL.

## Appendix: Generating Android App Versions

The list of valid Android App Versions used for the `User-Agent` header is not generated algorithmically. Instead, it is a hardcoded list that is periodically updated by a helper script: `scripts/update_oauth_resources.sh`.

### Version Scraping Process

The script automates the process of finding and formatting real, historical version data for the Reddit Android app.

1.  **Scrape APKCombo:** The script scrapes `apkcombo.com`, a repository for Android application packages (APKs), targeting the old versions page for the official Reddit app (`com.reddit.frontpage`).
2.  **Paginate:** It scrapes the first five pages of results to get a comprehensive list of past versions.
3.  **Extract Links:** It uses `curl` to fetch the HTML and `rg` (ripgrep) to find the download links for each version.
4.  **Fetch Build Numbers:** For each version link it finds, it makes another `curl` request to that page and scrapes it to extract the specific version number and the corresponding build number.
5.  **Format and Write:** Finally, it formats the scraped data into the `Version {version}/Build {build}` string format and writes it into the `src/oauth_resources.rs` file as a Rust array, which is then compiled into the application.

A full list of the known versions obtained by this script can be found in [VERSION-NUMBERS.md](VERSION-NUMBERS.md).