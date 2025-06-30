# Redlib API Method Analysis

This document lists the Reddit API methods accessed by this project. All requests are `GET` requests made to the base URL `https://oauth.reddit.com`, and the project appends `raw_json=1` to ensure it receives complete, unprocessed JSON data.

## Core Content

### Front Page and Subreddit Listings

Used for browsing posts on the front page or within specific subreddits.

-   `/{sort_order}.json`
    -   **Description**: Fetches listings from the user's home feed (e.g., Best, Hot, New).
    -   **Example**: `/hot.json`
-   `/r/{subreddit_name}/{sort_order}.json`
    -   **Description**: Fetches listings from a specific subreddit.
    -   **Example**: `/r/rust/new.json`

### Post and Comments

Used for viewing a specific post and its associated comment threads.

-   `/comments/{post_id}.json`
    -   **Description**: Retrieves a post and its full comment tree.
-   `/{subreddit_name}/comments/{post_id}.json`
    -   **Description**: An alternative path format for retrieving a post and its comments.
-   `/r/{subreddit_name}/comments/{post_id}/comment/{comment_id}.json`
    -   **Description**: Retrieves a specific comment and its replies within the context of its parent post.

### Subreddit Information

Used to display metadata about a community.

-   `/r/{subreddit_name}/about.json`
    -   **Description**: Fetches sidebar information, rules, description, and other metadata for a subreddit.

### User Information

Used for viewing user profiles and their activity.

-   `/user/{username}/about.json`
    -   **Description**: Fetches a user's public profile information (e.g., karma, cake day).
-   `/user/{username}/{listing_type}.json`
    -   **Description**: Fetches a user's activity, such as their submitted posts, comments, or general overview.
    -   **Example**: `/user/koushikroy/submitted.json`

## Specialized Listings

### Search

Used for finding posts and comments.

-   `/search.json?q={query}&sort={sort}&t={timeframe}`
    -   **Description**: Performs a global search across all of Reddit.
-   `/r/{subreddit_name}/search.json?q={query}&sort={sort}&t={timeframe}`
    -   **Description**: Performs a search limited to a specific subreddit.

### Duplicate Posts

Used to find other submissions of the same link.

-   `/r/{subreddit_name}/duplicates/{post_id}.json`
    -   **Description**: Finds other posts that link to the same URL as the specified post.

### Multireddits

Used for viewing custom collections of subreddits.

-   `/user/{username}/m/{multireddit_name}.json`
    -   **Description**: Fetches the post listing for a user's specified multireddit.
