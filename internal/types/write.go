package types

// WriteAction represents the type of write operation.
type WriteAction string

const (
	ActionPost  WriteAction = "post"
	ActionReply WriteAction = "reply"
	ActionReact WriteAction = "react"
)

// WriteContext holds the context for a write operation.
type WriteContext struct {
	Action    WriteAction
	Platform  string // adapter ID (used for Post routing)
	TargetURL string // target URL (used for Reply/React routing)
	Content   string // text content (Post/Reply)
	Reaction  string // reaction type (React): "like", "retweet", "upvote", "downvote"
	Trace     *TraceBuffer
}

// WritableAdapter is an optional interface for adapters that support write operations.
type WritableAdapter interface {
	Adapter
	Post(ctx WriteContext) (*NormalizedWriteResult, error)
	Reply(ctx WriteContext) (*NormalizedWriteResult, error)
	React(ctx WriteContext) (*NormalizedWriteResult, error)
}

// NormalizedWriteResult holds the output of a successful write operation.
type NormalizedWriteResult struct {
	Success     bool   `json:"success"`
	Action      string `json:"action"`                  // "post", "reply", "react"
	ResourceURL string `json:"resource_url,omitempty"`   // URL of the created resource
	Message     string `json:"message,omitempty"`        // human-readable result description
	Backend     string `json:"backend"`
}
