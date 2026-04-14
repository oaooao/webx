package youtube

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

func TestMatch_WhenYouTubeDomains_ShouldReturnTrue(t *testing.T) {
	a := New()

	cases := []struct {
		url  string
		want bool
	}{
		{"https://youtube.com/watch?v=abc123def45", true},
		{"https://www.youtube.com/watch?v=abc123def45", true},
		{"https://m.youtube.com/watch?v=abc123def45", true},
		{"https://youtu.be/abc123def45", true},
		{"https://www.youtube.com/shorts/abc123def45", true},
		{"https://www.youtube.com/playlist?list=PL123", true},
	}

	for _, tc := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, tc.url)}
		got := a.Match(ctx)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestMatch_WhenNonYouTubeDomains_ShouldReturnFalse(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://vimeo.com/12345",
		"https://example.com/watch?v=abc",
		"https://notyoutube.com/watch?v=abc",
		"https://reddit.com/r/test",
	}

	for _, raw := range noMatchCases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if a.Match(ctx) {
			t.Errorf("Match(%q) = true, want false", raw)
		}
	}
}

func TestAdapterMeta_ShouldReturnCorrectValues(t *testing.T) {
	a := New()
	if a.ID() != "youtube" {
		t.Errorf("ID: got %q, want %q", a.ID(), "youtube")
	}
	if a.Priority() != 89 {
		t.Errorf("Priority: got %d, want 89", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 3 {
		t.Fatalf("Kinds: expected 3, got %d", len(kinds))
	}

	foundVideo := false
	for _, k := range kinds {
		if k == types.KindVideo {
			foundVideo = true
		}
	}
	if !foundVideo {
		t.Error("Kinds should include KindVideo")
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}
