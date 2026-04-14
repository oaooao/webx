package twitter

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

const (
	// SearchTimelineQueryID is the GraphQL operation ID for SearchTimeline.
	// Update from https://github.com/fa0311/twitter-openapi/blob/main/src/config/placeholder.json
	// if requests start returning 404/422.
	SearchTimelineQueryID = "MJpyQGqgklrVl_0X9gNy3A"
)

// searchTimelineVariables is the JSON variables payload for SearchTimeline.
// Note: SearchTimeline rejects unknown variables — only include fields it expects.
type searchTimelineVariables struct {
	RawQuery    string `json:"rawQuery"`
	Count       int    `json:"count"`
	Product     string `json:"product"` // "Top" or "Latest"
	QuerySource string `json:"querySource"`
}

// searchTimelineFeatures is a minimal set of feature flags for SearchTimeline.
// Only True-valued flags are sent — adding too many inflates URL length and
// causes 414/431 errors. Derived from twitter-cli's working configuration.
var searchTimelineFeatures = map[string]bool{
	"responsive_web_graphql_exclude_directive_enabled":           true,
	"creator_subscriptions_tweet_preview_api_enabled":            true,
	"responsive_web_graphql_timeline_navigation_enabled":         true,
	"c9s_tweet_anatomy_moderator_badge_enabled":                  true,
	"tweetypie_unmention_optimization_enabled":                   true,
	"responsive_web_edit_tweet_api_enabled":                      true,
	"graphql_is_translatable_rweb_tweet_is_translatable_enabled": true,
	"view_counts_everywhere_api_enabled":                         true,
	"longform_notetweets_consumption_enabled":                    true,
	"responsive_web_twitter_article_tweet_consumption_enabled":   true,
	"longform_notetweets_rich_text_read_enabled":                 true,
	"rweb_video_timestamps_enabled":                              true,
	"responsive_web_media_download_video_enabled":                true,
	"freedom_of_speech_not_reach_fetch_enabled":                  true,
	"standardized_nudges_misinfo":                                true,
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

	// Twitter migrated SearchTimeline to POST. Build JSON body instead of URL params.
	body := buildSearchTimelineBody(query, limit, product)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(body))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to build request: "+err.Error())
	}

	SetChromeHeaders(req, auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := backends.StdClient().Do(req)
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
	respBody, err := io.ReadAll(io.LimitReader(reader, maxBody))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to read response body: "+err.Error())
	}

	tweets, err := ParseSearchTimelineResponse(json.RawMessage(respBody))
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
			Title:  tw.Text,
			URL:    tweetURL,
			Author: tw.Author.ScreenName,
			Date:   tw.CreatedAt,
			Score:  score,
			Kind:   types.KindThread,
			Meta: map[string]any{
				"metrics":     tw.Metrics,
				"author_name": tw.Author.Name,
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

// searchTimelineFieldToggles are required by the SearchTimeline endpoint.
// All set to false — these control optional response fields.
var searchTimelineFieldToggles = map[string]bool{
	"withPayments":                false,
	"withAuxiliaryUserLabels":     false,
	"withArticleRichContentState": false,
	"withArticlePlainText":        false,
	"withArticleSummaryText":      false,
	"withArticleVoiceOver":        false,
	"withGrokAnalyze":             false,
	"withDisallowedReplyControls": false,
}

// BuildSearchTimelineURL returns the base URL for SearchTimeline POST requests.
func BuildSearchTimelineURL(query string, count int, product string) string {
	return fmt.Sprintf("https://x.com/i/api/graphql/%s/SearchTimeline", SearchTimelineQueryID)
}

// buildSearchTimelineBody constructs the JSON body for SearchTimeline POST requests.
func buildSearchTimelineBody(query string, count int, product string) string {
	compactFeatures := make(map[string]bool)
	for k, v := range searchTimelineFeatures {
		if v {
			compactFeatures[k] = v
		}
	}

	body := map[string]any{
		"variables": map[string]any{
			"rawQuery":    query,
			"count":       count,
			"product":     product,
			"querySource": "typed_query",
		},
		"features": compactFeatures,
		"queryId":  SearchTimelineQueryID,
	}

	b, _ := json.Marshal(body)
	return string(b)
}
