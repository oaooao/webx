package claude

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

func TestMatch_WhenClaudeShareURL_ShouldReturnTrue(t *testing.T) {
	a := New()

	cases := []string{
		"https://claude.ai/share/48088842-673f-4ef9-a867-a8add9e71549",
		"https://claude.ai/share/abc-def-ghi",
	}

	for _, raw := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if !a.Match(ctx) {
			t.Errorf("Match(%q) = false, want true", raw)
		}
	}
}

func TestMatch_WhenNonClaudeURLs_ShouldReturnFalse(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://claude.ai/chat/some-id",
		"https://claude.ai/",
		"https://example.com/share/uuid",
		"https://notclaude.ai/share/uuid",
		"https://claude.ai/settings",
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
	if a.ID() != "claude-share" {
		t.Errorf("ID: got %q, want %q", a.ID(), "claude-share")
	}
	if a.Priority() != 91 {
		t.Errorf("Priority: got %d, want 91", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 3 {
		t.Fatalf("Kinds: expected 3, got %d", len(kinds))
	}

	foundConversation := false
	for _, k := range kinds {
		if k == types.KindConversation {
			foundConversation = true
		}
	}
	if !foundConversation {
		t.Error("Kinds should include KindConversation")
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}
