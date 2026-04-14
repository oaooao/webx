package generic

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

func TestMatch_WhenHTTPScheme_ShouldReturnTrue(t *testing.T) {
	a := New()

	cases := []string{
		"https://example.com",
		"http://example.com",
		"https://any-domain.org/any/path",
		"http://localhost:8080/test",
	}

	for _, raw := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if !a.Match(ctx) {
			t.Errorf("Match(%q) = false, want true", raw)
		}
	}
}

func TestMatch_WhenNonHTTPScheme_ShouldReturnFalse(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"ftp://example.com/file.txt",
		"file:///tmp/test.html",
		"mailto:user@example.com",
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
	if a.ID() != "generic-article" {
		t.Errorf("ID: got %q, want %q", a.ID(), "generic-article")
	}
	if a.Priority() != 10 {
		t.Errorf("Priority: got %d, want 10", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 1 || kinds[0] != types.KindArticle {
		t.Errorf("Kinds: got %v, want [article]", kinds)
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}

func TestPtrOf_WhenEmpty_ShouldReturnNil(t *testing.T) {
	if ptrOf("") != nil {
		t.Error("ptrOf(\"\") should return nil")
	}
}

func TestPtrOf_WhenNonEmpty_ShouldReturnPointer(t *testing.T) {
	p := ptrOf("hello")
	if p == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *p != "hello" {
		t.Errorf("got %q, want %q", *p, "hello")
	}
}
