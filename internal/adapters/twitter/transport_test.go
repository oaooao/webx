package twitter

import (
	"encoding/json"
	"testing"

	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

// tweetJSON renders one TweetDetail tweet_results.result node.
func tweetJSON(id, screenName, text, createdAt string, likes int, replyTo string) string {
	b, _ := json.Marshal(map[string]any{
		"__typename": "Tweet",
		"rest_id":    id,
		"core": map[string]any{"user_results": map[string]any{"result": map[string]any{
			"core": map[string]any{"name": screenName + " name", "screen_name": screenName},
		}}},
		"legacy": map[string]any{
			"full_text":                 text,
			"created_at":                createdAt,
			"favorite_count":            likes,
			"in_reply_to_status_id_str": replyTo,
		},
	})
	return string(b)
}

// conversationFixture: focal tweet, one reply thread module, one promoted ad.
func conversationFixture() json.RawMessage {
	return json.RawMessage(`{"data":{"threaded_conversation_with_injections_v2":{"instructions":[{"type":"TimelineAddEntries","entries":[
	  {"entryId":"tweet-100","content":{"itemContent":{"tweet_results":{"result":` + tweetJSON("100", "author", "focal text", "Mon Sep 14 02:34:07 +0000 2026", 50, "") + `}}}},
	  {"entryId":"conversationthread-200","content":{"items":[{"item":{"itemContent":{"tweet_results":{"result":` + tweetJSON("200", "replier", "a reply", "Mon Sep 14 03:00:00 +0000 2026", 7, "100") + `}}}}]}},
	  {"entryId":"conversationthread-300","content":{"items":[{"item":{"itemContent":{"promotedMetadata":{},"tweet_results":{"result":` + tweetJSON("300", "brand", "an ad", "Mon Aug 24 23:18:12 +0000 2026", 332, "") + `}}}}]}}
	]}]}}}`)
}

func stubTransports(t *testing.T) {
	t.Helper()
	oldLoad, oldDirect, oldEgo := loadAuth, directTweetDetail, egoliteTweetDetail
	oldDirectArticle, oldEgoArticle, oldRender := directArticle, egoliteArticle, renderPage
	oldDirectSearch, oldEgoSearch := directSearch, egoliteSearch
	t.Cleanup(func() {
		loadAuth, directTweetDetail, egoliteTweetDetail = oldLoad, oldDirect, oldEgo
		directArticle, egoliteArticle, renderPage = oldDirectArticle, oldEgoArticle, oldRender
		directSearch, egoliteSearch = oldDirectSearch, oldEgoSearch
	})
}

func readCtx(t *testing.T) types.RunContext {
	return types.RunContext{
		MatchContext: types.MatchContext{URL: mustParseURL(t, "https://x.com/author/status/100")},
		Trace:        types.NewTraceBuffer(),
	}
}

func TestRead_WhenDirectRejected_FallsBackToEgoLiteWithReplies(t *testing.T) {
	stubTransports(t)
	loadAuth = func() (*twitterbe.Auth, error) { return &twitterbe.Auth{AuthToken: "stale", CT0: "stale"}, nil }
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) {
		return nil, types.NewWebxError(types.ErrLoginRequired, "Twitter auth rejected (HTTP 401)")
	}
	egoCalls := 0
	egoliteTweetDetail = func(id string) (json.RawMessage, error) {
		egoCalls++
		if id != "100" {
			t.Errorf("egolite called with id %q", id)
		}
		return conversationFixture(), nil
	}

	res, err := New().Read(readCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if egoCalls != 1 {
		t.Fatalf("egolite calls = %d, want 1", egoCalls)
	}
	if res.Backend != "egolite" || res.FallbackDepth != 1 {
		t.Errorf("backend=%q depth=%d, want egolite/1", res.Backend, res.FallbackDepth)
	}
	data, ok := res.Data.(TwitterReadData)
	if !ok {
		t.Fatalf("Data is %T, want TwitterReadData", res.Data)
	}
	if data.Tweet.ID != "100" {
		t.Errorf("focal tweet = %q", data.Tweet.ID)
	}
	if len(data.Replies) != 1 {
		t.Fatalf("replies = %d, want 1 (ad must be filtered): %+v", len(data.Replies), data.Replies)
	}
	r := data.Replies[0]
	if r.Author != "replier" || r.Text != "a reply" || r.Likes != 7 || r.CreatedAt == "" {
		t.Errorf("reply missing author/text/likes/time: %+v", r)
	}
}

func TestRead_WhenNoCredentials_UsesEgoLite(t *testing.T) {
	stubTransports(t)
	loadAuth = func() (*twitterbe.Auth, error) {
		return nil, types.NewWebxError(types.ErrLoginRequired, "no creds")
	}
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) {
		t.Fatal("direct must not be called without credentials")
		return nil, nil
	}
	egoliteTweetDetail = func(string) (json.RawMessage, error) { return conversationFixture(), nil }

	res, err := New().Read(readCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "egolite" {
		t.Errorf("backend = %q", res.Backend)
	}
}

func TestRead_WhenDirectWorks_StaysDirect(t *testing.T) {
	stubTransports(t)
	loadAuth = func() (*twitterbe.Auth, error) { return &twitterbe.Auth{AuthToken: "a", CT0: "b"}, nil }
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) { return conversationFixture(), nil }
	egoliteTweetDetail = func(string) (json.RawMessage, error) {
		t.Fatal("egolite must not be called when direct succeeds")
		return nil, nil
	}

	res, err := New().Read(readCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "twitter_graphql" || res.FallbackDepth != 0 {
		t.Errorf("backend=%q depth=%d, want twitter_graphql/0", res.Backend, res.FallbackDepth)
	}
}

func TestRead_WhenDirectFailsOtherwise_DoesNotFallBack(t *testing.T) {
	stubTransports(t)
	loadAuth = func() (*twitterbe.Auth, error) { return &twitterbe.Auth{AuthToken: "a", CT0: "b"}, nil }
	directTweetDetail = func(string, *twitterbe.Auth) (json.RawMessage, error) {
		return nil, types.NewWebxError(types.ErrRateLimited, "429")
	}
	egoliteTweetDetail = func(string) (json.RawMessage, error) {
		t.Fatal("egolite is only for 401/403")
		return nil, nil
	}
	if _, err := New().Read(readCtx(t)); err == nil {
		t.Fatal("expected error")
	}
}

func TestRead_WhenEgoLiteNotLoggedIn_ReturnsLoginRequired(t *testing.T) {
	stubTransports(t)
	loadAuth = func() (*twitterbe.Auth, error) { return nil, types.NewWebxError(types.ErrLoginRequired, "no creds") }
	egoliteTweetDetail = func(string) (json.RawMessage, error) {
		return nil, types.NewWebxError(types.ErrLoginRequired, "Not signed in to x.com in ego lite. Open ego lite, sign in to x.com, then retry.")
	}
	_, err := New().Read(readCtx(t))
	if !isLoginRequired(err) {
		t.Fatalf("expected LOGIN_REQUIRED, got %v", err)
	}
}
