// Package twitter provides the Twitter/X.com adapter for webx.
// It fetches tweets via Twitter's internal GraphQL API using a Chrome TLS
// fingerprint and authenticated session cookies.
package twitter

import (
	"errors"
	"fmt"
	"regexp"

	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

var tweetIDRe = regexp.MustCompile(`/status/(\d+)(?:[/?#]|$)`)

type twitterAdapter struct{}

// TwitterExtractData is the structured data returned by Extract.
type TwitterExtractData struct {
	Tweets []twitterbe.Tweet `json:"tweets"`
}

// New returns a new Twitter adapter instance.
func New() types.ExtractableAdapter {
	return &twitterAdapter{}
}

func (a *twitterAdapter) ID() string         { return "twitter" }
func (a *twitterAdapter) Priority() int       { return 90 }
func (a *twitterAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindThread, types.KindMetadata}
}

// Match returns true for x.com and twitter.com URLs.
func (a *twitterAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	return host == "x.com" || host == "twitter.com" ||
		host == "www.x.com" || host == "www.twitter.com"
}

// tweetID extracts the numeric tweet ID from a /status/<id> URL path.
// Returns "" when the URL is not a tweet URL (e.g. x.com/home).
func tweetID(ctx types.MatchContext) string {
	m := tweetIDRe.FindStringSubmatch(ctx.URL.Path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// fetchTweets is the shared core for Read and Extract. It validates the URL,
// authenticates, fetches and parses the tweet detail response.
func (a *twitterAdapter) fetchTweets(ctx types.RunContext, step string) ([]twitterbe.Tweet, string, error) {
	id := tweetID(ctx.MatchContext)
	if id == "" {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceNoMatch,
			Adapter: "twitter",
			Detail:  "URL does not contain a /status/<id> path: " + ctx.URL.String(),
		})
		return nil, "", types.NewWebxError(types.ErrNoMatch,
			"URL does not point to a tweet (no /status/<id> found): "+ctx.URL.String())
	}

	auth, err := twitterbe.LoadAuth()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceLoginRequired,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, "", err
	}

	raw, err := twitterbe.FetchTweetDetail(id, auth)
	if err != nil {
		reason := traceReason(err)
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  reason,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, "", err
	}

	tweets, err := twitterbe.ParseTweetDetailResponse(raw)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceBackendFailed,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  "parse error: " + err.Error(),
		})
		return nil, "", types.NewWebxError(types.ErrBackendFailed, "failed to parse Twitter response: "+err.Error())
	}

	if len(tweets) == 0 {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceEmptyContent,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  "GraphQL response contained no tweet entries",
		})
		return nil, "", types.NewWebxError(types.ErrContentEmpty, "no tweets found in Twitter GraphQL response")
	}

	title := "Tweet"
	if tweets[0].Author.ScreenName != "" {
		title = fmt.Sprintf("@%s on Twitter", tweets[0].Author.ScreenName)
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: "twitter_graphql",
		Detail:  fmt.Sprintf("parsed %d tweets from TweetDetail GraphQL response", len(tweets)),
	})

	return tweets, title, nil
}

// Search implements types.SearchableAdapter for Twitter via GraphQL SearchTimeline.
// Requires TWITTER_AUTH_TOKEN and TWITTER_CT0 environment variables.
func (a *twitterAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	result, err := twitterbe.SearchTwitter(ctx, "", "")
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	result.Query = ctx.Query

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.search",
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: "twitter_graphql",
		Detail:  fmt.Sprintf("search returned %d results for %q", len(result.Items), ctx.Query),
	})

	return result, nil
}

func (a *twitterAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	tweets, title, err := a.fetchTweets(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := twitterbe.RenderMarkdown(tweets)

	return &types.NormalizedReadResult{
		Title:    &title,
		Markdown: &markdown,
		HTML:     nil,
		Backend:  "twitter_graphql",
	}, nil
}

func (a *twitterAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	tweets, title, err := a.fetchTweets(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := twitterbe.RenderMarkdown(tweets)

	return &types.NormalizedExtractResult{
		Title:    &title,
		Markdown: &markdown,
		HTML:     nil,
		Data:     TwitterExtractData{Tweets: tweets},
		Backend:  "twitter_graphql",
	}, nil
}

// traceReason maps a WebxError code to the appropriate TraceReason.
func traceReason(err error) types.TraceReason {
	var wxErr *types.WebxError
	if !errors.As(err, &wxErr) {
		return types.TraceBackendFailed
	}
	switch wxErr.Code {
	case types.ErrLoginRequired:
		return types.TraceLoginRequired
	case types.ErrRateLimited:
		return types.TraceRateLimited
	case types.ErrTLSBlocked:
		return types.TraceAntiBot
	case types.ErrAntiBot:
		return types.TraceAntiBot
	default:
		return types.TraceBackendFailed
	}
}
