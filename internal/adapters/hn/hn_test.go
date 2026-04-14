package hn

import (
	"net/url"
	"testing"

	"github.com/oaooao/webx/internal/backends"
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

func TestMatch_WhenValidHNItemURL_ShouldReturnTrue(t *testing.T) {
	a := New()

	cases := []string{
		"https://news.ycombinator.com/item?id=12345",
		"https://news.ycombinator.com/item?id=99999999",
	}

	for _, raw := range cases {
		ctx := types.MatchContext{URL: mustParseURL(t, raw)}
		if !a.Match(ctx) {
			t.Errorf("Match(%q) = false, want true", raw)
		}
	}
}

func TestMatch_WhenNonItemHNURL_ShouldReturnFalse(t *testing.T) {
	a := New()

	noMatchCases := []string{
		"https://news.ycombinator.com/newest",
		"https://news.ycombinator.com/show",
		"https://news.ycombinator.com/ask",
		"https://news.ycombinator.com/jobs",
		"https://news.ycombinator.com/",
		"https://news.ycombinator.com/item",             // no id param
		"https://news.ycombinator.com/item?id=",         // empty id
		"https://example.com/item?id=12345",             // wrong domain
		"https://news.ycombinator.com/submit?goto=news", // not /item path
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
	if a.ID() != "hacker-news" {
		t.Errorf("ID: got %q, want %q", a.ID(), "hacker-news")
	}
	if a.Priority() != 88 {
		t.Errorf("Priority: got %d, want 88", a.Priority())
	}

	kinds := a.Kinds()
	if len(kinds) != 4 {
		t.Fatalf("Kinds: expected 4, got %d", len(kinds))
	}
}

func TestNewReturnsExtractableAdapter(t *testing.T) {
	a := New()
	if _, ok := a.(types.ExtractableAdapter); !ok {
		t.Error("New() should return an ExtractableAdapter")
	}
}

func TestConvertComments_WhenEmpty_ShouldReturnNil(t *testing.T) {
	got := convertComments(nil)
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestConvertComments_WhenHasChildren_ShouldBuildTree(t *testing.T) {
	text := "hello world"
	childText := "reply text"
	items := []backends.HNItem{
		{
			ID:        1,
			Author:    "user1",
			Text:      &text,
			CreatedAt: "2024-01-01",
			Children: []backends.HNItem{
				{
					ID:        2,
					Author:    "user2",
					Text:      &childText,
					CreatedAt: "2024-01-02",
				},
			},
		},
	}

	nodes := convertComments(items)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Author != "user1" {
		t.Errorf("Author: got %q, want %q", nodes[0].Author, "user1")
	}
	if len(nodes[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(nodes[0].Children))
	}
	if nodes[0].Children[0].Author != "user2" {
		t.Errorf("Child Author: got %q, want %q", nodes[0].Children[0].Author, "user2")
	}
}
