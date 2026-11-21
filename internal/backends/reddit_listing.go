package backends

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParseRedditPostListing returns the posts (t3 things) of a subreddit or
// front-page listing, in Reddit's order.
func ParseRedditPostListing(listings []RedditListing) []RedditPost {
	posts := []RedditPost{}
	for _, l := range listings {
		for _, thing := range l.Data.Children {
			if thing.Kind != "t3" {
				continue
			}
			var p RedditPost
			if err := json.Unmarshal(thing.Data, &p); err == nil {
				posts = append(posts, p)
			}
		}
	}
	return posts
}

// RenderRedditPostListingMarkdown renders a listing as a numbered list of
// posts with their links, so a caller can pick threads to read next.
func RenderRedditPostListingMarkdown(title string, posts []RedditPost) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n%d posts\n", title, len(posts))
	for i, p := range posts {
		fmt.Fprintf(&sb, "\n## %d. %s\n\n", i+1, p.Title)
		if p.Permalink != "" {
			fmt.Fprintf(&sb, "URL: https://www.reddit.com%s\n", p.Permalink)
		}
		if p.Author != "" {
			fmt.Fprintf(&sb, "Author: u/%s\n", p.Author)
		}
		if p.CreatedUTC > 0 {
			fmt.Fprintf(&sb, "Date: %s\n", time.Unix(int64(p.CreatedUTC), 0).UTC().Format(time.RFC3339))
		}
		fmt.Fprintf(&sb, "Score: %d\nComments: %d\n", p.Score, p.NumComments)
		switch {
		case p.Selftext != "":
			snippet := p.Selftext
			if r := []rune(snippet); len(r) > 300 {
				snippet = string(r[:300]) + "..."
			}
			fmt.Fprintf(&sb, "\n%s\n", snippet)
		case p.URL != "" && !strings.Contains(p.URL, p.Permalink):
			fmt.Fprintf(&sb, "\nLink: %s\n", p.URL)
		}
	}
	return sb.String()
}
