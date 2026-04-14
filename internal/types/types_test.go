package types

import (
	"errors"
	"testing"
)

// --- WordCount ---

func TestWordCount_WhenNil_ShouldReturnZero(t *testing.T) {
	if got := WordCount(nil); got != 0 {
		t.Errorf("WordCount(nil) = %d, want 0", got)
	}
}

func TestWordCount_WhenEmpty_ShouldReturnZero(t *testing.T) {
	s := ""
	if got := WordCount(&s); got != 0 {
		t.Errorf("WordCount(\"\") = %d, want 0", got)
	}
}

func TestWordCount_WhenMultipleWords_ShouldCountCorrectly(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"hello", 1},
		{"hello world", 2},
		{"  hello   world  ", 2},
		{"one\ttwo\nthree\rfour", 4},
		{"a b c d e f", 6},
	}
	for _, tc := range cases {
		s := tc.input
		got := WordCount(&s)
		if got != tc.want {
			t.Errorf("WordCount(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// --- WebxKind.IsValid ---

func TestWebxKind_IsValid_WhenKnownKind_ShouldReturnTrue(t *testing.T) {
	for _, k := range ValidKinds {
		if !k.IsValid() {
			t.Errorf("expected %q to be valid", k)
		}
	}
}

func TestWebxKind_IsValid_WhenUnknownKind_ShouldReturnFalse(t *testing.T) {
	invalid := []WebxKind{"invalid", "unknown", "", "Article"}
	for _, k := range invalid {
		if k.IsValid() {
			t.Errorf("expected %q to be invalid", k)
		}
	}
}

// --- StringPtr ---

func TestStringPtr_ShouldReturnNonNilPointer(t *testing.T) {
	s := "hello"
	p := StringPtr(s)
	if p == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *p != s {
		t.Errorf("got %q, want %q", *p, s)
	}
}

func TestStringPtr_WhenEmpty_ShouldStillReturnPointer(t *testing.T) {
	p := StringPtr("")
	if p == nil {
		t.Fatal("expected non-nil pointer even for empty string")
	}
	if *p != "" {
		t.Errorf("got %q, want empty string", *p)
	}
}

// --- WebxError ---

func TestWebxError_Error_ShouldFormatCodeAndMessage(t *testing.T) {
	err := NewWebxError(ErrAntiBot, "blocked")
	got := err.Error()
	want := "[ANTI_BOT] blocked"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNewNoMatchError_ShouldContainURL(t *testing.T) {
	err := NewNoMatchError("https://example.com")
	if err.Code != ErrNoMatch {
		t.Errorf("code: got %q, want %q", err.Code, ErrNoMatch)
	}
	if err.Message == "" {
		t.Error("message should not be empty")
	}
}

func TestNewNotImplementedError_ShouldUseBackendUnavailable(t *testing.T) {
	err := NewNotImplementedError("search")
	if err.Code != ErrBackendUnavailable {
		t.Errorf("code: got %q, want %q", err.Code, ErrBackendUnavailable)
	}
}

// --- TraceReasonFromError ---

func TestTraceReasonFromError_WhenWebxError_ShouldMapCorrectly(t *testing.T) {
	cases := []struct {
		code ErrorCode
		want TraceReason
	}{
		{ErrContentEmpty, TraceEmptyContent},
		{ErrAntiBot, TraceAntiBot},
		{ErrRateLimited, TraceRateLimited},
		{ErrLoginRequired, TraceLoginRequired},
		{ErrTLSBlocked, TraceBackendFailed},
		{ErrPartialContent, TracePartialContent},
		{ErrFetchFailed, TraceBackendFailed},
	}
	for _, tc := range cases {
		err := NewWebxError(tc.code, "test")
		got := TraceReasonFromError(err)
		if got != tc.want {
			t.Errorf("TraceReasonFromError(%s) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestTraceReasonFromError_WhenNonWebxError_ShouldReturnBackendFailed(t *testing.T) {
	got := TraceReasonFromError(errors.New("random error"))
	if got != TraceBackendFailed {
		t.Errorf("got %q, want %q", got, TraceBackendFailed)
	}
}

// --- MakeEnvelope ---

func TestMakeEnvelope_ShouldSetSchemaVersion(t *testing.T) {
	env := MakeEnvelope(EnvelopeInput{
		OK:      true,
		Kind:    KindArticle,
		URL:     "https://example.com/test",
		Adapter: "test",
		Backend: "test",
	})
	if env.SchemaVersion != "1" {
		t.Errorf("SchemaVersion: got %q, want %q", env.SchemaVersion, "1")
	}
}

func TestMakeEnvelope_ShouldParseDomain(t *testing.T) {
	env := MakeEnvelope(EnvelopeInput{
		URL: "https://www.example.com/path",
	})
	if env.Source.Domain != "www.example.com" {
		t.Errorf("Domain: got %q, want %q", env.Source.Domain, "www.example.com")
	}
}

func TestMakeEnvelope_WhenNilTrace_ShouldReturnEmptySlice(t *testing.T) {
	env := MakeEnvelope(EnvelopeInput{
		URL:   "https://example.com",
		Trace: nil,
	})
	if env.Trace == nil {
		t.Fatal("Trace should be non-nil empty slice, not nil")
	}
	if len(env.Trace) != 0 {
		t.Errorf("expected empty trace, got %d events", len(env.Trace))
	}
}

func TestMakeEnvelope_ShouldSetFetchedAt(t *testing.T) {
	env := MakeEnvelope(EnvelopeInput{
		URL: "https://example.com",
	})
	if env.Meta.FetchedAt == "" {
		t.Error("FetchedAt should not be empty")
	}
}

// --- TraceBuffer ---

func TestTraceBuffer_PushAndAll(t *testing.T) {
	tb := NewTraceBuffer()
	tb.Push(TraceEvent{Step: "one", Reason: TraceRouteMatch})
	tb.Push(TraceEvent{Step: "two", Reason: TraceBackendFailed})

	events := tb.All()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Step != "one" {
		t.Errorf("first event step: got %q", events[0].Step)
	}
}

func TestTraceBuffer_All_ShouldReturnCopy(t *testing.T) {
	tb := NewTraceBuffer()
	tb.Push(TraceEvent{Step: "one"})

	events := tb.All()
	events[0].Step = "modified"

	original := tb.All()
	if original[0].Step != "one" {
		t.Error("All() should return a copy, not a reference to internal state")
	}
}
