// Package backends — Reddit write operation tests.
//
// Tests for SearchRedditPosts, PostReddit, CommentReddit, UpvoteReddit.
// All tests use httptest.NewServer to mock the Reddit OAuth API.
// No real network requests are made.
//
// Expected function signatures (for worker reference):
//
//   func PostReddit(apiURL, subreddit, title, content, accessToken string) (*types.NormalizedWriteResult, error)
//   func CommentReddit(apiURL, thingID, content, accessToken string) (*types.NormalizedWriteResult, error)
//   func VoteReddit(apiURL, thingID string, dir int, accessToken string) (*types.NormalizedWriteResult, error)

package backends

/*
import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

// buildRedditSubmitResponse returns a minimal Reddit /api/submit response.
func buildRedditSubmitResponse(postID string) []byte {
	return []byte(`{
		"json": {
			"errors": [],
			"data": {
				"id": "` + postID + `",
				"name": "t3_` + postID + `",
				"url": "https://www.reddit.com/r/golang/comments/` + postID + `/test_post/",
				"drafts_count": 0
			}
		}
	}`)
}

// buildRedditCommentResponse returns a minimal Reddit /api/comment response.
func buildRedditCommentResponse(commentID string) []byte {
	return []byte(`{
		"json": {
			"errors": [],
			"data": {
				"things": [
					{
						"kind": "t1",
						"data": {
							"id": "` + commentID + `",
							"name": "t1_` + commentID + `",
							"body": "This is a comment.",
							"author": "testuser"
						}
					}
				]
			}
		}
	}`)
}

// buildRedditVoteResponse returns a minimal Reddit /api/vote response (empty body on success).
func buildRedditVoteResponse() []byte {
	return []byte(`{}`)
}

// buildRedditAPIError returns a Reddit API error response.
func buildRedditAPIError(errMsg string) []byte {
	return []byte(`{
		"json": {
			"errors": [["` + errMsg + `", "` + errMsg + `", null]]
		}
	}`)
}

// --- PostReddit tests ---

func TestPostReddit_WhenValid_ShouldReturnPostURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditSubmitResponse("abc123"))
	}))
	defer ts.Close()

	result, err := PostReddit(ts.URL, "golang", "Test Post Title", "Test content body", "fake-access-token")
	if err != nil {
		t.Fatalf("PostReddit: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.ResourceURL == "" {
		t.Error("ResourceURL should not be empty")
	}
	if result.Action != string(types.ActionPost) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionPost)
	}
	if result.Backend == "" {
		t.Error("Backend should not be empty")
	}
}

func TestPostReddit_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := PostReddit("http://unused", "golang", "Title", "Content", "")
	if err == nil {
		t.Error("expected error when access token is empty")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("expected ErrLoginRequired, got %s", wxErr.Code)
	}
}

func TestPostReddit_WhenHTTPUnauthorized_ShouldReturnLoginRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message": "Unauthorized", "error": 401}`))
	}))
	defer ts.Close()

	_, err := PostReddit(ts.URL, "golang", "Title", "Content", "bad-token")
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("expected ErrLoginRequired, got %s", wxErr.Code)
	}
}

func TestPostReddit_WhenAPIReturnsErrors_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditAPIError("SUBREDDIT_NOTALLOWED"))
	}))
	defer ts.Close()

	_, err := PostReddit(ts.URL, "private_sub", "Title", "Content", "fake-token")
	if err == nil {
		t.Error("expected error when API returns errors array")
	}
}

func TestPostReddit_WhenRateLimited_ShouldReturnRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer ts.Close()

	_, err := PostReddit(ts.URL, "golang", "Title", "Content", "fake-token")
	if err == nil {
		t.Fatal("expected error for HTTP 429")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %s", wxErr.Code)
	}
}

// --- CommentReddit tests ---

func TestCommentReddit_WhenValid_ShouldReturnCommentURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditCommentResponse("def456"))
	}))
	defer ts.Close()

	result, err := CommentReddit(ts.URL, "t3_abc123", "This is my comment.", "fake-access-token")
	if err != nil {
		t.Fatalf("CommentReddit: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.Action != string(types.ActionReply) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionReply)
	}
}

func TestCommentReddit_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := CommentReddit("http://unused", "t3_abc123", "comment", "")
	if err == nil {
		t.Error("expected error when access token is empty")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("expected ErrLoginRequired, got %s", wxErr.Code)
	}
}

func TestCommentReddit_WhenHTTPForbidden_ShouldReturnLoginRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	_, err := CommentReddit(ts.URL, "t3_abc123", "comment", "bad-token")
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("expected ErrLoginRequired, got %s", wxErr.Code)
	}
}

// --- VoteReddit tests ---

func TestVoteReddit_WhenUpvote_ShouldSucceed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditVoteResponse())
	}))
	defer ts.Close()

	result, err := VoteReddit(ts.URL, "t3_abc123", 1, "fake-access-token")
	if err != nil {
		t.Fatalf("VoteReddit (upvote): %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true for upvote")
	}
	if result.Action != string(types.ActionReact) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionReact)
	}
}

func TestVoteReddit_WhenDownvote_ShouldSucceed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRedditVoteResponse())
	}))
	defer ts.Close()

	result, err := VoteReddit(ts.URL, "t3_abc123", -1, "fake-access-token")
	if err != nil {
		t.Fatalf("VoteReddit (downvote): %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true for downvote")
	}
}

func TestVoteReddit_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := VoteReddit("http://unused", "t3_abc123", 1, "")
	if err == nil {
		t.Error("expected error when access token is empty")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("expected ErrLoginRequired, got %s", wxErr.Code)
	}
}

func TestVoteReddit_WhenRateLimited_ShouldReturnRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer ts.Close()

	_, err := VoteReddit(ts.URL, "t3_abc123", 1, "fake-token")
	if err == nil {
		t.Fatal("expected error for HTTP 429")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %s", wxErr.Code)
	}
}

// --- Table-driven error paths ---

func TestRedditWriteOps_ErrorPaths(t *testing.T) {
	cases := []struct {
		name        string
		statusCode  int
		wantErrCode types.WebxErrorCode
	}{
		{"unauthorized", 401, types.ErrLoginRequired},
		{"forbidden", 403, types.ErrLoginRequired},
		{"rate_limited", 429, types.ErrRateLimited},
		{"server_error", 500, types.ErrFetchFailed},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("PostReddit/"+tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer ts.Close()

			_, err := PostReddit(ts.URL, "golang", "Title", "Content", "fake-token")
			if err == nil {
				t.Fatalf("expected error for HTTP %d", tc.statusCode)
			}
			wxErr, ok := err.(*types.WebxError)
			if !ok {
				t.Fatalf("expected *types.WebxError, got %T: %v", err, err)
			}
			if wxErr.Code != tc.wantErrCode {
				t.Errorf("error code: got %s, want %s", wxErr.Code, tc.wantErrCode)
			}
		})
	}
}
*/
