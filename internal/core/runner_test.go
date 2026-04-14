package core

import (
	"testing"

	"github.com/oaooao/webx/internal/types"
)

// errorAdapter is a mockAdapter whose Read always returns a WebxError.
type errorAdapter struct {
	mockAdapter
	readErr *types.WebxError
}

func (e *errorAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	return nil, e.readErr
}

// extractableAdapter is a mockAdapter that also implements ExtractableAdapter.
type extractableAdapter struct {
	mockAdapter
	extractResult *types.NormalizedExtractResult
	extractErr    error
}

func (e *extractableAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	if e.extractErr != nil {
		return nil, e.extractErr
	}
	return e.extractResult, nil
}

func TestRunDoctorKnownURL(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "test", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	env := RunDoctor("https://example.com/page", nil)
	if !env.OK {
		t.Fatalf("expected ok=true for known URL, got ok=false, error=%v", env.Error)
	}
	if env.Source.Adapter != "test" {
		t.Fatalf("expected adapter=test, got %s", env.Source.Adapter)
	}
	if len(env.Trace) == 0 {
		t.Fatal("expected non-empty trace")
	}
}

func TestRunDoctorUnknownURL(t *testing.T) {
	ResetRegistry()

	env := RunDoctor("https://unknown.example.com/page", nil)
	if env.OK {
		t.Fatal("expected ok=false for unknown URL")
	}
	if env.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if env.Error.Code != string(types.ErrNoMatch) {
		t.Fatalf("expected NO_MATCH error code, got %s", env.Error.Code)
	}
}

func TestRunDoctorTraceNotNil(t *testing.T) {
	ResetRegistry()
	env := RunDoctor("https://unknown.example.com/page", nil)
	if env.Trace == nil {
		t.Fatal("Trace must be non-nil (should be [] not null in JSON)")
	}
}

func TestRunReadSuccess(t *testing.T) {
	ResetRegistry()
	title := "Test Title"
	markdown := "# Hello World"
	adapter := &mockAdapter{id: "test", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	// Override Read to return content
	_ = adapter
	// Use a custom read adapter
	type readAdapter struct {
		mockAdapter
	}
	ra := &struct {
		mockAdapter
	}{
		mockAdapter: mockAdapter{id: "test", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"},
	}
	_ = ra
	// Simpler: just use the default mockAdapter which returns Backend=id
	RegisterAdapter(adapter)

	env := RunRead("https://example.com/page", nil)
	if !env.OK {
		t.Fatalf("expected ok=true, got ok=false, error=%v", env.Error)
	}
	if env.Source.Backend != "test" {
		t.Fatalf("expected backend=test, got %s", env.Source.Backend)
	}
	_ = title
	_ = markdown
}

func TestRunReadAdapterError(t *testing.T) {
	ResetRegistry()
	wxErr := types.NewWebxError(types.ErrAntiBot, "blocked by anti-bot")
	adapter := &errorAdapter{
		mockAdapter: mockAdapter{id: "error-adapter", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"},
		readErr:     wxErr,
	}
	RegisterAdapter(adapter)

	env := RunRead("https://example.com/page", nil)
	if env.OK {
		t.Fatal("expected ok=false when adapter returns error")
	}
	if env.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if env.Error.Code != string(types.ErrAntiBot) {
		t.Fatalf("expected ANTI_BOT error code, got %s", env.Error.Code)
	}
}

func TestRunReadNoMatch(t *testing.T) {
	ResetRegistry()

	env := RunRead("https://nomatch.example.com/page", nil)
	if env.OK {
		t.Fatal("expected ok=false for no match")
	}
	if env.Error.Code != string(types.ErrNoMatch) {
		t.Fatalf("expected NO_MATCH, got %s", env.Error.Code)
	}
}

func TestRunExtractNoMatch(t *testing.T) {
	ResetRegistry()
	kind := types.KindConversation
	env := RunExtract("https://nomatch.example.com/page", &kind)
	if env.OK {
		t.Fatal("expected ok=false for no match")
	}
	if env.Error.Code != string(types.ErrNoMatch) {
		t.Fatalf("expected NO_MATCH, got %s", env.Error.Code)
	}
	if env.Kind != types.KindConversation {
		t.Fatalf("expected kind=conversation, got %s", env.Kind)
	}
}

func TestRunExtractNotExtractable(t *testing.T) {
	ResetRegistry()
	// mockAdapter does NOT implement ExtractableAdapter
	adapter := &mockAdapter{id: "plain", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	articleKind := types.KindArticle
	env := RunExtract("https://example.com/page", &articleKind)
	if env.OK {
		t.Fatal("expected ok=false when adapter doesn't support extract")
	}
	if env.Error.Code != string(types.ErrUnsupportedKind) {
		t.Fatalf("expected UNSUPPORTED_KIND, got %s", env.Error.Code)
	}
}

func TestRunExtractSuccess(t *testing.T) {
	ResetRegistry()
	title := types.StringPtr("Chat Title")
	result := &types.NormalizedExtractResult{
		Title:   title,
		Backend: "mock-backend",
		Data:    map[string]any{"messages": 42},
	}
	adapter := &extractableAdapter{
		mockAdapter: mockAdapter{
			id: "extractable", priority: 100,
			kinds:    []types.WebxKind{types.KindConversation},
			hostname: "chat.example.com",
		},
		extractResult: result,
	}
	RegisterAdapter(adapter)

	kind := types.KindConversation
	env := RunExtract("https://chat.example.com/thread/1", &kind)
	if !env.OK {
		t.Fatalf("expected ok=true, got ok=false, error=%v", env.Error)
	}
	if env.Source.Backend != "mock-backend" {
		t.Fatalf("expected backend=mock-backend, got %s", env.Source.Backend)
	}
}

func TestSchemaVersion(t *testing.T) {
	ResetRegistry()
	env := RunRead("https://nothing.example.com/", nil)
	if env.SchemaVersion != "1" {
		t.Fatalf("expected schema_version=1, got %s", env.SchemaVersion)
	}
}

func TestEnvelopeSourceDomain(t *testing.T) {
	ResetRegistry()
	env := RunDoctor("https://example.com/some/path", nil)
	if env.Source.Domain != "example.com" {
		t.Fatalf("expected domain=example.com, got %s", env.Source.Domain)
	}
}

// --- RunSearch tests ---

// searchableAdapter is a mockAdapter that also implements SearchableAdapter.
type searchableAdapter struct {
	mockAdapter
	searchResult *types.NormalizedSearchResult
	searchErr    error
}

func (s *searchableAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, nil
}

func TestRunSearch_WhenPlatformNotFound_ShouldReturnError(t *testing.T) {
	ResetRegistry()

	env := RunSearch("test query", "nonexistent", types.DefaultSearchOptions())
	if env.OK {
		t.Fatal("expected ok=false for unknown platform")
	}
	if env.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if env.Error.Code != string(types.ErrNoMatch) {
		t.Fatalf("expected NO_MATCH error, got %s", env.Error.Code)
	}
	if env.Kind != types.KindSearch {
		t.Fatalf("expected kind=search, got %s", env.Kind)
	}
}

func TestRunSearch_WhenNotSearchable_ShouldReturnError(t *testing.T) {
	ResetRegistry()
	// mockAdapter does NOT implement SearchableAdapter
	adapter := &mockAdapter{id: "plain", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	env := RunSearch("test query", "plain", types.DefaultSearchOptions())
	if env.OK {
		t.Fatal("expected ok=false when adapter doesn't support search")
	}
	if env.Error.Code != string(types.ErrUnsupportedKind) {
		t.Fatalf("expected UNSUPPORTED_KIND, got %s", env.Error.Code)
	}
}

func TestRunSearch_WhenSuccess_ShouldReturnResults(t *testing.T) {
	ResetRegistry()
	result := &types.NormalizedSearchResult{
		Items: []types.SearchResultItem{
			{Title: "Result 1", URL: "https://example.com/1", Author: "user1", Kind: types.KindArticle},
			{Title: "Result 2", URL: "https://example.com/2", Author: "user2", Kind: types.KindArticle},
		},
		Query:   "test query",
		Backend: "mock_search",
		HasMore: false,
	}
	adapter := &searchableAdapter{
		mockAdapter:  mockAdapter{id: "test-search", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"},
		searchResult: result,
	}
	RegisterAdapter(adapter)

	env := RunSearch("test query", "test-search", types.DefaultSearchOptions())
	if !env.OK {
		t.Fatalf("expected ok=true, got ok=false, error=%v", env.Error)
	}
	if env.Kind != types.KindSearch {
		t.Fatalf("expected kind=search, got %s", env.Kind)
	}
	if env.Source.Adapter != "test-search" {
		t.Fatalf("expected adapter=test-search, got %s", env.Source.Adapter)
	}
	if env.Source.Backend != "mock_search" {
		t.Fatalf("expected backend=mock_search, got %s", env.Source.Backend)
	}
	if env.Content.Markdown == nil {
		t.Fatal("expected non-nil markdown")
	}
	if env.Data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestRunSearch_WhenAdapterReturnsError_ShouldReturnErrorEnvelope(t *testing.T) {
	ResetRegistry()
	adapter := &searchableAdapter{
		mockAdapter: mockAdapter{id: "failing", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"},
		searchErr:   types.NewWebxError(types.ErrRateLimited, "too many requests"),
	}
	RegisterAdapter(adapter)

	env := RunSearch("test query", "failing", types.DefaultSearchOptions())
	if env.OK {
		t.Fatal("expected ok=false when search returns error")
	}
	if env.Error.Code != string(types.ErrRateLimited) {
		t.Fatalf("expected RATE_LIMITED, got %s", env.Error.Code)
	}
}

func TestRenderSearchMarkdown_WhenResults_ShouldContainTitles(t *testing.T) {
	result := &types.NormalizedSearchResult{
		Items: []types.SearchResultItem{
			{Title: "First Result", URL: "https://example.com/1", Author: "alice", Score: 42},
			{Title: "Second Result", URL: "https://example.com/2", Snippet: "A brief description"},
		},
		Query: "test query",
	}
	md := RenderSearchMarkdown(result)

	checks := []string{
		"Search Results",
		"test query",
		"First Result",
		"Second Result",
		"alice",
		"42",
		"A brief description",
	}
	for _, want := range checks {
		if !contains(md, want) {
			t.Errorf("markdown missing %q in:\n%s", want, md)
		}
	}
}

func TestRenderSearchMarkdown_WhenEmpty_ShouldReturnNoResults(t *testing.T) {
	result := &types.NormalizedSearchResult{
		Items: []types.SearchResultItem{},
		Query: "empty query",
	}
	md := RenderSearchMarkdown(result)
	if md != "No results found.\n" {
		t.Errorf("expected 'No results found.' for empty results, got %q", md)
	}
}

func TestRenderSearchMarkdown_WhenNil_ShouldReturnNoResults(t *testing.T) {
	md := RenderSearchMarkdown(nil)
	if md != "No results found.\n" {
		t.Errorf("expected 'No results found.' for nil, got %q", md)
	}
}

// contains is a helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- RunPost / RunReply / RunReact tests ---

// writableAdapter is a mockAdapter that also implements WritableAdapter.
type writableAdapter struct {
	mockAdapter
	postResult  *types.NormalizedWriteResult
	postErr     error
	replyResult *types.NormalizedWriteResult
	replyErr    error
	reactResult *types.NormalizedWriteResult
	reactErr    error
}

func (w *writableAdapter) Post(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	if w.postErr != nil {
		return nil, w.postErr
	}
	return w.postResult, nil
}

func (w *writableAdapter) Reply(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	if w.replyErr != nil {
		return nil, w.replyErr
	}
	return w.replyResult, nil
}

func (w *writableAdapter) React(ctx types.WriteContext) (*types.NormalizedWriteResult, error) {
	if w.reactErr != nil {
		return nil, w.reactErr
	}
	return w.reactResult, nil
}

func TestRunPost_WhenPlatformNotFound_ShouldReturnError(t *testing.T) {
	ResetRegistry()

	env := RunPost("nonexistent", "Hello world!")
	if env.OK {
		t.Fatal("expected ok=false for unknown platform")
	}
	if env.Error.Code != string(types.ErrNoMatch) {
		t.Fatalf("expected NO_MATCH, got %s", env.Error.Code)
	}
	if env.Kind != types.KindWrite {
		t.Fatalf("expected kind=write, got %s", env.Kind)
	}
}

func TestRunPost_WhenNotWritable_ShouldReturnError(t *testing.T) {
	ResetRegistry()
	adapter := &mockAdapter{id: "readonly", priority: 100, kinds: []types.WebxKind{types.KindArticle}, hostname: "example.com"}
	RegisterAdapter(adapter)

	env := RunPost("readonly", "Hello!")
	if env.OK {
		t.Fatal("expected ok=false when adapter doesn't support write")
	}
	if env.Error.Code != string(types.ErrUnsupportedKind) {
		t.Fatalf("expected UNSUPPORTED_KIND, got %s", env.Error.Code)
	}
}

func TestRunPost_WhenSuccess_ShouldReturnResult(t *testing.T) {
	ResetRegistry()
	adapter := &writableAdapter{
		mockAdapter: mockAdapter{id: "test-write", priority: 100, kinds: []types.WebxKind{types.KindThread}, hostname: "example.com"},
		postResult: &types.NormalizedWriteResult{
			Success:     true,
			Action:      "post",
			ResourceURL: "https://example.com/post/123",
			Message:     "Posted successfully",
			Backend:     "mock_write",
		},
	}
	RegisterAdapter(adapter)

	env := RunPost("test-write", "Hello world!")
	if !env.OK {
		t.Fatalf("expected ok=true, got ok=false, error=%v", env.Error)
	}
	if env.Kind != types.KindWrite {
		t.Fatalf("expected kind=write, got %s", env.Kind)
	}
	if env.Source.Adapter != "test-write" {
		t.Fatalf("expected adapter=test-write, got %s", env.Source.Adapter)
	}
	if env.Source.Backend != "mock_write" {
		t.Fatalf("expected backend=mock_write, got %s", env.Source.Backend)
	}
	if env.Data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestRunReply_WhenSuccess_ShouldRouteByURL(t *testing.T) {
	ResetRegistry()
	adapter := &writableAdapter{
		mockAdapter: mockAdapter{id: "test-reply", priority: 100, kinds: []types.WebxKind{types.KindThread}, hostname: "example.com"},
		replyResult: &types.NormalizedWriteResult{
			Success:     true,
			Action:      "reply",
			ResourceURL: "https://example.com/post/123/reply/456",
			Backend:     "mock_write",
		},
	}
	RegisterAdapter(adapter)

	env := RunReply("https://example.com/post/123", "Great post!")
	if !env.OK {
		t.Fatalf("expected ok=true, got ok=false, error=%v", env.Error)
	}
	if env.Source.Adapter != "test-reply" {
		t.Fatalf("expected adapter=test-reply, got %s", env.Source.Adapter)
	}
}

func TestRunReact_WhenSuccess_ShouldRouteByURL(t *testing.T) {
	ResetRegistry()
	adapter := &writableAdapter{
		mockAdapter: mockAdapter{id: "test-react", priority: 100, kinds: []types.WebxKind{types.KindThread}, hostname: "example.com"},
		reactResult: &types.NormalizedWriteResult{
			Success: true,
			Action:  "react",
			Message: "Liked",
			Backend: "mock_write",
		},
	}
	RegisterAdapter(adapter)

	env := RunReact("https://example.com/post/123", "like")
	if !env.OK {
		t.Fatalf("expected ok=true, got ok=false, error=%v", env.Error)
	}
	if env.Source.Adapter != "test-react" {
		t.Fatalf("expected adapter=test-react, got %s", env.Source.Adapter)
	}
}

func TestRunReply_WhenNoMatchingAdapter_ShouldReturnError(t *testing.T) {
	ResetRegistry()

	env := RunReply("https://unknown.example.com/post/123", "reply text")
	if env.OK {
		t.Fatal("expected ok=false for unmatched URL")
	}
	if env.Error.Code != string(types.ErrNoMatch) {
		t.Fatalf("expected NO_MATCH, got %s", env.Error.Code)
	}
}

func TestRunPost_WhenWriteError_ShouldReturnErrorEnvelope(t *testing.T) {
	ResetRegistry()
	adapter := &writableAdapter{
		mockAdapter: mockAdapter{id: "failing-write", priority: 100, kinds: []types.WebxKind{types.KindThread}, hostname: "example.com"},
		postErr:     types.NewWebxError(types.ErrRateLimited, "slow down"),
	}
	RegisterAdapter(adapter)

	env := RunPost("failing-write", "spam")
	if env.OK {
		t.Fatal("expected ok=false when write returns error")
	}
	if env.Error.Code != string(types.ErrRateLimited) {
		t.Fatalf("expected RATE_LIMITED, got %s", env.Error.Code)
	}
}
