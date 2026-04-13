package twitter

import (
	"net/url"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL %s: %v", raw, err)
	}
	return u
}

func TestMatch_TwitterDomains(t *testing.T) {
	a := New()

	matchCases := []struct {
		url  string
		want bool
	}{
		{"https://x.com/user/status/123", true},
		{"https://twitter.com/user/status/123", true},
		{"https://www.x.com/user/status/123", true},
		{"https://www.twitter.com/user/status/123", true},
		{"https://x.com/home", true},
		{"https://twitter.com/i/spaces/1234", true},
	}

	for _, tc := range matchCases {
		ctx := types.MatchContext{URL: mustParseURL(t, tc.url)}
		got := a.Match(ctx)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestMatch_NonTwitterDomains(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://reddit.com/r/test",
		"https://example.com/status/123",
		"https://notx.com/user/status/123",
		"https://xtwitter.com/user/status/123",
	}

	for _, raw := range noMatchCases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if a.Match(ctx) {
			t.Errorf("Match(%q) = true, want false", raw)
		}
	}
}

func TestTweetID_Extraction(t *testing.T) {
	cases := []struct {
		url    string
		wantID string
	}{
		{"https://x.com/user/status/12345", "12345"},
		{"https://twitter.com/user/status/67890", "67890"},
		{"https://x.com/user/status/12345?s=20", "12345"},
		{"https://x.com/user/status/12345#fragment", "12345"},
		{"https://x.com/user/status/12345/", "12345"},
		{"https://www.x.com/user/status/99999/photo/1", "99999"},
	}

	for _, tc := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, tc.url)}
		got := tweetID(ctx)
		if got != tc.wantID {
			t.Errorf("tweetID(%q) = %q, want %q", tc.url, got, tc.wantID)
		}
	}
}

func TestTweetID_NonTweetURLs(t *testing.T) {
	noIDCases := []string{
		"https://x.com/home",
		"https://x.com/user",
		"https://x.com/user/likes",
		"https://x.com/i/spaces/1234",
		"https://twitter.com/explore",
	}

	for _, raw := range noIDCases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		got := tweetID(ctx)
		if got != "" {
			t.Errorf("tweetID(%q) = %q, want empty string", raw, got)
		}
	}
}

func TestAdapterMeta(t *testing.T) {
	a := New()
	if a.ID() != "twitter" {
		t.Errorf("ID: got %q, want %q", a.ID(), "twitter")
	}
	if a.Priority() != 90 {
		t.Errorf("Priority: got %d, want 90", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 2 {
		t.Fatalf("Kinds: expected 2, got %d", len(kinds))
	}
	foundThread, foundMetadata := false, false
	for _, k := range kinds {
		switch k {
		case types.KindThread:
			foundThread = true
		case types.KindMetadata:
			foundMetadata = true
		}
	}
	if !foundThread {
		t.Error("Kinds should include KindThread")
	}
	if !foundMetadata {
		t.Error("Kinds should include KindMetadata")
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}
