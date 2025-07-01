package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Koshroy/grapeddit/internal/redditclient"
)

func main() {
	ctx := context.Background()

	client, err := redditclient.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create Reddit client: %v", err)
	}

	fmt.Println("Authenticating with Reddit...")
	if err := client.Authenticate(ctx); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Println("Successfully authenticated!")

	// Example: Get posts from r/golang
	posts, err := client.GetSubreddit(ctx, "golang", "hot")
	if err != nil {
		log.Fatalf("Failed to get subreddit: %v", err)
	}

	fmt.Printf("Found %d posts in r/golang\n", len(posts.Data.Children))
	for i, post := range posts.Data.Children {
		if i >= 5 { // Show only first 5 posts
			break
		}
		fmt.Printf("- %s (Score: %d)\n", post.Data.Title, post.Data.Score)
	}
}
