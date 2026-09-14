package reddit

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

const directBackend = "reddit_json"

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

// Seams for tests.
var (
	directFetch   = backends.FetchRedditJSON
	egoliteFetch  = backends.FetchRedditJSONViaEgoLite
	directSearch  = backends.SearchRedditPosts
	egoliteSearch = backends.SearchRedditPostsViaEgoLite
)

func isLoginRequired(err error) bool {
	var wxErr *types.WebxError
	return errors.As(err, &wxErr) && wxErr.Code == types.ErrLoginRequired
}

// fetched is a parsed Reddit thread plus how it was fetched.
type fetched struct {
	*backends.RedditResult
	backend       string
	fallbackDepth int
}

// fetchReddit is the shared core for Read and Extract. It fetches the thread
// JSON directly, falls back to ego lite on 401/403, then parses the post and
// comment tree.
func (a *redditAdapter) fetchReddit(ctx types.RunContext, step string) (*fetched, error) {
	backend, depth := directBackend, 0
	listings, err := directFetch(ctx.URL.String())
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: directBackend,
			Detail:  err.Error(),
		})
		if !isLoginRequired(err) {
			return nil, err
		}
		backend, depth = egolite.BackendID, 1
		listings, err = egoliteFetch(ctx.URL.String())
		if err != nil {
			ctx.Trace.Push(types.TraceEvent{
				Step:    step + ".fallback",
				Reason:  types.TraceReasonFromError(err),
				Adapter: "reddit",
				Backend: backend,
				Detail:  err.Error(),
			})
			return nil, err
		}
	}

	result, parseErr := backends.ParseRedditListings(listings)
	if parseErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(parseErr),
			Adapter: "reddit",
			Backend: backend,
			Detail:  parseErr.Error(),
		})
		return nil, parseErr
	}

	// Expand "more" placeholders over the direct client. Best-effort and
	// skipped under egolite: the direct client is the one being rejected.
	pid := extractPostID(ctx.URL.Path)
	if pid != "" && backend == directBackend {
		backends.ExpandMoreComments(result, "t3_"+pid)
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: backend,
		Detail:  fmt.Sprintf("parsed %d top-level comments", len(result.Comments)),
	})

	return &fetched{RedditResult: result, backend: backend, fallbackDepth: depth}, nil
}

func (a *redditAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	result, err := a.fetchReddit(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderRedditMarkdown(result.RedditResult)

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}

	return &types.NormalizedReadResult{
		Title:         types.StringPtr(title),
		Markdown:      &markdown,
		HTML:          nil,
		Data:          RedditReadData{Post: result.Post, Comments: readComments(result.Comments)},
		Backend:       result.backend,
		FallbackDepth: result.fallbackDepth,
	}, nil
}

// RedditReadData is the structured data returned by Read: the post and its
// comment tree as fetched (Reddit's first page, nested replies included).
type RedditReadData struct {
	Post     backends.RedditPost `json:"post"`
	Comments []RedditComment     `json:"comments"`
}

// RedditComment is a clean view of one comment node, without Reddit's raw
// replies listing.
type RedditComment struct {
	ID         string          `json:"id"`
	Author     string          `json:"author"`
	CreatedAt  string          `json:"created_at"`
	CreatedUTC float64         `json:"created_utc"`
	Body       string          `json:"body"`
	Score      int             `json:"score"`
	Depth      int             `json:"depth"`
	Replies    []RedditComment `json:"replies"`
}

func readComments(nodes []backends.RedditCommentNode) []RedditComment {
	out := []RedditComment{}
	for _, n := range nodes {
		c := n.Comment
		if c.ID == "" || (c.Body == "" && len(c.Children) > 0) {
			continue // "more" placeholder, not a comment
		}
		out = append(out, RedditComment{
			ID:         c.ID,
			Author:     c.Author,
			CreatedAt:  time.Unix(int64(c.CreatedUTC), 0).UTC().Format(time.RFC3339),
			CreatedUTC: c.CreatedUTC,
			Body:       c.Body,
			Score:      c.Score,
			Depth:      c.Depth,
			Replies:    readComments(n.Replies),
		})
	}
	return out
}

// Search implements types.SearchableAdapter for Reddit via the public search.json endpoint.
func (a *redditAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	limit := ctx.Options.Limit
	if limit <= 0 {
		limit = 20
	}

	apiURL := backends.BuildRedditSearchURL(ctx.Query, limit, ctx.Options.Sort)
	result, err := directSearch(apiURL, limit, ctx.Options.Sort)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: directBackend,
			Detail:  err.Error(),
		})
		if !isLoginRequired(err) {
			return nil, err
		}
		result, err = egoliteSearch(apiURL)
		if err != nil {
			ctx.Trace.Push(types.TraceEvent{
				Step:    "adapter.search.fallback",
				Reason:  types.TraceReasonFromError(err),
				Adapter: "reddit",
				Backend: egolite.BackendID,
				Detail:  err.Error(),
			})
			return nil, err
		}
	}

	result.Query = ctx.Query

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.search",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: result.Backend,
		Detail:  fmt.Sprintf("search returned %d results for %q", len(result.Items), ctx.Query),
	})

	return result, nil
}

func (a *redditAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	result, err := a.fetchReddit(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderRedditMarkdown(result.RedditResult)

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}

	return &types.NormalizedExtractResult{
		Title:         types.StringPtr(title),
		Markdown:      &markdown,
		HTML:          nil,
		Data:          RedditExtractData{Post: result.Post, Comments: result.Comments},
		Backend:       result.backend,
		FallbackDepth: result.fallbackDepth,
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

