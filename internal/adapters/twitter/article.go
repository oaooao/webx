package twitter

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/oaooao/webx/internal/backends/egolite"
	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

// X Article links come in two shapes:
//
//	/i/article/<articleID>          shared link; articleID is the article entity id
//	/<screen_name>/article/<tweetID> canonical link; tweetID is the host tweet
//
// The API only reads articles through the host tweet (TweetResultByRestId),
// and nothing maps an article id to its tweet without the web client: x.com
// resolves /i/article/<id> client-side by redirecting to the canonical link.
var (
	articleEntityPathRe = regexp.MustCompile(`^/i/article/(\d+)/?$`)
	articleTweetPathRe  = regexp.MustCompile(`^/([A-Za-z0-9_]{1,15})/article/(\d+)/?$`)
	// articleRedirectPattern is matched in the browser (JavaScript RegExp).
	articleRedirectPattern = `^https://(?:www\.)?(?:x|twitter)\.com/(?!i/)[A-Za-z0-9_]{1,15}/article/\d+`
)

// articleRef identifies an X Article from its URL. Exactly one id is set.
type articleRef struct {
	articleID string
	tweetID   string
}

func parseArticleRef(path string) (articleRef, bool) {
	if m := articleEntityPathRe.FindStringSubmatch(path); m != nil {
		return articleRef{articleID: m[1]}, true
	}
	if m := articleTweetPathRe.FindStringSubmatch(path); m != nil && m[1] != "i" {
		return articleRef{tweetID: m[2]}, true
	}
	return articleRef{}, false
}

// TwitterArticleData is the structured data returned for an X Article link.
// The article body is in content.markdown.
type TwitterArticleData struct {
	ArticleID string           `json:"article_id,omitempty"`
	TweetID   string           `json:"tweet_id"`
	URL       string           `json:"url"`
	Title     string           `json:"title"`
	Author    twitterbe.Author `json:"author"`
	CreatedAt string           `json:"created_at"`
}

type article struct {
	title     string
	markdown  string
	data      TwitterArticleData
	transport transport
}

// resolveArticleTweetID follows x.com's client-side redirect from
// /i/article/<articleID> to /<screen_name>/article/<tweetID> in ego lite.
func resolveArticleTweetID(ctx types.RunContext, step, articleID string) (string, error) {
	link := "https://x.com/i/article/" + articleID
	page, err := renderPage(egolite.PageJob{
		Site:           "x.com",
		URL:            link,
		WaitURLPattern: articleRedirectPattern,
		WaitMs:         12000,
	})
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step + ".resolve",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: egolite.BackendID,
			Detail:  "resolving article link in ego lite failed: " + err.Error(),
		})
		return "", err
	}
	u := page.URL
	if parsed, perr := parseURL(u); perr == nil {
		if ref, ok := parseArticleRef(parsed.Path); ok && ref.tweetID != "" {
			ctx.Trace.Push(types.TraceEvent{
				Step:    step + ".resolve",
				Reason:  types.TraceRouteMatch,
				Adapter: "twitter",
				Backend: egolite.BackendID,
				Detail:  fmt.Sprintf("article %s redirects to %s", articleID, u),
			})
			return ref.tweetID, nil
		}
	}
	ctx.Trace.Push(types.TraceEvent{
		Step:    step + ".resolve",
		Reason:  types.TraceEmptyContent,
		Adapter: "twitter",
		Backend: egolite.BackendID,
		Detail:  "x.com did not redirect the article link to its post; page ended on " + u,
	})
	return "", types.NewWebxError(types.ErrContentEmpty, fmt.Sprintf(
		"x.com did not resolve %s to a post (page ended on %s): the article may be deleted, private or unpublished. Open it in ego lite to check.",
		link, u))
}

// fetchArticle reads an X Article link: resolve the host tweet when needed,
// then fetch the full article via TweetResultByRestId over the transport chain.
func (a *twitterAdapter) fetchArticle(ctx types.RunContext, step string, ref articleRef) (*article, error) {
	tweetID := ref.tweetID
	if tweetID == "" {
		id, err := resolveArticleTweetID(ctx, step, ref.articleID)
		if err != nil {
			return nil, err
		}
		tweetID = id
	}

	tr, tweet, err := fetchWithFallback(ctx.Trace, step, "article body",
		func(auth *twitterbe.Auth) (json.RawMessage, error) { return directArticle(tweetID, auth) },
		func() (json.RawMessage, error) { return egoliteArticle(tweetID) },
		func(raw json.RawMessage) (*twitterbe.Tweet, error) {
			t, err := twitterbe.ParseTweetResultByRestIdResponse(raw)
			if err != nil {
				return nil, types.NewWebxError(types.ErrBackendFailed, "failed to parse TweetResultByRestId response: "+err.Error())
			}
			return t, nil
		},
		func(t *twitterbe.Tweet) bool { return t == nil || t.ArticleText == "" },
	)
	if err != nil {
		return nil, err
	}
	if tweet == nil || tweet.ArticleText == "" {
		detail := "post " + tweetID + " not found"
		if tweet != nil {
			detail = "post " + tweetID + " has no article body"
		}
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceEmptyContent,
			Adapter: "twitter",
			Backend: tr.backend,
			Detail:  detail,
		})
		return nil, types.NewWebxError(types.ErrContentEmpty,
			"X returned no article content ("+detail+"): the article may be deleted or private")
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "twitter",
		Backend: tr.backend,
		Detail:  fmt.Sprintf("read X Article %q via TweetResultByRestId (%d chars)", tweet.ArticleTitle, len(tweet.ArticleText)),
	})

	title := tweet.ArticleTitle
	if title == "" {
		title = fmt.Sprintf("X Article by @%s", authorOrI(*tweet))
	}
	return &article{
		title:    title,
		markdown: twitterbe.RenderArticleMarkdown(*tweet),
		data: TwitterArticleData{
			ArticleID: ref.articleID,
			TweetID:   tweet.ID,
			URL:       fmt.Sprintf("https://x.com/%s/article/%s", authorOrI(*tweet), tweet.ID),
			Title:     tweet.ArticleTitle,
			Author:    tweet.Author,
			CreatedAt: tweet.CreatedAt,
		},
		transport: tr,
	}, nil
}
