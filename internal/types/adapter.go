package types

import "net/url"

type MatchContext struct {
	URL           *url.URL
	RequestedKind *WebxKind // nil means "any"
}

type RunContext struct {
	MatchContext
	Trace *TraceBuffer
}

type NormalizedReadResult struct {
	Title    *string
	Markdown *string
	HTML     *string
	// Data is optional structured content (e.g. a thread's replies) placed
	// in the envelope's data field. Nil for plain article reads.
	Data          any
	Backend       string
	FallbackDepth int
}

type NormalizedExtractResult struct {
	Title         *string
	Markdown      *string
	HTML          *string
	Data          any
	Backend       string
	FallbackDepth int
}

type Adapter interface {
	ID() string
	Priority() int
	Kinds() []WebxKind
	Match(ctx MatchContext) bool
	Read(ctx RunContext) (*NormalizedReadResult, error)
}

// ExtractableAdapter is an optional interface for adapters that support extract().
type ExtractableAdapter interface {
	Adapter
	Extract(ctx RunContext) (*NormalizedExtractResult, error)
}
