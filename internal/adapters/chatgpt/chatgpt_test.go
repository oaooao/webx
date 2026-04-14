package chatgpt

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

func TestMatch_WhenChatGPTShareURL_ShouldReturnTrue(t *testing.T) {
	a := New()

	cases := []string{
		"https://chatgpt.com/share/69ddbda6-a51c-83ea-af56-fa0be87039e6",
		"https://chat.openai.com/share/abc-def-ghi",
	}

	for _, raw := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if !a.Match(ctx) {
			t.Errorf("Match(%q) = false, want true", raw)
		}
	}
}

func TestMatch_WhenNonChatGPTURLs_ShouldReturnFalse(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://chatgpt.com/c/some-chat-id",
		"https://chatgpt.com/",
		"https://example.com/share/uuid",
		"https://openai.com/share/uuid",
		"https://chat.openai.com/auth/login",
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
	if a.ID() != "chatgpt-share" {
		t.Errorf("ID: got %q, want %q", a.ID(), "chatgpt-share")
	}
	if a.Priority() != 91 {
		t.Errorf("Priority: got %d, want 91", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 3 {
		t.Fatalf("Kinds: expected 3, got %d", len(kinds))
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}
