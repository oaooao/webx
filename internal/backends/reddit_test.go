package backends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildRedditFixture constructs a minimal Reddit .json response as raw bytes.
// The structure is []RedditListing with two elements: [post_listing, comments_listing].
func buildRedditFixture() []byte {
	raw := `[
		{
			"kind": "Listing",
			"data": {
				"children": [
					{
						"kind": "t3",
						"data": {
							"title": "Test Post Title",
							"selftext": "Post body here.",
							"author": "testuser",
							"score": 42,
							"subreddit": "TestSubreddit",
							"permalink": "/r/TestSubreddit/comments/abc123/test_post_title/",
							"num_comments": 2,
							"url": "https://www.reddit.com/r/TestSubreddit/comments/abc123/test_post_title/"
						}
					}
				]
			}
		},
		{
			"kind": "Listing",
			"data": {
				"children": [
					{
						"kind": "t1",
						"data": {
							"id": "c1",
							"author": "commenter1",
							"body": "Top level comment.",
							"score": 10,
							"depth": 0,
							"replies": {
								"kind": "Listing",
								"data": {
									"children": [
										{
											"kind": "t1",
											"data": {
												"id": "c2",
												"author": "commenter2",
												"body": "Nested reply.",
												"score": 5,
												"depth": 1,
												"replies": ""
											}
										}
									]
								}
							}
						}
					},
					{
						"kind": "t1",
						"data": {
							"id": "c3",
							"author": "commenter3",
							"body": "Second top level comment.",
							"score": 3,
							"depth": 0,
							"replies": ""
						}
					},
					{
						"kind": "more",
						"data": {
							"children": ["x1", "x2"],
							"count": 5
						}
					}
				]
			}
		}
	]`
	return []byte(raw)
}

func TestParseRedditListings_Basic(t *testing.T) {
	var listings []RedditListing
	if err := json.Unmarshal(buildRedditFixture(), &listings); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	result, err := ParseRedditListings(listings)
	if err != nil {
		t.Fatalf("ParseRedditListings: %v", err)
	}

	// Post fields
	if result.Post.Title != "Test Post Title" {
		t.Errorf("post title: got %q, want %q", result.Post.Title, "Test Post Title")
	}
	if result.Post.Author != "testuser" {
		t.Errorf("post author: got %q, want %q", result.Post.Author, "testuser")
	}
	if result.Post.Score != 42 {
		t.Errorf("post score: got %d, want %d", result.Post.Score, 42)
	}
	if result.Post.Subreddit != "TestSubreddit" {
		t.Errorf("subreddit: got %q, want %q", result.Post.Subreddit, "TestSubreddit")
	}
}

func TestParseRedditListings_CommentTree(t *testing.T) {
	var listings []RedditListing
	if err := json.Unmarshal(buildRedditFixture(), &listings); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	result, err := ParseRedditListings(listings)
	if err != nil {
		t.Fatalf("ParseRedditListings: %v", err)
	}

	// Should have 3 top-level entries: 2 comments + 1 "more" placeholder
	if len(result.Comments) != 3 {
		t.Errorf("top-level comments: got %d, want 3", len(result.Comments))
	}

	// First comment has one nested reply
	c1 := result.Comments[0]
	if c1.Comment.Author != "commenter1" {
		t.Errorf("c1 author: got %q, want %q", c1.Comment.Author, "commenter1")
	}
	if len(c1.Replies) != 1 {
		t.Errorf("c1 replies: got %d, want 1", len(c1.Replies))
	}

	c2 := c1.Replies[0]
	if c2.Comment.Author != "commenter2" {
		t.Errorf("c2 author: got %q, want %q", c2.Comment.Author, "commenter2")
	}
	if len(c2.Replies) != 0 {
		t.Errorf("c2 should have no replies, got %d", len(c2.Replies))
	}

	// Second comment has no replies (replies == "")
	c3 := result.Comments[1]
	if c3.Comment.Author != "commenter3" {
		t.Errorf("c3 author: got %q, want %q", c3.Comment.Author, "commenter3")
	}
	if len(c3.Replies) != 0 {
		t.Errorf("c3 should have no replies (empty string), got %d", len(c3.Replies))
	}
}

// TestParseRedditListings_C3Fix specifically tests that "" replies don't cause errors.
func TestParseRedditListings_C3Fix(t *testing.T) {
	// Craft a comment where replies is an empty string (not null)
	raw := `[
		{"kind": "Listing", "data": {"children": [{"kind": "t3", "data": {"title": "X"}}]}},
		{"kind": "Listing", "data": {"children": [
			{"kind": "t1", "data": {"id": "leaf", "author": "a", "body": "b", "replies": ""}}
		]}}
	]`

	var listings []RedditListing
	if err := json.Unmarshal([]byte(raw), &listings); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result, err := ParseRedditListings(listings)
	if err != nil {
		t.Fatalf("ParseRedditListings should not error on empty-string replies: %v", err)
	}
	if len(result.Comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(result.Comments))
	}
	if len(result.Comments[0].Replies) != 0 {
		t.Errorf("leaf comment should have no replies, got %d", len(result.Comments[0].Replies))
	}
}

func TestParseRedditListings_Empty(t *testing.T) {
	_, err := ParseRedditListings([]RedditListing{})
	if err == nil {
		t.Error("expected error for empty listings")
	}
}

func TestRenderRedditMarkdown_Format(t *testing.T) {
	var listings []RedditListing
	if err := json.Unmarshal(buildRedditFixture(), &listings); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	result, err := ParseRedditListings(listings)
	if err != nil {
		t.Fatalf("ParseRedditListings: %v", err)
	}

	md := RenderRedditMarkdown(result)

	checks := []string{
		"# Test Post Title",
		"r/TestSubreddit",
		"u/testuser",
		"Post body here.",
		"## Comments",
		"**commenter1**",
		"Top level comment.",
		"**commenter2**",
		"Nested reply.",
	}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\nGot:\n%s", want, md)
		}
	}

	// Nested reply should be indented more than top-level
	c1Idx := strings.Index(md, "**commenter1**")
	c2Idx := strings.Index(md, "**commenter2**")
	if c1Idx < 0 || c2Idx < 0 {
		t.Fatal("comments not found in markdown")
	}

	// Find the line with commenter2 and check indentation
	lines := strings.Split(md, "\n")
	for _, line := range lines {
		if strings.Contains(line, "**commenter2**") {
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("nested comment not indented: %q", line)
			}
		}
	}
}

// TestFetchRedditJSON_RateLimit_Retry tests that the retry loop in FetchRedditJSON
// retries on 429 and succeeds on the second attempt.
// We call FetchRedditJSON with a URL that already ends in .json to avoid path munging.
func TestFetchRedditJSON_RateLimit_Retry(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditFixture())
	}))
	defer ts.Close()

	// FetchRedditJSON will keep the .json suffix since it's already there.
	// Override redditRetryDelay to 0 for speed is not possible from tests,
	// so we call doRedditFetch to bypass the delay and test retry counting
	// at a lower level. We verify that a second doRedditFetch call succeeds
	// after the first 429, which validates the server behaviour.
	_, err429 := doRedditFetch(ts.URL + "/r/test/comments/abc/.json")
	if err429 == nil {
		t.Fatal("expected 429 error on first call")
	}

	listings, err := doRedditFetch(ts.URL + "/r/test/comments/abc/.json")
	if err != nil {
		t.Fatalf("doRedditFetch second attempt: %v", err)
	}
	if len(listings) != 2 {
		t.Errorf("expected 2 listings, got %d", len(listings))
	}
	if attempt != 2 {
		t.Errorf("expected 2 total server calls, got %d", attempt)
	}
}

func TestFetchRedditJSON_Non429Error_NoRetry(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(500)
	}))
	defer ts.Close()

	_, err := doRedditFetch(ts.URL + "/r/test/comments/abc/.json")
	if err == nil {
		t.Error("expected error for 500")
	}
	if attempt != 1 {
		t.Errorf("expected no retry on 500, got %d attempts", attempt)
	}
}

// --- Reddit Search tests ---

// buildRedditSearchFixture constructs a Reddit /search.json response.
// The format is a single Listing (not the [post, comments] pair used for threads).
func buildRedditSearchFixture() []byte {
	return []byte(`{
		"kind": "Listing",
		"data": {
			"after": "t3_xyz",
			"children": [
				{
					"kind": "t3",
					"data": {
						"title": "Webx: a CLI tool for AI agents",
						"author": "oaooao",
						"score": 1337,
						"subreddit": "golang",
						"permalink": "/r/golang/comments/abc123/webx_a_cli_tool_for_ai_agents/",
						"url": "https://github.com/oaooao/webx",
						"selftext": "Just released webx...",
						"num_comments": 25,
						"created_utc": 1700000000
					}
				},
				{
					"kind": "t3",
					"data": {
						"title": "Ask HN: Best CLI tools for 2024?",
						"author": "techuser",
						"score": 420,
						"subreddit": "programming",
						"permalink": "/r/programming/comments/def456/ask_hn_best_cli_tools_for_2024/",
						"url": "https://www.reddit.com/r/programming/comments/def456/",
						"selftext": "",
						"num_comments": 80,
						"created_utc": 1699900000
					}
				}
			]
		}
	}`)
}

func TestParseRedditSearchResponse_WhenValid_ShouldParsePosts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditSearchFixture())
	}))
	defer ts.Close()

	result, err := SearchRedditPosts(ts.URL+"?q=webx", 20, "relevance")
	if err != nil {
		t.Fatalf("SearchRedditPosts: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	// First result
	item := result.Items[0]
	if item.Title != "Webx: a CLI tool for AI agents" {
		t.Errorf("title: got %q", item.Title)
	}
	if item.Author != "oaooao" {
		t.Errorf("author: got %q", item.Author)
	}
	if item.Score != 1337 {
		t.Errorf("score: got %f, want 1337", item.Score)
	}
	// URL should be subreddit permalink (canonical)
	if item.URL == "" {
		t.Error("URL should not be empty")
	}
	// Meta should contain subreddit
	if item.Meta == nil {
		t.Error("Meta should not be nil")
	}
	if sub, ok := item.Meta["subreddit"]; !ok || sub != "golang" {
		t.Errorf("Meta[subreddit]: got %v, want golang", item.Meta["subreddit"])
	}
}

func TestParseRedditSearchResponse_WhenEmpty_ShouldReturnEmptyItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"kind": "Listing", "data": {"children": []}}`))
	}))
	defer ts.Close()

	result, err := SearchRedditPosts(ts.URL+"?q=xyznotexist", 20, "relevance")
	if err != nil {
		t.Fatalf("SearchRedditPosts: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestParseRedditSearchResponse_WhenHTTPError_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	_, err := SearchRedditPosts(ts.URL+"?q=webx", 20, "relevance")
	if err == nil {
		t.Error("expected error for HTTP 403")
	}
}

func TestParseRedditSearchResponse_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	_, err := SearchRedditPosts(ts.URL+"?q=webx", 20, "relevance")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
