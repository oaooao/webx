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
	TweetDetailQueryID = "xIYgDwjboktoFeXe_fgacw"

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
	client := sharedClient

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

	variablesJSON, _ := json.Marshal(variables)

	// Only include True feature flags — omitting False values keeps URL under
	// server limits (Twitter returns 414 if the URL is too long).
	compactFeatures := make(map[string]bool)
	for k, v := range tweetDetailFeatures {
		if v {
			compactFeatures[k] = v
		}
	}
	featuresJSON, _ := json.Marshal(compactFeatures)

	params := url.Values{}
	params.Set("variables", string(variablesJSON))
	params.Set("features", string(featuresJSON))

	apiURL := fmt.Sprintf(
		"https://x.com/i/api/graphql/%s/TweetDetail?%s",
		TweetDetailQueryID, params.Encode(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), graphQLTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to build request: "+err.Error())
	}

	SetChromeHeaders(req, auth)

	resp, err := client.Do(req)
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
			fmt.Sprintf("Twitter returned HTTP %d — queryId may be stale. Update TweetDetailQueryID from https://github.com/fa0311/twitter-openapi", resp.StatusCode),
		)
	default:
		return nil, types.NewWebxError(types.ErrFetchFailed, "unexpected Twitter HTTP status: "+resp.Status)
	}

	// Twitter almost always gzip-encodes responses; decompress if needed.
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzr, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return nil, types.NewWebxError(types.ErrBackendFailed, "failed to create gzip reader: "+gzErr.Error())
		}
		defer gzr.Close()
		reader = gzr
	}

	const maxTwitterBody = 10 * 1024 * 1024 // 10 MB
	body, err := io.ReadAll(io.LimitReader(reader, maxTwitterBody))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to read response body: "+err.Error())
	}

	return json.RawMessage(body), nil
}
