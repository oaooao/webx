package reddit

import (
	"encoding/json"
	"testing"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

const threadFixture = `[
 {"kind":"Listing","data":{"children":[{"kind":"t3","data":{"title":"Rune is now open source","author":"op","score":120,"subreddit":"golang","num_comments":2,"selftext":"body"}}]}},
 {"kind":"Listing","data":{"children":[
   {"kind":"t1","data":{"id":"c1","author":"alice","body":"top comment","score":42,"created_utc":1757610016,"depth":0,
     "replies":{"kind":"Listing","data":{"children":[{"kind":"t1","data":{"id":"c2","author":"bob","body":"nested","score":3,"created_utc":1757611016,"depth":1,"replies":""}}]}}}},
   {"kind":"more","data":{"id":"m1","children":["c9"],"count":1}}
 ]}}
]`

func fixtureListings(t *testing.T) []backends.RedditListing {
	t.Helper()
	var l []backends.RedditListing
	if err := json.Unmarshal([]byte(threadFixture), &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestRead_WhenDirect403_FallsBackToEgoLiteWithCommentTree(t *testing.T) {
	oldDirect, oldEgo := directFetch, egoliteFetch
	t.Cleanup(func() { directFetch, egoliteFetch = oldDirect, oldEgo })
	directFetch = func(string, ...string) ([]backends.RedditListing, error) {
		return nil, types.NewWebxError(types.ErrLoginRequired, "Reddit returned 403")
	}
	egoliteFetch = func(string, ...string) ([]backends.RedditListing, error) { return fixtureListings(t), nil }

	ctx := types.RunContext{
		MatchContext: types.MatchContext{URL: mustParseURL(t, "https://www.reddit.com/r/golang/comments/1wdjr5u/rune/")},
		Trace:        types.NewTraceBuffer(),
	}
	res, err := New().Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "egolite" || res.FallbackDepth != 1 {
		t.Errorf("backend=%q depth=%d, want egolite/1", res.Backend, res.FallbackDepth)
	}
	data, ok := res.Data.(RedditReadData)
	if !ok {
		t.Fatalf("Data is %T", res.Data)
	}
	if len(data.Comments) != 1 {
		t.Fatalf("top-level comments = %d, want 1 (more placeholder skipped)", len(data.Comments))
	}
	c := data.Comments[0]
	if c.Author != "alice" || c.Body != "top comment" || c.Score != 42 || c.CreatedAt != "2025-09-11T17:00:16Z" {
		t.Errorf("comment fields: %+v", c)
	}
	if len(c.Replies) != 1 || c.Replies[0].Author != "bob" {
		t.Errorf("nested replies: %+v", c.Replies)
	}
}

func TestRead_WhenDirectFailsOtherwise_DoesNotFallBack(t *testing.T) {
	oldDirect, oldEgo := directFetch, egoliteFetch
	t.Cleanup(func() { directFetch, egoliteFetch = oldDirect, oldEgo })
	directFetch = func(string, ...string) ([]backends.RedditListing, error) {
		return nil, types.NewWebxError(types.ErrFetchTimeout, "timeout")
	}
	egoliteFetch = func(string, ...string) ([]backends.RedditListing, error) {
		t.Fatal("egolite is only for 401/403")
		return nil, nil
	}
	ctx := types.RunContext{
		MatchContext: types.MatchContext{URL: mustParseURL(t, "https://www.reddit.com/r/golang/comments/abc/")},
		Trace:        types.NewTraceBuffer(),
	}
	if _, err := New().Read(ctx); err == nil {
		t.Fatal("expected error")
	}
}
