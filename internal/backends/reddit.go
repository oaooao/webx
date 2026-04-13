package backends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/types"
)

const (
	redditTimeout    = 30 * time.Second
	redditUserAgent  = "webx/0.1 (+https://github.com/oaooao/webx)"
	redditMaxRetries = 3
	redditRetryDelay = 2 * time.Second
)

// Reddit API JSON types

type RedditListing struct {
	Kind string         `json:"kind"`
	Data RedditListData `json:"data"`
}

type RedditListData struct {
	Children []RedditThing `json:"children"`
	After    string        `json:"after"`
}

type RedditThing struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type RedditPost struct {
	Title       string  `json:"title"`
	Selftext    string  `json:"selftext"`
	Author      string  `json:"author"`
	Score       int     `json:"score"`
	Subreddit   string  `json:"subreddit"`
	Permalink   string  `json:"permalink"`
	NumComments int     `json:"num_comments"`
	URL         string  `json:"url"`
	CreatedUTC  float64 `json:"created_utc"`
}

// RedditComment represents a Reddit comment. The Replies field is json.RawMessage
// because Reddit returns "" (empty string) for leaf comments instead of null.
// We manually handle the two cases: "" means no replies, object means parse as RedditListing.
type RedditComment struct {
	ID         string          `json:"id"`
	Author     string          `json:"author"`
	Body       string          `json:"body"`
	Score      int             `json:"score"`
	CreatedUTC float64         `json:"created_utc"`
	Depth      int             `json:"depth"`
	Replies    json.RawMessage `json:"replies"` // "" or RedditListing object
	// Fields present on "more" kind things
	Children []string `json:"children"` // child IDs for "more" placeholders
	Count    int      `json:"count"`    // number of collapsed comments
}

// RedditResult is the parsed output of a Reddit thread fetch.
type RedditResult struct {
	Post     RedditPost
	Comments []RedditCommentNode
}

// RedditCommentNode is a node in the parsed comment tree.
type RedditCommentNode struct {
	Comment RedditComment
	Replies []RedditCommentNode
}

// FetchRedditJSON fetches the .json endpoint for a Reddit URL with retries.
func FetchRedditJSON(redditURL string) ([]RedditListing, error) {
	parsedURL, err := url.Parse(redditURL)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}

	path := strings.TrimSuffix(parsedURL.Path, "/")
	if !strings.HasSuffix(path, ".json") {
		parsedURL.Path = path + ".json"
	}
	// Preserve query params but remove reddit tracking params that can cause issues
	jsonURL := parsedURL.String()

	var lastErr error
	for attempt := 0; attempt < redditMaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(redditRetryDelay)
		}

		listings, err := doRedditFetch(jsonURL)
		if err == nil {
			return listings, nil
		}

		lastErr = err
		var wxErr *types.WebxError
		if errors.As(err, &wxErr) && wxErr.Code == types.ErrRateLimited {
			continue // retry on 429
		}
		return nil, err // don't retry other errors
	}
	return nil, lastErr
}

func doRedditFetch(jsonURL string) ([]RedditListing, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redditTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	req.Header.Set("User-Agent", redditUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "Reddit fetch timed out")
		}
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 429:
		return nil, types.NewWebxError(types.ErrRateLimited, "Reddit rate limited (429)")
	case 403:
		return nil, types.NewWebxError(types.ErrLoginRequired, "Reddit returned 403 (login required or private)")
	case 200:
		// ok
	default:
		return nil, types.NewWebxError(types.ErrFetchFailed, "Reddit HTTP "+resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}

	var listings []RedditListing
	if err := json.Unmarshal(body, &listings); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "Reddit returned invalid JSON: "+err.Error())
	}

	return listings, nil
}

// ParseRedditListings parses Reddit's [post_listing, comments_listing] response.
func ParseRedditListings(listings []RedditListing) (*RedditResult, error) {
	if len(listings) < 1 {
		return nil, types.NewWebxError(types.ErrContentEmpty, "Reddit returned empty response")
	}

	// First listing contains the post (kind "t3")
	var post RedditPost
	if len(listings[0].Data.Children) > 0 && listings[0].Data.Children[0].Kind == "t3" {
		if err := json.Unmarshal(listings[0].Data.Children[0].Data, &post); err != nil {
			return nil, types.NewWebxError(types.ErrBackendFailed, "Failed to parse Reddit post: "+err.Error())
		}
	}

	// Second listing contains comments
	var comments []RedditCommentNode
	if len(listings) > 1 {
		comments = parseCommentChildren(listings[1].Data.Children)
	}

	return &RedditResult{Post: post, Comments: comments}, nil
}

// parseCommentChildren recursively parses a slice of RedditThings into a comment tree.
// C3 fix: Replies field is json.RawMessage; we detect "" vs object manually.
func parseCommentChildren(things []RedditThing) []RedditCommentNode {
	var nodes []RedditCommentNode
	for _, thing := range things {
		if thing.Kind != "t1" {
			// "more" placeholders are silently skipped in v0
			// (expandMoreComments is a no-op stub; see reddit adapter)
			continue
		}

		var comment RedditComment
		if err := json.Unmarshal(thing.Data, &comment); err != nil {
			continue
		}

		node := RedditCommentNode{Comment: comment}

		// C3 fix: manually handle replies field.
		// Reddit returns "" (empty string) for leaf nodes, or a Listing object.
		if len(comment.Replies) > 0 {
			trimmed := strings.TrimSpace(string(comment.Replies))
			if trimmed != `""` && trimmed != "" {
				// Attempt to parse as RedditListing
				var listing RedditListing
				if err := json.Unmarshal(comment.Replies, &listing); err == nil {
					node.Replies = parseCommentChildren(listing.Data.Children)
				}
			}
		}

		nodes = append(nodes, node)
	}
	return nodes
}

// ExpandMoreComments is a stub for v0. Full "more" comment expansion requires
// POST /api/morechildren.json with link_id + children IDs, subject to stricter
// rate limits. Not implemented in v0; the initial .json response's top-level
// comments are sufficient for most use cases.
func ExpandMoreComments(_ *RedditResult, _ string) {
	// TODO(v1): implement morechildren expansion
}

// RenderRedditMarkdown converts a RedditResult to readable markdown.
func RenderRedditMarkdown(result *RedditResult) string {
	var sb strings.Builder

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}
	fmt.Fprintf(&sb, "# %s\n\n", title)

	if result.Post.Subreddit != "" {
		fmt.Fprintf(&sb, "Subreddit: r/%s\n", result.Post.Subreddit)
	}
	if result.Post.Author != "" {
		fmt.Fprintf(&sb, "Author: u/%s\n", result.Post.Author)
	}
	fmt.Fprintf(&sb, "Score: %d\n", result.Post.Score)
	if result.Post.NumComments > 0 {
		fmt.Fprintf(&sb, "Comments: %d\n", result.Post.NumComments)
	}
	if result.Post.Permalink != "" {
		fmt.Fprintf(&sb, "Permalink: https://www.reddit.com%s\n", result.Post.Permalink)
	}

	sb.WriteString("\n## Post\n\n")
	if result.Post.Selftext != "" {
		sb.WriteString(result.Post.Selftext)
	} else if result.Post.URL != "" {
		fmt.Fprintf(&sb, "[Link post](%s)", result.Post.URL)
	} else {
		sb.WriteString("*(No post body.)*")
	}

	if len(result.Comments) > 0 {
		sb.WriteString("\n\n## Comments\n\n")
		for _, node := range result.Comments {
			renderCommentNode(&sb, node, 0)
		}
	}

	return sb.String()
}

func renderCommentNode(sb *strings.Builder, node RedditCommentNode, depth int) {
	indent := strings.Repeat("  ", depth)
	author := node.Comment.Author
	if author == "" {
		author = "[deleted]"
	}

	fmt.Fprintf(sb, "%s- **%s**", indent, author)
	if node.Comment.Score != 0 {
		fmt.Fprintf(sb, " (%d points)", node.Comment.Score)
	}
	sb.WriteString(":\n")

	if node.Comment.Body != "" {
		for _, line := range strings.Split(node.Comment.Body, "\n") {
			fmt.Fprintf(sb, "%s  %s\n", indent, line)
		}
	}
	sb.WriteString("\n")

	for _, reply := range node.Replies {
		renderCommentNode(sb, reply, depth+1)
	}
}
