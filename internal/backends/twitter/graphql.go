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
	"time"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

const (
	// TweetDetailQueryID is the GraphQL operation ID for TweetDetail.
	// Hardcoded from twitter-openapi's fallback table; if requests start
	// returning 404/422, update from:
	// https://github.com/fa0311/twitter-openapi/blob/main/src/config/placeholder.json
	TweetDetailQueryID = "rU08O-YiXdr0IZfE7qaUMg"

	// TweetResultByRestIdQueryID is the GraphQL operation ID for
	// TweetResultByRestId. This endpoint returns a single tweet result with
	// the full Draft.js content_state for X Articles, unlike TweetDetail
	// which only returns {title, preview_text} for articles.
	TweetResultByRestIdQueryID = "7xflPyRiUxGVbJd4uWmbfg"

	graphQLTimeout = 30 * time.Second
)

// tweetDetailVariables is the JSON variables payload for TweetDetail.
type tweetDetailVariables struct {
	FocalTweetID                           string `json:"focalTweetId"`
	WithRuxInjections                      bool   `json:"with_rux_injections"`
	RankingMode                            string `json:"rankingMode"`
	IncludePromotedContent                 bool   `json:"includePromotedContent"`
	WithCommunity                          bool   `json:"withCommunity"`
	WithQuickPromoteEligibilityTweetFields bool   `json:"withQuickPromoteEligibilityTweetFields"`
	WithBirdwatchNotes                     bool   `json:"withBirdwatchNotes"`
	WithVoice                              bool   `json:"withVoice"`
}

// tweetDetailFeatures is the feature-flag map required by TweetDetail.
// Only True values are sent to avoid 414 URI Too Long.
// Sourced from twitter-cli/twitter_cli/graphql.py _DEFAULT_FEATURES.
var tweetDetailFeatures = map[string]bool{
	"responsive_web_graphql_exclude_directive_enabled":                        true,
	"creator_subscriptions_tweet_preview_api_enabled":                        true,
	"responsive_web_graphql_timeline_navigation_enabled":                     true,
	"c9s_tweet_anatomy_moderator_badge_enabled":                              true,
	"tweetypie_unmention_optimization_enabled":                               true,
	"responsive_web_edit_tweet_api_enabled":                                  true,
	"graphql_is_translatable_rweb_tweet_is_translatable_enabled":             true,
	"view_counts_everywhere_api_enabled":                                     true,
	"longform_notetweets_consumption_enabled":                                true,
	"responsive_web_twitter_article_tweet_consumption_enabled":               true,
	"longform_notetweets_rich_text_read_enabled":                             true,
	"longform_notetweets_inline_media_enabled":                               true,
	"rweb_video_timestamps_enabled":                                          true,
	"responsive_web_media_download_video_enabled":                            true,
	"freedom_of_speech_not_reach_fetch_enabled":                              true,
	"standardized_nudges_misinfo":                                            true,
}

// FetchTweetDetail calls the TweetDetail GraphQL endpoint and returns the
// raw JSON response body. The caller is responsible for parsing.
//
// Uses backends.NewUTLSClient() for Chrome TLS fingerprinting (see tlsclient.go
// for HTTP/2 ALPN handling).
// sharedClient is reused across requests to avoid repeated TLS handshakes.
var sharedClient = backends.NewUTLSClient()

func FetchTweetDetail(tweetID string, auth *Auth) (json.RawMessage, error) {
	variables := tweetDetailVariables{
		FocalTweetID:                           tweetID,
		WithRuxInjections:                      false,
		RankingMode:                            "Relevance",
		IncludePromotedContent:                 false,
		WithCommunity:                          true,
		WithQuickPromoteEligibilityTweetFields: false,
		WithBirdwatchNotes:                     true,
		WithVoice:                              true,
	}
	return doGraphQLGet(graphQLRequest{
		Operation:    "TweetDetail",
		QueryID:      TweetDetailQueryID,
		Variables:    variables,
		Features:     tweetDetailFeatures,
		FieldToggles: nil,
		Auth:         auth,
	})
}

// articleFeatures + articleFieldToggles request the full Draft.js
// content_state for X Articles via TweetResultByRestId. Sourced from
// twitter-cli/twitter_cli/client.py:fetch_article (MIT) and
// fa0311/twitter-openapi placeholder.json.
var articleFeatures = map[string]bool{
	"longform_notetweets_consumption_enabled":                  true,
	"responsive_web_twitter_article_tweet_consumption_enabled": true,
	"longform_notetweets_rich_text_read_enabled":               true,
	"longform_notetweets_inline_media_enabled":                 true,
	"articles_preview_enabled":                                 true,
	"responsive_web_graphql_exclude_directive_enabled":         true,
}

var articleFieldToggles = map[string]bool{
	"withArticleRichContentState": true,
	"withArticlePlainText":        true,
}

// FetchArticleByTweetID retrieves the full X Article payload (including
// Draft.js content_state) for a tweet that hosts long-form content.
//
// Returns the raw GraphQL response; caller parses via
// ParseTweetResultByRestIdResponse.
func FetchArticleByTweetID(tweetID string, auth *Auth) (json.RawMessage, error) {
	variables := map[string]any{
		"tweetId":                tweetID,
		"withCommunity":          false,
		"includePromotedContent": false,
		"withVoice":              false,
	}
	return doGraphQLGet(graphQLRequest{
		Operation:    "TweetResultByRestId",
		QueryID:      TweetResultByRestIdQueryID,
		Variables:    variables,
		Features:     articleFeatures,
		FieldToggles: articleFieldToggles,
		Auth:         auth,
	})
}

type graphQLRequest struct {
	Operation    string
	QueryID      string
	Variables    any             // serialized to JSON in `variables` query param
	Features     map[string]bool // only true values are sent
	FieldToggles map[string]bool // optional; sent verbatim when non-empty
	Auth         *Auth
}

// doGraphQLGet performs a Twitter/X GraphQL GET request and returns the raw
// response body. Compact-features rule: drop false flags to keep URL under
// the server's URI length cap (we have seen 414 otherwise).
func doGraphQLGet(req graphQLRequest) (json.RawMessage, error) {
	variablesJSON, err := json.Marshal(req.Variables)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to marshal variables: "+err.Error())
	}

	compactFeatures := make(map[string]bool)
	for k, v := range req.Features {
		if v {
			compactFeatures[k] = v
		}
	}
	featuresJSON, _ := json.Marshal(compactFeatures)

	params := url.Values{}
	params.Set("variables", string(variablesJSON))
	params.Set("features", string(featuresJSON))
	if len(req.FieldToggles) > 0 {
		togglesJSON, _ := json.Marshal(req.FieldToggles)
		params.Set("fieldToggles", string(togglesJSON))
	}

	apiURL := fmt.Sprintf(
		"https://x.com/i/api/graphql/%s/%s?%s",
		req.QueryID, req.Operation, params.Encode(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), graphQLTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to build request: "+err.Error())
	}
	SetChromeHeaders(httpReq, req.Auth)

	resp, err := sharedClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "Twitter GraphQL request timed out")
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
			fmt.Sprintf("Twitter %s returned HTTP %d — queryId may be stale. Update from https://github.com/fa0311/twitter-openapi", req.Operation, resp.StatusCode),
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

	const maxTwitterBody = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(reader, maxTwitterBody))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to read response body: "+err.Error())
	}
	return json.RawMessage(body), nil
}
