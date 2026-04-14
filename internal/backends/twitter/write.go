package twitter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/oaooao/webx/internal/types"
)

const (
	// CreateTweetQueryID is the GraphQL operation ID for CreateTweet mutation.
	// Update from https://github.com/fa0311/twitter-openapi/blob/main/src/config/placeholder.json
	// if requests start returning 404/422.
	CreateTweetQueryID = "tTsjMKyhajZvK4q76mpIBg"

	// FavoriteTweetQueryID is the GraphQL operation ID for FavoriteTweet mutation.
	FavoriteTweetQueryID = "lI07N6Otwv1PhnEgXILM7A"

	// CreateRetweetQueryID is the GraphQL operation ID for CreateRetweet mutation.
	CreateRetweetQueryID = "ojPdsZsimiJrUGLR1sjUtA"
)

// graphQLMutationEndpoint is the base URL for Twitter's internal GraphQL API.
const graphQLMutationEndpoint = "https://x.com/i/api/graphql"

// createTweetVariables is the JSON variables payload for CreateTweet mutation.
type createTweetVariables struct {
	TweetText   string           `json:"tweet_text"`
	DarkRequest bool             `json:"dark_request"`
	Media       createTweetMedia `json:"media"`
	Reply       *createTweetReply `json:"reply,omitempty"`
}

type createTweetMedia struct {
	MediaEntities     []interface{} `json:"media_entities"`
	PossiblySensitive bool          `json:"possibly_sensitive"`
}

type createTweetReply struct {
	InReplyToTweetID    string        `json:"in_reply_to_tweet_id"`
	ExcludeReplyUserIDs []interface{} `json:"exclude_reply_user_ids"`
}

// createTweetFeatures are the feature flags required by CreateTweet.
var createTweetFeatures = map[string]bool{
	"responsive_web_graphql_exclude_directive_enabled":           true,
	"responsive_web_graphql_timeline_navigation_enabled":        true,
	"longform_notetweets_consumption_enabled":                   true,
	"responsive_web_twitter_article_tweet_consumption_enabled":  true,
	"rweb_video_timestamps_enabled":                             true,
	"creator_subscriptions_tweet_preview_api_enabled":           true,
	"tweetypie_unmention_optimization_enabled":                  true,
	"responsive_web_edit_tweet_api_enabled":                     true,
	"graphql_is_translatable_rweb_tweet_is_translatable_enabled": true,
	"view_counts_everywhere_api_enabled":                        true,
	"longform_notetweets_rich_text_read_enabled":                true,
	"longform_notetweets_inline_media_enabled":                  true,
	"freedom_of_speech_not_reach_fetch_enabled":                 true,
	"standardized_nudges_misinfo":                               true,
}

// CreateTweet posts a new tweet using production endpoint.
func CreateTweet(content string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for CreateTweet")
	}
	apiURL := fmt.Sprintf("%s/%s/CreateTweet", graphQLMutationEndpoint, CreateTweetQueryID)
	return CreateTweetWithURL(apiURL, content, auth)
}

// CreateTweetWithURL posts a tweet to the given apiURL (allows test injection).
func CreateTweetWithURL(apiURL, content string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil || auth.AuthToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for CreateTweet")
	}
	variables := createTweetVariables{
		TweetText:   content,
		DarkRequest: false,
		Media: createTweetMedia{
			MediaEntities:     []interface{}{},
			PossiblySensitive: false,
		},
	}
	raw, err := doCreateTweetPOST(apiURL, variables, auth)
	if err != nil {
		return nil, err
	}
	tweetID, err := extractCreatedTweetID(raw)
	if err != nil {
		return nil, err
	}
	tweetURL := fmt.Sprintf("https://x.com/i/status/%s", tweetID)
	return &types.NormalizedWriteResult{
		Success:     true,
		Action:      string(types.ActionPost),
		ResourceURL: tweetURL,
		Message:     "Tweet posted successfully",
		Backend:     "twitter_graphql",
	}, nil
}

// ReplyTweet posts a reply using production endpoint.
func ReplyTweet(content string, inReplyToID string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for ReplyTweet")
	}
	apiURL := fmt.Sprintf("%s/%s/CreateTweet", graphQLMutationEndpoint, CreateTweetQueryID)
	return ReplyTweetWithURL(apiURL, content, inReplyToID, auth)
}

// ReplyTweetWithURL posts a reply to the given apiURL (allows test injection).
func ReplyTweetWithURL(apiURL, content, inReplyToID string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil || auth.AuthToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for ReplyTweet")
	}
	variables := createTweetVariables{
		TweetText:   content,
		DarkRequest: false,
		Media: createTweetMedia{
			MediaEntities:     []interface{}{},
			PossiblySensitive: false,
		},
		Reply: &createTweetReply{
			InReplyToTweetID:    inReplyToID,
			ExcludeReplyUserIDs: []interface{}{},
		},
	}
	raw, err := doCreateTweetPOST(apiURL, variables, auth)
	if err != nil {
		return nil, err
	}
	tweetID, err := extractCreatedTweetID(raw)
	if err != nil {
		return nil, err
	}
	tweetURL := fmt.Sprintf("https://x.com/i/status/%s", tweetID)
	return &types.NormalizedWriteResult{
		Success:     true,
		Action:      string(types.ActionReply),
		ResourceURL: tweetURL,
		Message:     "Reply posted successfully",
		Backend:     "twitter_graphql",
	}, nil
}

// FavoriteTweet likes a tweet using production endpoint.
func FavoriteTweet(tweetID string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for FavoriteTweet")
	}
	apiURL := fmt.Sprintf("%s/%s/FavoriteTweet", graphQLMutationEndpoint, FavoriteTweetQueryID)
	return FavoriteTweetWithURL(apiURL, tweetID, auth)
}

// FavoriteTweetWithURL likes a tweet via apiURL (allows test injection).
func FavoriteTweetWithURL(apiURL, tweetID string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil || auth.AuthToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for FavoriteTweet")
	}
	variables := map[string]string{"tweet_id": tweetID}
	variablesJSON, _ := json.Marshal(variables)
	payload := map[string]json.RawMessage{"variables": variablesJSON}
	body, _ := json.Marshal(payload)

	_, err := doMutationPOST(apiURL, body, auth)
	if err != nil {
		return nil, err
	}
	return &types.NormalizedWriteResult{
		Success: true,
		Action:  string(types.ActionReact),
		Message: "Liked tweet " + tweetID,
		Backend: "twitter_graphql",
	}, nil
}

// RetweetTweet retweets a tweet using production endpoint.
func RetweetTweet(tweetID string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for RetweetTweet")
	}
	apiURL := fmt.Sprintf("%s/%s/CreateRetweet", graphQLMutationEndpoint, CreateRetweetQueryID)
	return RetweetTweetWithURL(apiURL, tweetID, auth)
}

// RetweetTweetWithURL retweets via apiURL (allows test injection).
func RetweetTweetWithURL(apiURL, tweetID string, auth *Auth) (*types.NormalizedWriteResult, error) {
	if auth == nil || auth.AuthToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired, "auth is required for RetweetTweet")
	}
	variables := map[string]string{"tweet_id": tweetID}
	variablesJSON, _ := json.Marshal(variables)
	payload := map[string]json.RawMessage{"variables": variablesJSON}
	body, _ := json.Marshal(payload)

	_, err := doMutationPOST(apiURL, body, auth)
	if err != nil {
		return nil, err
	}
	return &types.NormalizedWriteResult{
		Success: true,
		Action:  string(types.ActionReact),
		Message: "Retweeted tweet " + tweetID,
		Backend: "twitter_graphql",
	}, nil
}

// doCreateTweetPOST marshals CreateTweet variables and sends the POST request.
func doCreateTweetPOST(apiURL string, variables createTweetVariables, auth *Auth) (json.RawMessage, error) {
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to marshal variables: "+err.Error())
	}
	featuresJSON, err := json.Marshal(createTweetFeatures)
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to marshal features: "+err.Error())
	}
	payload := map[string]json.RawMessage{
		"variables": variablesJSON,
		"features":  featuresJSON,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to marshal payload: "+err.Error())
	}
	return doMutationPOST(apiURL, body, auth)
}

// doMutationPOST sends a POST request to a Twitter GraphQL mutation endpoint.
func doMutationPOST(apiURL string, body []byte, auth *Auth) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), graphQLTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to build request: "+err.Error())
	}

	SetChromeHeaders(req, auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "Twitter mutation request timed out")
		}
		errStr := err.Error()
		if strings.Contains(errStr, "handshake") || strings.Contains(errStr, "tls") {
			return nil, types.NewWebxError(types.ErrTLSBlocked, "TLS error: "+errStr)
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
			fmt.Sprintf("Twitter auth rejected (HTTP %d). Verify TWITTER_AUTH_TOKEN and TWITTER_CT0.", resp.StatusCode),
		)
	case http.StatusTooManyRequests:
		return nil, types.NewWebxError(types.ErrRateLimited, "Twitter rate limited (HTTP 429)")
	case http.StatusNotFound, 422:
		return nil, types.NewWebxError(
			types.ErrAntiBot,
			fmt.Sprintf("Twitter returned HTTP %d — queryId may be stale.", resp.StatusCode),
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
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to read response: "+err.Error())
	}

	return json.RawMessage(respBody), nil
}

// extractCreatedTweetID parses the CreateTweet response to get the new tweet ID.
// Path: data → create_tweet → tweet_results → result → rest_id
func extractCreatedTweetID(raw json.RawMessage) (string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return "", types.NewWebxError(types.ErrBackendFailed, "invalid JSON in CreateTweet response: "+err.Error())
	}

	dataRaw, ok := top["data"]
	if !ok {
		return "", types.NewWebxError(types.ErrBackendFailed, "CreateTweet response missing 'data' key")
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		return "", types.NewWebxError(types.ErrBackendFailed, "failed to parse data: "+err.Error())
	}

	ctRaw, ok := data["create_tweet"]
	if !ok {
		return "", types.NewWebxError(types.ErrBackendFailed, "CreateTweet response missing 'create_tweet'")
	}

	var ct map[string]json.RawMessage
	if err := json.Unmarshal(ctRaw, &ct); err != nil {
		return "", types.NewWebxError(types.ErrBackendFailed, "failed to parse create_tweet: "+err.Error())
	}

	trRaw, ok := ct["tweet_results"]
	if !ok {
		return "", types.NewWebxError(types.ErrBackendFailed, "missing 'tweet_results'")
	}

	var tr map[string]json.RawMessage
	if err := json.Unmarshal(trRaw, &tr); err != nil {
		return "", types.NewWebxError(types.ErrBackendFailed, "failed to parse tweet_results: "+err.Error())
	}

	resultRaw, ok := tr["result"]
	if !ok {
		return "", types.NewWebxError(types.ErrBackendFailed, "missing 'result' in tweet_results")
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return "", types.NewWebxError(types.ErrBackendFailed, "failed to parse result: "+err.Error())
	}

	var tweetID string
	if err := json.Unmarshal(result["rest_id"], &tweetID); err != nil || tweetID == "" {
		return "", types.NewWebxError(types.ErrBackendFailed, "could not extract tweet ID from response")
	}

	return tweetID, nil
}
