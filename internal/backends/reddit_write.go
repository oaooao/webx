package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/oaooao/webx/internal/types"
)

const (
	redditOAuthBase    = "https://oauth.reddit.com"
	redditWriteTimeout = 30 * time.Second
)

// RedditSubmitURL returns the production URL for /api/submit.
func RedditSubmitURL() string { return redditOAuthBase + "/api/submit" }

// RedditCommentURL returns the production URL for /api/comment.
func RedditCommentURL() string { return redditOAuthBase + "/api/comment" }

// RedditVoteURL returns the production URL for /api/vote.
func RedditVoteURL() string { return redditOAuthBase + "/api/vote" }

// redditAPIResponse is the common wrapper for Reddit API responses.
type redditAPIResponse struct {
	JSON struct {
		Errors [][]json.RawMessage `json:"errors"`
		Data   struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			URL  string `json:"url"`
			Things []struct {
				Kind string `json:"kind"`
				Data struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					Body string `json:"body"`
				} `json:"data"`
			} `json:"things"`
		} `json:"data"`
	} `json:"json"`
}

// PostReddit submits a new self-post to a subreddit via the Reddit OAuth API.
// apiURL allows test injection; in production use redditOAuthBase + "/api/submit".
func PostReddit(apiURL, subreddit, title, content, accessToken string) (*types.NormalizedWriteResult, error) {
	if accessToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired,
			"REDDIT_ACCESS_TOKEN is required for posting. "+
				"Set the environment variable with a valid Reddit OAuth access token.")
	}

	params := url.Values{}
	params.Set("kind", "self")
	params.Set("sr", subreddit)
	params.Set("title", title)
	params.Set("text", content)
	params.Set("api_type", "json")

	raw, err := doRedditWritePOST(apiURL, params, accessToken)
	if err != nil {
		return nil, err
	}

	var resp redditAPIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to parse Reddit submit response: "+err.Error())
	}

	if len(resp.JSON.Errors) > 0 {
		if errMsg := redditErrorMsg(resp.JSON.Errors[0]); errMsg != "" {
			return nil, types.NewWebxError(types.ErrBackendFailed, "Reddit API error: "+errMsg)
		}
	}

	postURL := resp.JSON.Data.URL
	if postURL == "" {
		postURL = fmt.Sprintf("https://www.reddit.com/r/%s/", subreddit)
	}

	return &types.NormalizedWriteResult{
		Success:     true,
		Action:      string(types.ActionPost),
		ResourceURL: postURL,
		Message:     fmt.Sprintf("Posted to r/%s: %s", subreddit, title),
		Backend:     "reddit_oauth",
	}, nil
}

// CommentReddit posts a comment (reply) on a Reddit thing (post or comment).
// thingID is the fullname, e.g. "t3_abc123" (post) or "t1_def456" (comment).
// apiURL allows test injection; in production use redditOAuthBase + "/api/comment".
func CommentReddit(apiURL, thingID, content, accessToken string) (*types.NormalizedWriteResult, error) {
	if accessToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired,
			"REDDIT_ACCESS_TOKEN is required for commenting.")
	}

	params := url.Values{}
	params.Set("thing_id", thingID)
	params.Set("text", content)
	params.Set("api_type", "json")

	raw, err := doRedditWritePOST(apiURL, params, accessToken)
	if err != nil {
		return nil, err
	}

	var resp redditAPIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to parse Reddit comment response: "+err.Error())
	}

	if len(resp.JSON.Errors) > 0 {
		if errMsg := redditErrorMsg(resp.JSON.Errors[0]); errMsg != "" {
			return nil, types.NewWebxError(types.ErrBackendFailed, "Reddit API error: "+errMsg)
		}
	}

	commentID := ""
	if len(resp.JSON.Data.Things) > 0 {
		commentID = resp.JSON.Data.Things[0].Data.Name
	}

	return &types.NormalizedWriteResult{
		Success:     true,
		Action:      string(types.ActionReply),
		ResourceURL: commentID,
		Message:     fmt.Sprintf("Commented on %s", thingID),
		Backend:     "reddit_oauth",
	}, nil
}

// VoteReddit votes on a Reddit thing.
// dir: 1 = upvote, -1 = downvote, 0 = remove vote.
// apiURL allows test injection; in production use redditOAuthBase + "/api/vote".
func VoteReddit(apiURL, thingID string, dir int, accessToken string) (*types.NormalizedWriteResult, error) {
	if accessToken == "" {
		return nil, types.NewWebxError(types.ErrLoginRequired,
			"REDDIT_ACCESS_TOKEN is required for voting.")
	}

	params := url.Values{}
	params.Set("id", thingID)
	params.Set("dir", fmt.Sprintf("%d", dir))

	_, err := doRedditWritePOST(apiURL, params, accessToken)
	if err != nil {
		return nil, err
	}

	action := "upvoted"
	if dir == -1 {
		action = "downvoted"
	} else if dir == 0 {
		action = "unvoted"
	}

	return &types.NormalizedWriteResult{
		Success: true,
		Action:  string(types.ActionReact),
		Message: fmt.Sprintf("%s %s", capitalize(action), thingID),
		Backend: "reddit_oauth",
	}, nil
}

// capitalize uppercases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// LoadRedditAccessToken loads the Reddit OAuth access token from environment variable.
func LoadRedditAccessToken() (string, error) {
	token := getEnv("REDDIT_ACCESS_TOKEN")
	if token == "" {
		return "", types.NewWebxError(types.ErrLoginRequired,
			"REDDIT_ACCESS_TOKEN environment variable is required for Reddit write operations. "+
				"See https://github.com/oaooao/webx#reddit-setup for setup instructions.")
	}
	return token, nil
}

// doRedditWritePOST sends a POST request to the Reddit OAuth API with form-encoded parameters.
func doRedditWritePOST(apiURL string, params url.Values, accessToken string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redditWriteTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL,
		strings.NewReader(params.Encode()))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to build request: "+err.Error())
	}

	req.Header.Set("User-Agent", redditUserAgent)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := sharedStdClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "Reddit API request timed out")
		}
		return nil, types.NewWebxError(types.ErrFetchFailed, "request failed: "+err.Error())
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// proceed
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, types.NewWebxError(types.ErrLoginRequired,
			fmt.Sprintf("Reddit auth rejected (HTTP %d). Verify REDDIT_ACCESS_TOKEN is valid.", resp.StatusCode))
	case http.StatusTooManyRequests:
		return nil, types.NewWebxError(types.ErrRateLimited, "Reddit rate limited (HTTP 429)")
	default:
		return nil, types.NewWebxError(types.ErrFetchFailed, "Reddit API HTTP "+resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, "failed to read response: "+err.Error())
	}

	return body, nil
}

// redditErrorMsg extracts a human-readable error message from a Reddit errors entry.
// Reddit returns errors as [[code, message, field], ...].
func redditErrorMsg(entry []json.RawMessage) string {
	if len(entry) < 2 {
		return ""
	}
	var msg string
	if err := json.Unmarshal(entry[1], &msg); err != nil {
		return string(entry[1])
	}
	return msg
}

// getEnv is a thin wrapper around os.Getenv for testability.
// Defined here as a var so tests can override it.
var getEnv = func(key string) string {
	return os.Getenv(key)
}
