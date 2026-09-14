// Package twitter provides the Twitter/X.com adapter for webx.
// It fetches tweets via Twitter's internal GraphQL API. Reads try the direct
// uTLS client with stored session cookies first; when those are missing or
// rejected (401/403) they fall back to the egolite transport, which runs the
// same GraphQL request inside the user's logged-in ego lite browser.
package twitter

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"

	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

const directBackend = "twitter_graphql"

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

// thread is the parsed TweetDetail conversation plus how it was fetched.
type thread struct {
	tweets    []twitterbe.Tweet
	focal     int // index of the requested tweet in tweets
	title     string
	transport transport
}

// fetchTweets is the shared core for Read and Extract. It validates the URL,
// fetches the TweetDetail conversation (focal tweet and first page of replies)
// through the transport chain and parses it.
func (a *twitterAdapter) fetchTweets(ctx types.RunContext, step string) (*thread, error) {
	id := tweetID(ctx.MatchContext)
	if id == "" {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceNoMatch,
			Adapter: "twitter",
			Detail:  "URL does not contain a /status/<id> path: " + ctx.URL.String(),
		})
		return nil, types.NewWebxError(types.ErrNoMatch,
			"URL does not point to a tweet (no /status/<id> found): "+ctx.URL.String())
	}

	tr, raw, err := fetchTweetDetail(ctx, step, id)
	if err != nil {
		return nil, err
	}

	tweets, err := twitterbe.ParseTweetDetailResponse(raw)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceBackendFailed,
			Adapter: "twitter",
			Backend: tr.backend,
			Detail:  "parse error: " + err.Error(),
		})
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to parse Twitter response: "+err.Error())
	}

	if len(tweets) == 0 {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceEmptyContent,
			Adapter: "twitter",
			Backend: tr.backend,
			Detail:  "GraphQL response contained no tweet entries",
		})
		return nil, types.NewWebxError(types.ErrContentEmpty, "no tweets found in Twitter GraphQL response")
	}

	focal := 0
	for i, t := range tweets {
		if t.ID == id {
			focal = i
			break
		}
	}

	title := "Tweet"
	if tweets[focal].Author.ScreenName != "" {
		title = fmt.Sprintf("@%s on Twitter", tweets[focal].Author.ScreenName)
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: tr.backend,
		Detail:  fmt.Sprintf("parsed %d tweets (%d replies) from TweetDetail GraphQL response", len(tweets), len(tweets)-focal-1),
	})

	// X Article enrichment: TweetDetail only returns {title, preview_text}
	// for articles. When the focal tweet has a title but empty body, fetch
	// the full content_state via TweetResultByRestId.
	if t := &tweets[focal]; t.ArticleTitle != "" && t.ArticleText == "" {
		articleRaw, articleErr := tr.article(t.ID)
		if articleErr != nil {
			ctx.Trace.Push(types.TraceEvent{
				Step:    step + ".article",
				Reason:  traceReason(articleErr),
				Adapter: "twitter",
				Backend: tr.backend,
				Detail:  "article enrich failed: " + articleErr.Error(),
			})
			// Non-fatal: keep the title-only article rather than failing the whole read.
		} else if enriched, parseErr := twitterbe.ParseTweetResultByRestIdResponse(articleRaw); parseErr != nil {
			ctx.Trace.Push(types.TraceEvent{
				Step:    step + ".article",
				Reason:  types.TraceBackendFailed,
				Adapter: "twitter",
				Backend: tr.backend,
				Detail:  "article parse error: " + parseErr.Error(),
			})
		} else if enriched != nil && enriched.ArticleText != "" {
			t.ArticleText = enriched.ArticleText
			if t.ArticleTitle == "" {
				t.ArticleTitle = enriched.ArticleTitle
			}
			ctx.Trace.Push(types.TraceEvent{
				Step:    step + ".article",
				Reason:  types.TraceRouteMatch,
				Adapter: "twitter",
				Backend: tr.backend,
				Detail:  fmt.Sprintf("enriched article body via TweetResultByRestId (%d chars)", len(t.ArticleText)),
			})
		}
	}

	return &thread{tweets: tweets, focal: focal, title: title, transport: tr}, nil
}

// Search implements types.SearchableAdapter for Twitter via GraphQL SearchTimeline.
// Uses stored credentials when present, otherwise (or on 401/403) ego lite.
func (a *twitterAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	result, err := searchWithFallback(ctx)
	if err != nil {
		return nil, err
	}

	result.Query = ctx.Query

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.search",
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: result.Backend,
		Detail:  fmt.Sprintf("search returned %d results for %q", len(result.Items), ctx.Query),
	})

	return result, nil
}

// TwitterReadData is the structured data returned by Read: the focal tweet,
// the conversation above it, and the first page of replies below it.
type TwitterReadData struct {
	Tweet     twitterbe.Tweet   `json:"tweet"`
	Ancestors []twitterbe.Tweet `json:"ancestors,omitempty"`
	Replies   []TwitterReply    `json:"replies"`
}

// TwitterReply is a flat view of one reply for downstream consumers.
type TwitterReply struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	AuthorName  string `json:"author_name,omitempty"`
	CreatedAt   string `json:"created_at"`
	Text        string `json:"text"`
	Likes       int    `json:"likes"`
	Replies     int    `json:"replies"`
	InReplyToID string `json:"in_reply_to_id,omitempty"`
}

func readData(th *thread) TwitterReadData {
	data := TwitterReadData{
		Tweet:     th.tweets[th.focal],
		Ancestors: th.tweets[:th.focal],
		Replies:   []TwitterReply{},
	}
	for _, t := range th.tweets[th.focal+1:] {
		data.Replies = append(data.Replies, TwitterReply{
			ID:          t.ID,
			URL:         fmt.Sprintf("https://x.com/%s/status/%s", authorOrI(t), t.ID),
			Author:      t.Author.ScreenName,
			AuthorName:  t.Author.Name,
			CreatedAt:   t.CreatedAt,
			Text:        t.Text,
			Likes:       t.Metrics["favorite_count"],
			Replies:     t.Metrics["reply_count"],
			InReplyToID: t.InReplyToID,
		})
	}
	return data
}

func authorOrI(t twitterbe.Tweet) string {
	if t.Author.ScreenName != "" {
		return t.Author.ScreenName
	}
	return "i"
}

func (a *twitterAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	th, err := a.fetchTweets(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := twitterbe.RenderMarkdown(th.tweets)

	return &types.NormalizedReadResult{
		Title:         &th.title,
		Markdown:      &markdown,
		HTML:          nil,
		Data:          readData(th),
		Backend:       th.transport.backend,
		FallbackDepth: th.transport.fallbackDepth,
	}, nil
}

func (a *twitterAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	th, err := a.fetchTweets(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := twitterbe.RenderMarkdown(th.tweets)

	return &types.NormalizedExtractResult{
		Title:         &th.title,
		Markdown:      &markdown,
		HTML:          nil,
		Data:          TwitterExtractData{Tweets: th.tweets},
		Backend:       th.transport.backend,
		FallbackDepth: th.transport.fallbackDepth,
	}, nil
}

// Post implements WritableAdapter — posts a new tweet.
func (a *twitterAdapter) Post(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	auth, err := twitterbe.LoadAuth()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.post",
			Reason:  types.TraceLoginRequired,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	result, err := twitterbe.CreateTweet(ctx.Content, auth)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.post",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.post",
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: "twitter_graphql",
		Detail:  fmt.Sprintf("tweet posted: %s", result.ResourceURL),
	})
	return result, nil
}

// Reply implements WritableAdapter — posts a reply to a tweet.
func (a *twitterAdapter) Reply(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	auth, err := twitterbe.LoadAuth()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.reply",
			Reason:  types.TraceLoginRequired,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	// Extract tweet ID from target URL.
	parsedURL, err := parseURL(ctx.TargetURL)
	if err != nil || parsedURL == nil {
		return nil, types.NewWebxError(types.ErrNoMatch, "invalid target URL for reply: "+ctx.TargetURL)
	}
	mc := types.MatchContext{URL: parsedURL}
	inReplyToID := tweetID(mc)
	if inReplyToID == "" {
		return nil, types.NewWebxError(types.ErrNoMatch, "target URL does not point to a tweet: "+ctx.TargetURL)
	}

	result, err := twitterbe.ReplyTweet(ctx.Content, inReplyToID, auth)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.reply",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.reply",
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: "twitter_graphql",
		Detail:  fmt.Sprintf("reply posted: %s", result.ResourceURL),
	})
	return result, nil
}

// React implements WritableAdapter — likes or retweets a tweet.
// Supported reactions: "like", "retweet".
func (a *twitterAdapter) React(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	auth, err := twitterbe.LoadAuth()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.react",
			Reason:  types.TraceLoginRequired,
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	parsedURL, err := parseURL(ctx.TargetURL)
	if err != nil || parsedURL == nil {
		return nil, types.NewWebxError(types.ErrNoMatch, "invalid target URL for react: "+ctx.TargetURL)
	}
	mc := types.MatchContext{URL: parsedURL}
	targetTweetID := tweetID(mc)
	if targetTweetID == "" {
		return nil, types.NewWebxError(types.ErrNoMatch, "target URL does not point to a tweet: "+ctx.TargetURL)
	}

	var result *types.NormalizedWriteResult
	switch ctx.Reaction {
	case "like":
		result, err = twitterbe.FavoriteTweet(targetTweetID, auth)
	case "retweet":
		result, err = twitterbe.RetweetTweet(targetTweetID, auth)
	default:
		return nil, types.NewWebxError(types.ErrUnsupportedKind,
			fmt.Sprintf("Twitter does not support reaction %q. Supported: like, retweet", ctx.Reaction))
	}

	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.react",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: "twitter_graphql",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.react",
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: "twitter_graphql",
		Detail:  fmt.Sprintf("reaction %q applied to tweet %s", ctx.Reaction, targetTweetID),
	})
	return result, nil
}

// parseURL is a helper to parse a URL string; returns nil on error.
func parseURL(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
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
