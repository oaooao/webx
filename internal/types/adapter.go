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

// URLKindAdapter is implemented by adapters whose default kind depends on
// the URL (the twitter adapter reads /status/ links as threads and
// /i/article/ links as articles). The runner uses it when the caller did not
// request a kind; other adapters default to the first of Kinds().
type URLKindAdapter interface {
	KindFor(ctx MatchContext) WebxKind
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
