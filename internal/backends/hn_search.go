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

const hnSearchTimeout = 15 * time.Second

// hnSearchResponse is the Algolia search API response (internal parsing type).
type hnSearchResponse struct {
	Hits        []hnSearchHit `json:"hits"`
	NbHits      int           `json:"nbHits"`
	HitsPerPage int           `json:"hitsPerPage"`
	Page        int           `json:"page"`
	NbPages     int           `json:"nbPages"`
}

type hnSearchHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAt   string `json:"created_at"`
	StoryText   string `json:"story_text"`
}

// SearchHNStories searches Hacker News via the Algolia API and returns normalized results.
// apiURL should be the full search URL (e.g. "https://hn.algolia.com/api/v1/search?query=foo&hitsPerPage=20&tags=story").
// In tests, a test server URL is passed directly; in production, callers use BuildHNSearchURL.
func SearchHNStories(apiURL string, limit int) (*types.NormalizedSearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hnSearchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "webx/0.1 (+https://github.com/oaooao/webx)")

	resp, err := sharedStdClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "HN Algolia search timed out")
		}
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewWebxError(types.ErrFetchFailed, "HN Algolia search HTTP "+resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}

	var raw hnSearchResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "HN Algolia returned invalid JSON: "+err.Error())
	}

	items := make([]types.SearchResultItem, 0, len(raw.Hits))
	for _, hit := range raw.Hits {
		itemURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		snippet := hit.StoryText
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		items = append(items, types.SearchResultItem{
			Title:   hit.Title,
			URL:     itemURL,
			Snippet: snippet,
			Author:  hit.Author,
			Date:    hit.CreatedAt,
			Score:   float64(hit.Points),
			Kind:    types.KindThread,
			Meta: map[string]any{
				"num_comments": hit.NumComments,
				"external_url": hit.URL,
			},
		})
	}

	hasMore := raw.Page+1 < raw.NbPages

	return &types.NormalizedSearchResult{
		Items:         items,
		Query:         "",
		TotalEstimate: raw.NbHits,
		Backend:       "hn_algolia",
		HasMore:       hasMore,
	}, nil
}

// BuildHNSearchURL constructs the Algolia HN search URL for the given query, limit, and sort.
// sort: "relevance" → /search, "recent" → /search_by_date
func BuildHNSearchURL(query string, limit int, sort string) string {
	endpoint := "search"
	if sort == "recent" {
		endpoint = "search_by_date"
	}
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("query", query)
	params.Set("hitsPerPage", fmt.Sprintf("%d", limit))
	params.Set("tags", "story")
	return fmt.Sprintf("https://hn.algolia.com/api/v1/%s?%s", endpoint, params.Encode())
}
