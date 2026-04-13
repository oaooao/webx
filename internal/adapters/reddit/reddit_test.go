package reddit

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

func TestMatch_RedditDomains(t *testing.T) {
	a := New()

	matchCases := []struct {
		url  string
		want bool
	}{
		{"https://reddit.com/r/golang/comments/abc123/test", true},
		{"https://www.reddit.com/r/golang/comments/abc123/test", true},
		{"https://old.reddit.com/r/golang/comments/abc123/test", true},
		{"https://new.reddit.com/r/golang/comments/abc123/test", true},
	}

	for _, tc := range matchCases {
		ctx := types.MatchContext{URL: mustParseURL(t, tc.url)}
		got := a.Match(ctx)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestMatch_NonRedditDomains(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://x.com/user/status/123",
		"https://example.com/r/test",
		"https://notreddit.com/r/test",
	}

	for _, raw := range noMatchCases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if a.Match(ctx) {
			t.Errorf("Match(%q) = true, want false", raw)
		}
	}
}

func TestExtractPostID(t *testing.T) {
	cases := []struct {
		path   string
		wantID string
	}{
		{"/r/golang/comments/abc123/test_post", "abc123"},
		{"/r/golang/comments/xyz789/", "xyz789"},
		{"/r/golang/", ""},
		{"/r/golang", ""},
	}

	for _, tc := range cases {
		got := extractPostID(tc.path)
		if got != tc.wantID {
			t.Errorf("extractPostID(%q) = %q, want %q", tc.path, got, tc.wantID)
		}
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}

func TestAdapterMeta(t *testing.T) {
	a := New()
	if a.ID() != "reddit" {
		t.Errorf("ID: got %q, want %q", a.ID(), "reddit")
	}
	if a.Priority() != 90 {
		t.Errorf("Priority: got %d, want 90", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 3 {
		t.Fatalf("Kinds: expected 3, got %d", len(kinds))
	}
}
