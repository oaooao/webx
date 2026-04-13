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
