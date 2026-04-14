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

// HNSearchResponse is the Algolia search API response.
type HNSearchResponse struct {
	Hits             []HNSearchHit `json:"hits"`
	NbHits           int           `json:"nbHits"`
	HitsPerPage      int           `json:"hitsPerPage"`
	Page             int           `json:"page"`
	NbPages          int           `json:"nbPages"`
}

// HNSearchHit is a single result from the Algolia HN search API.
type HNSearchHit struct {
	ObjectID    string  `json:"objectID"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Points      int     `json:"points"`
	NumComments int     `json:"num_comments"`
	CreatedAt   string  `json:"created_at"`
	StoryText   string  `json:"story_text"`
}

// SearchHNStories searches Hacker News via the Algolia API.
// sort: "relevance" → /search, "recent" → /search_by_date
func SearchHNStories(query string, limit int, sort string) (*HNSearchResponse, error) {
	endpoint := "search"
	if sort == "recent" {
		endpoint = "search_by_date"
	}

	ctx, cancel := context.WithTimeout(context.Background(), hnSearchTimeout)
	defer cancel()

	params := url.Values{}
	params.Set("query", query)
	params.Set("hitsPerPage", fmt.Sprintf("%d", limit))
	params.Set("tags", "story")

	apiURL := fmt.Sprintf("https://hn.algolia.com/api/v1/%s?%s", endpoint, params.Encode())

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

	var result HNSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "HN Algolia returned invalid JSON: "+err.Error())
	}

	return &result, nil
}
