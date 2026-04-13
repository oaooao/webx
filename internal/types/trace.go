package types

type TraceReason string

const (
	TraceRouteMatch         TraceReason = "ROUTE_MATCH"
	TraceRouteSkip          TraceReason = "ROUTE_SKIP"
	TraceBackendPlaceholder TraceReason = "BACKEND_PLACEHOLDER"
	TraceBackendFailed      TraceReason = "BACKEND_FAILED"
	TraceNoMatch            TraceReason = "NO_MATCH"
	TraceUnsupportedKind    TraceReason = "UNSUPPORTED_KIND"
	TraceLoginRequired      TraceReason = "LOGIN_REQUIRED"
	TraceDOMChanged         TraceReason = "DOM_CHANGED"
	TraceRateLimited        TraceReason = "RATE_LIMITED"
	TraceAntiBot            TraceReason = "ANTI_BOT"
	TraceEmptyContent       TraceReason = "EMPTY_CONTENT"
	TracePartialContent     TraceReason = "PARTIAL_CONTENT"
	TraceNotImplemented     TraceReason = "NOT_IMPLEMENTED"
)

type TraceEvent struct {
	Step    string      `json:"step"`
	Reason  TraceReason `json:"reason"`
	Adapter string      `json:"adapter,omitempty"`
	Backend string      `json:"backend,omitempty"`
	Detail  string      `json:"detail"`
}

type TraceBuffer struct {
	events []TraceEvent
}

func NewTraceBuffer() *TraceBuffer {
	return &TraceBuffer{}
}

func (t *TraceBuffer) Push(event TraceEvent) {
	t.events = append(t.events, event)
}

func (t *TraceBuffer) All() []TraceEvent {
	result := make([]TraceEvent, len(t.events))
	copy(result, t.events)
	return result
}
