package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/oaooao/webx/internal/types"
)

const redditSearchTimeout = 30 * time.Second

// redditSearchListing is the Reddit search.json response (single Listing, not a pair).
type redditSearchListing struct {
	Kind string              `json:"kind"`
	Data redditSearchData    `json:"data"`
}

type redditSearchData struct {
	Children []RedditThing `json:"children"`
	After    string        `json:"after"`
	Dist     int           `json:"dist"`
}

// redditSearchPost is the t3 data returned in search results.
type redditSearchPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	Permalink   string  `json:"permalink"`
	URL         string  `json:"url"`
	Selftext    string  `json:"selftext"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
}

// SearchRedditPosts searches Reddit via the public search.json endpoint.
// apiURL should be the full URL. In production, use BuildRedditSearchURL.
func SearchRedditPosts(apiURL string, limit int, sort string) (*types.NormalizedSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redditSearchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	req.Header.Set("User-Agent", redditUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := sharedStdClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "Reddit search timed out")
		}
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 429:
		return nil, types.NewWebxError(types.ErrRateLimited, "Reddit search rate limited (429)")
	case 403:
		return nil, types.NewWebxError(types.ErrLoginRequired, "Reddit search returned 403")
	case 200:
		// ok
	default:
		return nil, types.NewWebxError(types.ErrFetchFailed, "Reddit search HTTP "+resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}

	var listing redditSearchListing
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "Reddit search returned invalid JSON: "+err.Error())
	}

	items := make([]types.SearchResultItem, 0, len(listing.Data.Children))
	for _, thing := range listing.Data.Children {
		if thing.Kind != "t3" {
			continue
		}
		var post redditSearchPost
		if err := json.Unmarshal(thing.Data, &post); err != nil {
			continue
		}

		postURL := "https://www.reddit.com" + post.Permalink
		snippet := post.Selftext
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}

		items = append(items, types.SearchResultItem{
			Title:   post.Title,
			URL:     postURL,
			Snippet: snippet,
			Author:  post.Author,
			Score:   float64(post.Score),
			Kind:    types.KindThread,
			Meta: map[string]any{
				"subreddit":    post.Subreddit,
				"num_comments": post.NumComments,
				"external_url": post.URL,
			},
		})
	}

	hasMore := listing.Data.After != ""

	return &types.NormalizedSearchResult{
		Items:   items,
		Query:   "",
		Backend: "reddit_json",
		HasMore: hasMore,
	}, nil
}

// BuildRedditSearchURL constructs the Reddit search.json URL.
// sort: "relevance" → sort=relevance, "recent" → sort=new, "top" → sort=top
func BuildRedditSearchURL(query string, limit int, sort string) string {
	redditSort := "relevance"
	switch sort {
	case "recent":
		redditSort = "new"
	case "top":
		redditSort = "top"
	}
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("sort", redditSort)
	params.Set("type", "link")
	return "https://www.reddit.com/search.json?" + params.Encode()
}
