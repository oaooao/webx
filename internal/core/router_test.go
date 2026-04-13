package core

import (
	"net/url"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

// mockAdapter implements types.Adapter for testing.
type mockAdapter struct {
	id       string
	priority int
	kinds    []types.WebxKind
	hostname string // matches URLs with this hostname
}

func (m *mockAdapter) ID() string              { return m.id }
func (m *mockAdapter) Priority() int           { return m.priority }
func (m *mockAdapter) Kinds() []types.WebxKind { return m.kinds }
func (m *mockAdapter) Match(ctx types.MatchContext) bool {
	return ctx.URL != nil && ctx.URL.Hostname() == m.hostname
}
func (m *mockAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	return &types.NormalizedReadResult{Backend: m.id}, nil
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL %s: %v", raw, err)
	}
	return u
}

func TestRouteMatchesAdapter(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "test", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	ctx := types.MatchContext{URL: mustParseURL(t, "https://example.com/page")}
	got := Route(ctx)
	if got == nil {
		t.Fatal("expected adapter, got nil")
	}
	if got.ID() != "test" {
		t.Fatalf("expected id=test, got %s", got.ID())
	}
}

func TestRouteNoMatchReturnsNil(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "test", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	ctx := types.MatchContext{URL: mustParseURL(t, "https://other.com/page")}
	got := Route(ctx)
	if got != nil {
		t.Fatalf("expected nil, got %s", got.ID())
	}
}

func TestRouteKindFilterMismatch(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "article-only", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	conversationKind := types.KindConversation
	ctx := types.MatchContext{URL: mustParseURL(t, "https://example.com/page"), RequestedKind: &conversationKind}
	got := Route(ctx)
	if got != nil {
		t.Fatalf("expected nil (kind mismatch), got %s", got.ID())
	}
}

func TestRouteKindFilterMatch(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "article-adapter", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	articleKind := types.KindArticle
	ctx := types.MatchContext{URL: mustParseURL(t, "https://example.com/page"), RequestedKind: &articleKind}
	got := Route(ctx)
	if got == nil {
		t.Fatal("expected adapter, got nil")
	}
	if got.ID() != "article-adapter" {
		t.Fatalf("expected article-adapter, got %s", got.ID())
	}
}

func TestRoutePriorityOrdering(t *testing.T) {
	ResetRegistry()
	low := &mockAdapter{id: "low", priority: 10, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	high := &mockAdapter{id: "high", priority: 200, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	// Register low first, high second — high should win due to priority sorting
	RegisterAdapter(low)
	RegisterAdapter(high)

	ctx := types.MatchContext{URL: mustParseURL(t, "https://example.com/page")}
	got := Route(ctx)
	if got == nil {
		t.Fatal("expected adapter, got nil")
	}
	if got.ID() != "high" {
		t.Fatalf("expected high-priority adapter, got %s", got.ID())
	}
}

func TestListAdapters(t *testing.T) {
	ResetRegistry()
	a1 := &mockAdapter{id: "a1", priority: 10, kinds: []types.WebxKind{types.KindArticle}, hostname: "a.com"}
	a2 := &mockAdapter{id: "a2", priority: 20, kinds: []types.WebxKind{types.KindThread}, hostname: "b.com"}
	RegisterAdapter(a1)
	RegisterAdapter(a2)

	list := ListAdapters()
	if len(list) != 2 {
		t.Fatalf("expected 2 adapters, got %d", len(list))
	}
}

func TestResetRegistry(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "test", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)
	ResetRegistry()

	list := ListAdapters()
	if len(list) != 0 {
		t.Fatalf("expected empty registry after reset, got %d", len(list))
	}
}
