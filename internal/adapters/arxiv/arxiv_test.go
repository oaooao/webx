package arxiv

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

func TestMatch_WhenArxivURLs_ShouldReturnTrue(t *testing.T) {
	a := New()

	cases := []string{
		"https://arxiv.org/abs/2401.00001",
		"https://arxiv.org/abs/2401.00001v2",
		"https://arxiv.org/pdf/2401.00001",
		"https://arxiv.org/pdf/2401.00001.pdf",
		"https://arxiv.org/html/2401.00001",
	}

	for _, raw := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if !a.Match(ctx) {
			t.Errorf("Match(%q) = false, want true", raw)
		}
	}
}

func TestMatch_WhenNonArxivURLs_ShouldReturnFalse(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://example.com/abs/2401.00001",
		"https://arxiv.org/list/cs.AI",
		"https://arxiv.org/search/?query=test",
		"https://arxiv.org/",
		"https://notarxiv.org/abs/2401.00001",
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
	if a.ID() != "arxiv" {
		t.Errorf("ID: got %q, want %q", a.ID(), "arxiv")
	}
	if a.Priority() != 80 {
		t.Errorf("Priority: got %d, want 80", a.Priority())
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

func TestRewriteToHTML_WhenAbsURL_ShouldRewriteCorrectly(t *testing.T) {
	cases := []struct {
		path    string
		origURL string
		want    string
	}{
		{"/abs/2401.00001", "https://arxiv.org/abs/2401.00001", "https://arxiv.org/html/2401.00001"},
		{"/abs/2401.00001v2", "https://arxiv.org/abs/2401.00001v2", "https://arxiv.org/html/2401.00001"},
		{"/pdf/2401.00001.pdf", "https://arxiv.org/pdf/2401.00001.pdf", "https://arxiv.org/html/2401.00001"},
		{"/pdf/2401.00001", "https://arxiv.org/pdf/2401.00001", "https://arxiv.org/html/2401.00001"},
		{"/html/2401.00001", "https://arxiv.org/html/2401.00001", "https://arxiv.org/html/2401.00001"},
	}

	for _, tc := range cases {
		got := rewriteToHTML(tc.path, tc.origURL)
		if got != tc.want {
			t.Errorf("rewriteToHTML(%q, %q) = %q, want %q", tc.path, tc.origURL, got, tc.want)
		}
	}
}

func TestStripVersion_ShouldRemoveVersionSuffix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2401.00001v2", "2401.00001"},
		{"2401.00001v10", "2401.00001"},
		{"2401.00001", "2401.00001"},
		{"2503.23350v1", "2503.23350"},
	}

	for _, tc := range cases {
		got := stripVersion(tc.input)
		if got != tc.want {
			t.Errorf("stripVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractArxivID_ShouldExtractFromVariousPaths(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/abs/2401.00001v2", "2401.00001v2"},
		{"/pdf/2401.00001.pdf", "2401.00001"},
		{"/html/2401.00001", "2401.00001"},
		{"/other/path", ""},
	}

	for _, tc := range cases {
		got := extractArxivID(tc.path)
		if got != tc.want {
			t.Errorf("extractArxivID(%q) = %q, want %q", tc.path, got, tc.want)
		}
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
