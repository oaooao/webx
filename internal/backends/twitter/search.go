package twitter

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/oaooao/webx/internal/types"
)

const (
	// SearchTimelineQueryID is the GraphQL operation ID for SearchTimeline.
	// Update from https://github.com/fa0311/twitter-openapi/blob/main/src/config/placeholder.json
	// if requests start returning 404/422.
	SearchTimelineQueryID = "lZ0GCEojmtQfiUQa5oJSEw"
)

// searchTimelineVariables is the JSON variables payload for SearchTimeline.
type searchTimelineVariables struct {
	RawQuery                string `json:"rawQuery"`
	Count                   int    `json:"count"`
	Product                 string `json:"product"` // "Top" or "Latest"
	QuerySource             string `json:"querySource"`
	WithDownvotePerspective bool   `json:"withDownvotePerspective"`
	WithReactionsMetadata   bool   `json:"withReactionsMetadata"`
	WithReactionsPerspective bool  `json:"withReactionsPerspective"`
}

// searchTimelineFeatures mirrors the feature flags used by the web client for SearchTimeline.
var searchTimelineFeatures = map[string]bool{
	"responsive_web_graphql_exclude_directive_enabled":           true,
	"responsive_web_graphql_timeline_navigation_enabled":        true,
	"longform_notetweets_consumption_enabled":                   true,
	"responsive_web_twitter_article_tweet_consumption_enabled":  true,
	"rweb_video_timestamps_enabled":                             true,
	"freedom_of_speech_not_reach_fetch_enabled":                 true,
	"standardized_nudges_misinfo":                               true,
	"creator_subscriptions_tweet_preview_api_enabled":           true,
}

// SearchTwitter performs a Twitter GraphQL SearchTimeline query using credentials
// loaded from environment variables. Returns ErrLoginRequired if no auth is set.
func SearchTwitter(ctx types.SearchContext, authToken, ct0 string) (*types.NormalizedSearchResult, error) {
	if authToken == "" || ct0 == "" {
		// Try loading from environment.
		auth, err := LoadAuth()
		if err != nil {
			return nil, err
		}
		authToken = auth.AuthToken
		ct0 = auth.CT0
	}

	limit := ctx.Options.Limit
	if limit <= 0 {
		limit = 20
	}
	product := "Top"
	if ctx.Options.Sort == "recent" {
		product = "Latest"
	}

	apiURL := BuildSearchTimelineURL(ctx.Query, limit, product)
	return SearchTwitterWithURL(apiURL, ctx.Query, authToken, ct0, limit, product)
}

// SearchTwitterWithURL performs a SearchTimeline request against the given apiURL.
// Allows injecting a test server URL in unit tests.
func SearchTwitterWithURL(apiURL, query, authToken, ct0 string, limit int, product string) (*types.NormalizedSearchResult, error) {
	auth := &Auth{AuthToken: authToken, CT0: ct0}

	ctx, cancel := context.WithTimeout(context.Background(), graphQLTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to build request: "+err.Error())
	}

	SetChromeHeaders(req, auth)

	resp, err := sharedClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "Twitter search request timed out")
		}
		errStr := err.Error()
		if strings.Contains(errStr, "handshake") || strings.Contains(errStr, "tls") {
			return nil, types.NewWebxError(types.ErrTLSBlocked, "TLS error connecting to Twitter: "+errStr)
		}
		return nil, types.NewWebxError(types.ErrFetchFailed, "request failed: "+errStr)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// proceed
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, types.NewWebxError(
			types.ErrLoginRequired,
			fmt.Sprintf("Twitter auth rejected (HTTP %d). Verify TWITTER_AUTH_TOKEN and TWITTER_CT0 are current.", resp.StatusCode),
		)
	case http.StatusTooManyRequests:
		return nil, types.NewWebxError(types.ErrRateLimited, "Twitter rate limited (HTTP 429)")
	case http.StatusNotFound, 422:
		return nil, types.NewWebxError(
			types.ErrAntiBot,
			fmt.Sprintf("Twitter returned HTTP %d — queryId may be stale. Update SearchTimelineQueryID.", resp.StatusCode),
		)
	default:
		return nil, types.NewWebxError(types.ErrFetchFailed, "unexpected Twitter HTTP status: "+resp.Status)
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzr, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return nil, types.NewWebxError(types.ErrBackendFailed, "failed to create gzip reader: "+gzErr.Error())
		}
		defer gzr.Close()
		reader = gzr
	}

	const maxBody = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(reader, maxBody))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to read response body: "+err.Error())
	}

	tweets, err := ParseSearchTimelineResponse(json.RawMessage(body))
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to parse Twitter search response: "+err.Error())
	}

	items := make([]types.SearchResultItem, 0, len(tweets))
	for _, tw := range tweets {
		tweetURL := fmt.Sprintf("https://x.com/%s/status/%s", tw.Author.ScreenName, tw.ID)
		score := 0.0
		if tw.Metrics != nil {
			score = float64(tw.Metrics["favorite_count"])
		}
		items = append(items, types.SearchResultItem{
			Title:   tw.Text,
			URL:     tweetURL,
			Author:  tw.Author.ScreenName,
			Date:    tw.CreatedAt,
			Score:   score,
			Kind:    types.KindThread,
			Meta: map[string]any{
				"metrics":      tw.Metrics,
				"author_name":  tw.Author.Name,
			},
		})
	}

	return &types.NormalizedSearchResult{
		Items:   items,
		Query:   query,
		Backend: "twitter_graphql",
		HasMore: len(tweets) >= limit,
	}, nil
}

// ParseSearchTimelineResponse parses the Twitter GraphQL SearchTimeline JSON response.
// JSON path: data → search_by_raw_query → search_timeline → timeline → instructions[]
func ParseSearchTimelineResponse(raw json.RawMessage) ([]Tweet, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}

	dataRaw, ok := top["data"]
	if !ok {
		return nil, nil
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		return nil, err
	}

	sbrqRaw, ok := data["search_by_raw_query"]
	if !ok {
		return nil, nil
	}

	var sbrq map[string]json.RawMessage
	if err := json.Unmarshal(sbrqRaw, &sbrq); err != nil {
		return nil, err
	}

	stRaw, ok := sbrq["search_timeline"]
	if !ok {
		return nil, nil
	}

	var st map[string]json.RawMessage
	if err := json.Unmarshal(stRaw, &st); err != nil {
		return nil, err
	}

	timelineRaw, ok := st["timeline"]
	if !ok {
		return nil, nil
	}

	var timeline map[string]json.RawMessage
	if err := json.Unmarshal(timelineRaw, &timeline); err != nil {
		return nil, err
	}

	instructionsRaw, ok := timeline["instructions"]
	if !ok {
		return nil, nil
	}

	var instructions []json.RawMessage
	if err := json.Unmarshal(instructionsRaw, &instructions); err != nil {
		return nil, err
	}

	var tweets []Tweet
	for _, instrRaw := range instructions {
		var instr map[string]json.RawMessage
		if json.Unmarshal(instrRaw, &instr) != nil {
			continue
		}
		entriesRaw, ok := instr["entries"]
		if !ok {
			continue
		}
		var entries []json.RawMessage
		if json.Unmarshal(entriesRaw, &entries) != nil {
			continue
		}
		for _, entryRaw := range entries {
			tweets = append(tweets, extractTweetsFromEntry(entryRaw)...)
		}
	}

	return tweets, nil
}

// BuildSearchTimelineURL constructs the Twitter GraphQL SearchTimeline URL.
func BuildSearchTimelineURL(query string, count int, product string) string {
	variables := searchTimelineVariables{
		RawQuery:    query,
		Count:       count,
		Product:     product,
		QuerySource: "typed_query",
	}
	variablesJSON, _ := json.Marshal(variables)

	compactFeatures := make(map[string]bool)
	for k, v := range searchTimelineFeatures {
		if v {
			compactFeatures[k] = v
		}
	}
	featuresJSON, _ := json.Marshal(compactFeatures)

	params := url.Values{}
	params.Set("variables", string(variablesJSON))
	params.Set("features", string(featuresJSON))

	return fmt.Sprintf(
		"https://x.com/i/api/graphql/%s/SearchTimeline?%s",
		SearchTimelineQueryID, params.Encode(),
	)
}

