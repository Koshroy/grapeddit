# Analysis of Fetching Post Comments in Redlib

This document provides a detailed breakdown of how the Redlib client fetches and processes comments for a Reddit post. The process involves a primary API call followed by special handling for large and deeply nested comment sections.

## 1. The Comment Fetching Process

### Step 1: The Primary API Call

The initial request to view a post and its comments uses one of the following API endpoints:

-   `/comments/{post_id}.json`
-   `/{subreddit_name}/comments/{post_id}.json`

This single API call efficiently returns a JSON object containing both the post data and the top-level comments.

#### Comment Sorting

The client can specify the sort order for the comments by passing a `sort` query parameter in the URL, which is then passed directly to the Reddit API. Common values include `confidence` (Best), `top`, `new`, `controversial`, `old`, and `q&a`.

**Example `curl` for initial fetch:**
```bash
# Assumes $ACCESS_TOKEN is set from the authentication flow
# Fetches a post and its comments, sorted by "new"
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://oauth.reddit.com/comments/18t5968.json?raw_json=1&sort=new" | jq .
```

### Step 2: Handling Large Comment Sections (The "More Comments" Object)

For posts with many comments, the Reddit API truncates the comment tree and inserts special `kind: "more"` placeholder objects. This object acts as a "Load More Comments" button.

The Redlib client parses the comment tree, and when it encounters a `"more"` object, it renders a standard HTML link containing the IDs of the comments that need to be loaded.

-   **Example Link:** `/comments/{post_id}?children={id_1},{id_2},{id_3}`

When a user clicks this link, the server makes a new API call to fetch only those specific comments.

### Step 3: Handling Deeply Nested Threads ("Continue this thread")

If a single comment thread is too long, the API truncates it and inserts a `"more"` object specific to that thread. The client renders this as a "Continue this thread ->" link, which points to a URL focused on that single comment.

-   **Example Link:** `/r/{subreddit}/comments/{post_id}/comment/{comment_id}`

Clicking this link loads a new page focused solely on that part of the conversation.

## 2. JSON Response Shapes

The JSON responses for comments have a well-defined, recursive structure.

### Shape 1: Initial Post and Comments Response

The primary request to `/comments/{post_id}.json` returns a top-level **array** containing two `Listing` objects: one for the post and one for the comments.

```json
[
  {
    "kind": "Listing",
    "data": {
      "children": [ /* Contains one 't3' post object */ ]
    }
  },
  {
    "kind": "Listing",
    "data": {
      "after": "t1_...", // Cursor for next page of comments
      "children": [ /* Array of 't1' comment and 'more' objects */ ]
    }
  }
]
```

#### The Post Object (`kind: "t3"`)

This object, found in the first `Listing`, represents the post itself.

```json
{
  "kind": "t3",
  "data": {
    "id": "18t5968",
    "author": "koushikroy",
    "title": "Why use tuple struct over standard struct?",
    "selftext": "The body of the post goes here in Markdown format.",
    "score": 1234,
    "subreddit": "rust",
    "num_comments": 567
    // ... and many other fields
  }
}
```

#### The Comment Object (`kind: "t1"`)

Found in the second `Listing`, these objects represent individual comments. The `replies` field contains another `Listing` object, creating the recursive tree. If there are no replies, `replies` is an empty string.

```json
{
  "kind": "t1",
  "data": {
    "id": "kfbqlbc",
    "author": "SomeUser",
    "body": "This is the text of the comment in Markdown.",
    "score": 89,
    "replies": {
      "kind": "Listing",
      "data": {
        "children": [
          // This array contains more 't1' (reply) or 'more' objects
        ]
      }
    }
    // ... and other fields
  }
}
```

#### The "More Comments" Object (`kind: "more"`)

This placeholder is found in any `children` array when comments are truncated.

```json
{
  "kind": "more",
  "data": {
    "count": 25,
    "id": "kfc1234",
    "parent_id": "t1_kfbqlbc",
    "children": [
      // An array of STRING IDs of the comments to load
      "kfc1234",
      "kfc5678",
      "kfc9abc"
    ]
  }
}
```

### Shape 2: The "Load More Comments" API Response

When fetching the comments from a `"more"` object, the client makes a `POST` request to `/api/morechildren.json`. The response is **not** a `Listing`.

```json
{
  "json": {
    "errors": [],
    "data": {
      "things": [
        // An array containing the fully-formed 't1' comment objects
        // that were requested. Each object has the same recursive
        // shape as described above.
      ]
    }
  }
}
```
This structure allows the client to efficiently fetch and insert entire branches into the comment tree where the `"more"` placeholder was.
