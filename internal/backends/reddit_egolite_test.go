package backends

import (
	"errors"
	"strings"
	"testing"

	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

// fakeRedditEgoLite records the job and answers every request with status.
func fakeRedditEgoLite(t *testing.T, status int, body string) *egolite.Job {
	t.Helper()
	old := redditEgoLiteRun
	t.Cleanup(func() { redditEgoLiteRun = old })
	var seen egolite.Job
	redditEgoLiteRun = func(job egolite.Job) (*egolite.Result, error) {
		seen = job
		resps := make([]egolite.Response, len(job.Requests))
		for i, r := range job.Requests {
			resps[i] = egolite.Response{Status: status, URL: r.URL, Body: body}
		}
		return &egolite.Result{Responses: resps}, nil
	}
	return &seen
}

// Regression for 2026-11-21: a .json fetch issued from
// https://www.reddit.com/robots.txt got 403 in the signed-in ego lite
// browser; the same fetch issued from the thread or subreddit HTML page got
// 200. The origin page must be the HTML page the JSON belongs to.
func TestRedditEgoLite_OpensTheHTMLPageBeforeFetchingJSON(t *testing.T) {
	cases := map[string]string{
		"https://old.reddit.com/r/golang/comments/1wdjr5u/rune/":     "https://www.reddit.com/r/golang/comments/1wdjr5u/rune/",
		"https://www.reddit.com/r/ExperiencedDevs/top/?t=day":        "https://www.reddit.com/r/ExperiencedDevs/top/",
		"https://www.reddit.com/r/golang/comments/1wdjr5u/rune.json": "https://www.reddit.com/r/golang/comments/1wdjr5u/rune/",
	}
	for in, wantOrigin := range cases {
		seen := fakeRedditEgoLite(t, 200, `[]`)
		if _, err := FetchRedditJSONViaEgoLite(in); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if seen.Origin != wantOrigin {
			t.Errorf("%s: origin = %q, want %q", in, seen.Origin, wantOrigin)
		}
		if len(seen.Requests) != 1 || !strings.HasPrefix(seen.Requests[0].URL, "https://www.reddit.com/") {
			t.Errorf("%s: request must stay same-origin with the page: %+v", in, seen.Requests)
		}
	}
}

// Regression for 2026-11-21: search.json is 403 from any page, signed in or
// not. The error must tell the calling agent what to do instead.
func TestRedditEgoLiteSearch_403_SuggestsSubredditListings(t *testing.T) {
	seen := fakeRedditEgoLite(t, 403, "")
	_, err := SearchRedditPostsViaEgoLite(BuildRedditSearchURL("agentic coding", 10, "top"))
	var wxErr *types.WebxError
	if err == nil || !errors.As(err, &wxErr) {
		t.Fatalf("err = %v, want WebxError", err)
	}
	if wxErr.Code != types.ErrBackendUnavailable {
		t.Errorf("code = %s, want BACKEND_UNAVAILABLE (signing in again does not help)", wxErr.Code)
	}
	for _, want := range []string{"Reddit search is unavailable", "/top/?t=week", "/new/"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should contain %q", err, want)
		}
	}
	if seen.Origin != "https://www.reddit.com/" {
		t.Errorf("search origin = %q, want the Reddit home page", seen.Origin)
	}
}
