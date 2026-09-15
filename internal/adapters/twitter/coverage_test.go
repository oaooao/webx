package twitter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oaooao/webx/internal/backends/egolite"
	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

func staleAuth() (*twitterbe.Auth, error) {
	return &twitterbe.Auth{AuthToken: "stale", CT0: "stale"}, nil
}

// selfThreadFixture: focal tweet, the author's 2-tweet continuation module,
// then a commenter thread where the author answers the commenter.
func selfThreadFixture() json.RawMessage {
	return json.RawMessage(`{"data":{"threaded_conversation_with_injections_v2":{"instructions":[{"type":"TimelineAddEntries","entries":[
	  {"entryId":"tweet-100","content":{"itemContent":{"tweet_results":{"result":` + tweetJSON("100", "author", "intro 🧵", "Thu Sep 10 20:30:45 +0000 2026", 50, "") + `}}}},
	  {"entryId":"conversationthread-101","content":{"items":[
	    {"item":{"itemContent":{"tweet_results":{"result":` + tweetJSON("101", "author", "part two", "Thu Sep 10 20:30:46 +0000 2026", 3, "100") + `}}}},
	    {"item":{"itemContent":{"tweet_results":{"result":` + tweetJSON("102", "author", "part three", "Thu Sep 10 20:30:47 +0000 2026", 2, "101") + `}}}}
	  ]}},
	  {"entryId":"conversationthread-200","content":{"items":[
	    {"item":{"itemContent":{"tweet_results":{"result":` + tweetJSON("200", "raul", "question?", "Fri Sep 11 01:00:00 +0000 2026", 1, "100") + `}}}},
	    {"item":{"itemContent":{"tweet_results":{"result":` + tweetJSON("201", "author", "answer", "Fri Sep 11 02:00:00 +0000 2026", 1, "200") + `}}}}
	  ]}}
	]}]}}}`)
}

func emptyTimeline() json.RawMessage {
	return json.RawMessage(`{"data":{"threaded_conversation_with_injections_v2":{"instructions":[]}}}`)
}

func TestRead_SelfThreadGoesToDataThreadAndMarkdownBody(t *testing.T) {
	stubTransports(t)
	loadAuth = staleAuth
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) { return selfThreadFixture(), nil }

	res, err := New().Read(readCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	data := res.Data.(TwitterReadData)
	if len(data.Thread) != 2 || data.Thread[0].Text != "part two" || data.Thread[1].Text != "part three" {
		t.Fatalf("thread = %+v, want part two, part three", data.Thread)
	}
	if len(data.Replies) != 2 || data.Replies[0].Author != "raul" || data.Replies[1].Author != "author" {
		t.Fatalf("replies = %+v, want raul then the author's answer", data.Replies)
	}
	md := *res.Markdown
	if strings.Count(md, "Reply by @author") != 1 || !strings.Contains(md, "intro 🧵\n\npart two\n\npart three") {
		t.Errorf("thread must render as body paragraphs:\n%s", md)
	}
}

func TestRead_WhenDirectAnswersEmpty_FallsBackToEgoLite(t *testing.T) {
	stubTransports(t)
	loadAuth = staleAuth
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) { return emptyTimeline(), nil }
	egoliteTweetDetail = func(string) (json.RawMessage, error) { return conversationFixture(), nil }

	ctx := readCtx(t)
	res, err := New().Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != egolite.BackendID || res.FallbackDepth != 1 {
		t.Errorf("backend=%q depth=%d, want egolite/1", res.Backend, res.FallbackDepth)
	}
	found := false
	for _, ev := range ctx.Trace.All() {
		if ev.Reason == types.TraceEmptyContent && ev.Backend == directBackend {
			found = true
		}
	}
	if !found {
		t.Error("trace should record the empty direct answer")
	}
}

func TestRead_WhenDirectEmptyAndEgoLiteDown_ExplainsBoth(t *testing.T) {
	stubTransports(t)
	loadAuth = staleAuth
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) { return emptyTimeline(), nil }
	egoliteTweetDetail = func(string) (json.RawMessage, error) {
		return nil, types.NewWebxError(types.ErrLoginRequired, "ego lite is not running")
	}
	_, err := New().Read(readCtx(t))
	if !isLoginRequired(err) || !strings.Contains(err.Error(), "may be stale") {
		t.Fatalf("want LOGIN_REQUIRED mentioning stale credentials, got %v", err)
	}
}

func searchCtx() types.SearchContext {
	return types.SearchContext{Query: "claude code", Platform: "twitter", Trace: types.NewTraceBuffer()}
}

func TestSearch_WhenDirectReturnsZero_FallsBackToEgoLite(t *testing.T) {
	stubTransports(t)
	directSearch = func(types.SearchContext) (*types.NormalizedSearchResult, error) {
		return &types.NormalizedSearchResult{Backend: directBackend}, nil
	}
	egoliteSearch = func(types.SearchContext) (*types.NormalizedSearchResult, error) {
		return &types.NormalizedSearchResult{Backend: egolite.BackendID, Items: []types.SearchResultItem{{Title: "hit"}}}, nil
	}
	res, err := New().(types.SearchableAdapter).Search(searchCtx())
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != egolite.BackendID || len(res.Items) != 1 {
		t.Errorf("backend=%q items=%d, want egolite/1", res.Backend, len(res.Items))
	}
}

func TestSearch_WhenDirectHasResults_StaysDirect(t *testing.T) {
	stubTransports(t)
	directSearch = func(types.SearchContext) (*types.NormalizedSearchResult, error) {
		return &types.NormalizedSearchResult{Backend: directBackend, Items: []types.SearchResultItem{{Title: "hit"}}}, nil
	}
	egoliteSearch = func(types.SearchContext) (*types.NormalizedSearchResult, error) {
		t.Fatal("egolite must not run when direct has results")
		return nil, nil
	}
	if _, err := New().(types.SearchableAdapter).Search(searchCtx()); err != nil {
		t.Fatal(err)
	}
}

func TestParseArticleRef(t *testing.T) {
	cases := []struct {
		path, articleID, tweetID string
		ok                       bool
	}{
		{"/i/article/2099801823892922368", "2099801823892922368", "", true},
		{"/ojisan_aibaka/article/2099803476880658555", "", "2099803476880658555", true},
		{"/ojisan_aibaka/status/2099803476880658555", "", "", false},
		{"/i/articles", "", "", false},
	}
	for _, c := range cases {
		ref, ok := parseArticleRef(c.path)
		if ok != c.ok || ref.articleID != c.articleID || ref.tweetID != c.tweetID {
			t.Errorf("parseArticleRef(%q) = %+v,%v", c.path, ref, ok)
		}
	}
}

func TestKindFor(t *testing.T) {
	a := New().(types.URLKindAdapter)
	if k := a.KindFor(types.MatchContext{URL: mustParseURL(t, "https://x.com/i/article/1")}); k != types.KindArticle {
		t.Errorf("article link kind = %q", k)
	}
	if k := a.KindFor(types.MatchContext{URL: mustParseURL(t, "https://x.com/a/status/1")}); k != types.KindThread {
		t.Errorf("status link kind = %q", k)
	}
}

func articleResponse(tweetID string) json.RawMessage {
	return json.RawMessage(`{"data":{"tweetResult":{"result":{"__typename":"Tweet","rest_id":"` + tweetID + `",
	  "core":{"user_results":{"result":{"core":{"name":"Oji","screen_name":"ojisan"}}}},
	  "legacy":{"full_text":"https://t.co/x","created_at":"Tue Sep 15 10:12:16 +0000 2026"},
	  "article":{"article_results":{"result":{"rest_id":"777","title":"Skills attendance",
	    "content_state":{"blocks":[{"type":"header-two","text":"Section","entityRanges":[]},{"type":"unstyled","text":"Body text","entityRanges":[]}],"entityMap":{}}}}}}}}}`)
}

func articleCtx(t *testing.T, raw string) types.RunContext {
	return types.RunContext{
		MatchContext: types.MatchContext{URL: mustParseURL(t, raw)},
		Trace:        types.NewTraceBuffer(),
	}
}

func TestRead_ArticleLinkResolvesHostTweetAndReadsFullArticle(t *testing.T) {
	stubTransports(t)
	loadAuth = func() (*twitterbe.Auth, error) { return nil, types.NewWebxError(types.ErrLoginRequired, "no creds") }
	renderPage = func(job egolite.PageJob) (*egolite.Page, error) {
		if job.URL != "https://x.com/i/article/777" || job.WaitURLPattern == "" {
			t.Errorf("unexpected page job %+v", job)
		}
		return &egolite.Page{URL: "https://x.com/ojisan/article/555"}, nil
	}
	egoliteArticle = func(id string) (json.RawMessage, error) {
		if id != "555" {
			t.Errorf("article fetched for tweet %q, want 555", id)
		}
		return articleResponse(id), nil
	}

	res, err := New().Read(articleCtx(t, "https://x.com/i/article/777"))
	if err != nil {
		t.Fatal(err)
	}
	data := res.Data.(TwitterArticleData)
	if data.ArticleID != "777" || data.TweetID != "555" || data.Author.ScreenName != "ojisan" || data.CreatedAt == "" || data.Title != "Skills attendance" {
		t.Errorf("data = %+v", data)
	}
	if *res.Title != "Skills attendance" || !strings.Contains(*res.Markdown, "## Section\n\nBody text") {
		t.Errorf("title=%q markdown=%q", *res.Title, *res.Markdown)
	}
	if res.Backend != egolite.BackendID {
		t.Errorf("backend = %q", res.Backend)
	}
}

func TestRead_CanonicalArticleLinkSkipsResolve(t *testing.T) {
	stubTransports(t)
	loadAuth = staleAuth
	renderPage = func(egolite.PageJob) (*egolite.Page, error) {
		t.Fatal("canonical link already carries the tweet id")
		return nil, nil
	}
	directArticle = func(id string, _ *twitterbe.Auth) (json.RawMessage, error) { return articleResponse(id), nil }

	res, err := New().Read(articleCtx(t, "https://x.com/ojisan/article/555"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != directBackend {
		t.Errorf("backend = %q", res.Backend)
	}
}

func TestRead_ArticleLinkWithoutRedirect_IsContentEmpty(t *testing.T) {
	stubTransports(t)
	renderPage = func(egolite.PageJob) (*egolite.Page, error) {
		return &egolite.Page{URL: "https://x.com/i/article/777"}, nil
	}
	_, err := New().Read(articleCtx(t, "https://x.com/i/article/777"))
	if code := codeOf(err); code != types.ErrContentEmpty {
		t.Fatalf("code = %q (%v), want CONTENT_EMPTY", code, err)
	}
}

func codeOf(err error) types.ErrorCode {
	if wxErr, ok := err.(*types.WebxError); ok {
		return wxErr.Code
	}
	return ""
}
