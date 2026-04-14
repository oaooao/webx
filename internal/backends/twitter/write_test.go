package twitter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

// buildCreateTweetResponse returns a minimal Twitter GraphQL CreateTweet response.
func buildCreateTweetResponse(tweetID string) []byte {
	return []byte(`{
		"data": {
			"create_tweet": {
				"tweet_results": {
					"result": {
						"__typename": "Tweet",
						"rest_id": "` + tweetID + `",
						"core": {
							"user_results": {
								"result": {
									"legacy": {
										"screen_name": "testuser"
									}
								}
							}
						},
						"legacy": {
							"full_text": "Hello world!",
							"created_at": "Mon Apr 14 10:00:00 +0000 2025"
						}
					}
				}
			}
		}
	}`)
}

// buildFavoriteTweetResponse returns a minimal FavoriteTweet response.
func buildFavoriteTweetResponse(tweetID string) []byte {
	return []byte(`{
		"data": {
			"favorite_tweet": "Done"
		}
	}`)
}

// buildRetweetResponse returns a minimal CreateRetweet response.
func buildRetweetResponse(tweetID string) []byte {
	return []byte(`{
		"data": {
			"create_retweet": {
				"retweet_results": {
					"result": {
						"__typename": "Tweet",
						"rest_id": "9999",
						"legacy": {
							"full_text": "RT @original: Hello world!"
						}
					}
				}
			}
		}
	}`)
}

func fakeAuth() *Auth {
	return &Auth{AuthToken: "fake-auth-token", CT0: "fake-ct0"}
}

// --- CreateTweet tests ---

func TestCreateTweet_WhenValid_ShouldReturnTweetURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildCreateTweetResponse("1234567890"))
	}))
	defer ts.Close()

	result, err := CreateTweetWithURL(ts.URL, "Hello world!", fakeAuth())
	if err != nil {
		t.Fatalf("CreateTweetWithURL: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.ResourceURL == "" {
		t.Error("ResourceURL should not be empty")
	}
	if result.Backend == "" {
		t.Error("Backend should not be empty")
	}
	if result.Action != string(types.ActionPost) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionPost)
	}
}

func TestCreateTweet_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := CreateTweet("Hello world!", nil)
	if err == nil {
		t.Error("expected error when auth is nil")
	}
}

func TestCreateTweet_WhenEmptyAuthToken_ShouldReturnError(t *testing.T) {
	auth := &Auth{AuthToken: "", CT0: ""}
	_, err := CreateTweetWithURL("http://unused", "Hello!", auth)
	if err == nil {
		t.Error("expected error for empty auth token")
	}
}

func TestCreateTweet_WhenHTTPUnauthorized_ShouldReturnLoginRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"errors": [{"message": "Authorization: Token invalid."}]}`))
	}))
	defer ts.Close()

	_, err := CreateTweetWithURL(ts.URL, "Hello!", fakeAuth())
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

func TestCreateTweet_WhenHTTPRateLimit_ShouldReturnRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer ts.Close()

	_, err := CreateTweetWithURL(ts.URL, "Hello!", fakeAuth())
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

func TestCreateTweet_WhenInvalidJSONResponse_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	_, err := CreateTweetWithURL(ts.URL, "Hello!", fakeAuth())
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

// --- ReplyTweet tests ---

func TestReplyTweet_WhenValid_ShouldReturnReplyURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildCreateTweetResponse("9876543210"))
	}))
	defer ts.Close()

	result, err := ReplyTweetWithURL(ts.URL, "Great post!", "1111111111", fakeAuth())
	if err != nil {
		t.Fatalf("ReplyTweetWithURL: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.Action != string(types.ActionReply) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionReply)
	}
	if result.ResourceURL == "" {
		t.Error("ResourceURL should not be empty")
	}
}

func TestReplyTweet_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := ReplyTweet("reply text", "123", nil)
	if err == nil {
		t.Error("expected error when auth is nil")
	}
}

func TestReplyTweet_WhenHTTPUnauthorized_ShouldReturnLoginRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	_, err := ReplyTweetWithURL(ts.URL, "reply", "123", fakeAuth())
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

// --- FavoriteTweet tests ---

func TestFavoriteTweet_WhenValid_ShouldSucceed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildFavoriteTweetResponse("1234567890"))
	}))
	defer ts.Close()

	result, err := FavoriteTweetWithURL(ts.URL, "1234567890", fakeAuth())
	if err != nil {
		t.Fatalf("FavoriteTweetWithURL: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.Action != string(types.ActionReact) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionReact)
	}
}

func TestFavoriteTweet_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := FavoriteTweet("123", nil)
	if err == nil {
		t.Error("expected error when auth is nil")
	}
}

func TestFavoriteTweet_WhenHTTPRateLimit_ShouldReturnRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer ts.Close()

	_, err := FavoriteTweetWithURL(ts.URL, "123", fakeAuth())
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

// --- RetweetTweet tests ---

func TestRetweetTweet_WhenValid_ShouldSucceed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildRetweetResponse("1234567890"))
	}))
	defer ts.Close()

	result, err := RetweetTweetWithURL(ts.URL, "1234567890", fakeAuth())
	if err != nil {
		t.Fatalf("RetweetTweetWithURL: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.Action != string(types.ActionReact) {
		t.Errorf("Action: got %q, want %q", result.Action, types.ActionReact)
	}
}

func TestRetweetTweet_WhenNoAuth_ShouldReturnError(t *testing.T) {
	_, err := RetweetTweet("123", nil)
	if err == nil {
		t.Error("expected error when auth is nil")
	}
}

func TestRetweetTweet_WhenHTTPUnauthorized_ShouldReturnLoginRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	_, err := RetweetTweetWithURL(ts.URL, "123", fakeAuth())
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

// --- Table-driven error path tests ---

func TestTwitterWriteOps_ErrorPaths(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		wantErrCode types.ErrorCode
	}{
		{"unauthorized", 401, types.ErrLoginRequired},
		{"forbidden", 403, types.ErrLoginRequired},
		{"rate_limited", 429, types.ErrRateLimited},
		{"server_error", 500, types.ErrFetchFailed},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer ts.Close()

			_, err := CreateTweetWithURL(ts.URL, "test", fakeAuth())
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
