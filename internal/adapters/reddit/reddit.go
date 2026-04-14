package reddit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

// commentIDRe extracts the fullname from Reddit comment URLs:
// /r/<sub>/comments/<postID>/title/<commentID>/
var commentIDRe = regexp.MustCompile(`/comments/([a-zA-Z0-9]+)/[^/]*/([a-zA-Z0-9]+)/?$`)

var redditPostIDRe = regexp.MustCompile(`/comments/([a-zA-Z0-9]+)`)

// RedditExtractData is the structured data returned by Extract.
type RedditExtractData struct {
	Post     backends.RedditPost          `json:"post"`
	Comments []backends.RedditCommentNode `json:"comments"`
}

type redditAdapter struct{}

// New returns a new Reddit adapter.
func New() types.ExtractableAdapter {
	return &redditAdapter{}
}

func (a *redditAdapter) ID() string { return "reddit" }

// Priority 90: higher than generic adapters, matched before fallback chains.
func (a *redditAdapter) Priority() int { return 90 }

func (a *redditAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindThread, types.KindComments, types.KindMetadata}
}

// Match returns true for all reddit.com subdomains (www, old, new, etc.).
func (a *redditAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	return host == "reddit.com" || strings.HasSuffix(host, ".reddit.com")
}

func extractPostID(path string) string {
	m := redditPostIDRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// fetchReddit is the shared core for Read and Extract. It fetches, parses,
// and expands the Reddit thread.
func (a *redditAdapter) fetchReddit(ctx types.RunContext, step string) (*backends.RedditResult, error) {
	listings, err := backends.FetchRedditJSON(ctx.URL.String())
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_json",
			Detail:  err.Error(),
		})
		return nil, err
	}

	result, parseErr := backends.ParseRedditListings(listings)
	if parseErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(parseErr),
			Adapter: "reddit",
			Backend: "reddit_json",
			Detail:  parseErr.Error(),
		})
		return nil, parseErr
	}

	// Attempt to expand "more" comments (no-op in v0, best-effort, never fails).
	pid := extractPostID(ctx.URL.Path)
	if pid != "" {
		backends.ExpandMoreComments(result, "t3_"+pid)
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_json",
		Detail:  fmt.Sprintf("parsed %d top-level comments", len(result.Comments)),
	})

	return result, nil
}

func (a *redditAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	result, err := a.fetchReddit(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderRedditMarkdown(result)

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}

	return &types.NormalizedReadResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		HTML:     nil,
		Backend:  "reddit_json",
	}, nil
}

// Search implements types.SearchableAdapter for Reddit via the public search.json endpoint.
func (a *redditAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	limit := ctx.Options.Limit
	if limit <= 0 {
		limit = 20
	}

	apiURL := backends.BuildRedditSearchURL(ctx.Query, limit, ctx.Options.Sort)
	result, err := backends.SearchRedditPosts(apiURL, limit, ctx.Options.Sort)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_json",
			Detail:  err.Error(),
		})
		return nil, err
	}

	result.Query = ctx.Query

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.search",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_json",
		Detail:  fmt.Sprintf("search returned %d results for %q", len(result.Items), ctx.Query),
	})

	return result, nil
}

func (a *redditAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	result, err := a.fetchReddit(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderRedditMarkdown(result)

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}

	return &types.NormalizedExtractResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		HTML:     nil,
		Data:     RedditExtractData{Post: result.Post, Comments: result.Comments},
		Backend:  "reddit_json",
	}, nil
}

// Post implements WritableAdapter — submits a new self-post to a subreddit.
// Subreddit is parsed from ctx.Platform (format: "reddit/<subreddit>") or from
// a "subreddit:<name>" prefix in ctx.Content.
// Title is the first line of ctx.Content; body is the remainder.
func (a *redditAdapter) Post(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	token, err := backends.LoadRedditAccessToken()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.post",
			Reason:  types.TraceLoginRequired,
			Adapter: "reddit",
			Backend: "reddit_oauth",
			Detail:  err.Error(),
		})
		return nil, err
	}

	subreddit, title, body := parseRedditPostArgs(ctx)
	if subreddit == "" {
		return nil, types.NewWebxError(types.ErrNoMatch,
			"Reddit post requires a subreddit. Use platform 'reddit/<subreddit>' or prefix content with 'subreddit:<name>'")
	}
	if title == "" {
		return nil, types.NewWebxError(types.ErrNoMatch, "Reddit post requires a non-empty title (first line of content)")
	}

	apiURL := backends.RedditSubmitURL()
	result, err := backends.PostReddit(apiURL, subreddit, title, body, token)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.post",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_oauth",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.post",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_oauth",
		Detail:  fmt.Sprintf("posted to r/%s: %s", subreddit, result.ResourceURL),
	})
	return result, nil
}

// Reply implements WritableAdapter — posts a comment on a Reddit post or comment.
// ctx.TargetURL must point to a Reddit post or comment URL.
func (a *redditAdapter) Reply(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	token, err := backends.LoadRedditAccessToken()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.reply",
			Reason:  types.TraceLoginRequired,
			Adapter: "reddit",
			Backend: "reddit_oauth",
			Detail:  err.Error(),
		})
		return nil, err
	}

	thingID := extractRedditThingID(ctx.TargetURL)
	if thingID == "" {
		return nil, types.NewWebxError(types.ErrNoMatch,
			"could not extract Reddit post/comment ID from URL: "+ctx.TargetURL)
	}

	apiURL := backends.RedditCommentURL()
	result, err := backends.CommentReddit(apiURL, thingID, ctx.Content, token)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.reply",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_oauth",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.reply",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_oauth",
		Detail:  fmt.Sprintf("commented on %s", thingID),
	})
	return result, nil
}

// React implements WritableAdapter — votes on a Reddit post or comment.
// Supported reactions: "upvote", "downvote", "unvote".
func (a *redditAdapter) React(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	token, err := backends.LoadRedditAccessToken()
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.react",
			Reason:  types.TraceLoginRequired,
			Adapter: "reddit",
			Backend: "reddit_oauth",
			Detail:  err.Error(),
		})
		return nil, err
	}

	thingID := extractRedditThingID(ctx.TargetURL)
	if thingID == "" {
		return nil, types.NewWebxError(types.ErrNoMatch,
			"could not extract Reddit post/comment ID from URL: "+ctx.TargetURL)
	}

	var dir int
	switch ctx.Reaction {
	case "upvote":
		dir = 1
	case "downvote":
		dir = -1
	case "unvote", "":
		dir = 0
	default:
		return nil, types.NewWebxError(types.ErrUnsupportedKind,
			fmt.Sprintf("Reddit does not support reaction %q. Supported: upvote, downvote, unvote", ctx.Reaction))
	}

	apiURL := backends.RedditVoteURL()
	result, err := backends.VoteReddit(apiURL, thingID, dir, token)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.react",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_oauth",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.react",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_oauth",
		Detail:  fmt.Sprintf("voted %d on %s", dir, thingID),
	})
	return result, nil
}

// parseRedditPostArgs extracts subreddit, title, and body from WriteContext.
// Platform format: "reddit/<subreddit>" sets the subreddit.
// Content format: first line is the title; remaining lines are the body.
func parseRedditPostArgs(ctx types.WriteContext) (subreddit, title, body string) {
	if parts := strings.SplitN(ctx.Platform, "/", 2); len(parts) == 2 && parts[0] == "reddit" {
		subreddit = parts[1]
	}
	lines := strings.SplitN(ctx.Content, "\n", 2)
	title = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
	}
	return
}

// extractRedditThingID derives the Reddit fullname (e.g. "t3_abc123" or "t1_def456")
// from a Reddit URL. For comment URLs it returns the comment fullname; for post
// URLs it returns the post fullname.
func extractRedditThingID(rawURL string) string {
	// Try comment URL first: /r/<sub>/comments/<postID>/title/<commentID>/
	if m := commentIDRe.FindStringSubmatch(rawURL); len(m) == 3 {
		return "t1_" + m[2]
	}
	// Fall back to post URL: /r/<sub>/comments/<postID>/...
	if m := redditPostIDRe.FindStringSubmatch(rawURL); len(m) == 2 {
		return "t3_" + m[1]
	}
	return ""
}

