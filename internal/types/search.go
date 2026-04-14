package types

// SearchContext holds the context for a search operation.
// Unlike RunContext (which is URL-centric), SearchContext is query-centric
// and routes by platform ID rather than URL matching.
type SearchContext struct {
	Query    string
	Platform string // adapter ID, e.g. "twitter", "reddit"
	Options  SearchOptions
	Trace    *TraceBuffer
}

// SearchOptions holds optional parameters for search operations.
type SearchOptions struct {
	Limit int    // max results to return (default 20)
	Sort  string // "relevance", "recent", "top" (platform-specific mapping)
	Lang  string // language filter (optional)
}

// DefaultSearchOptions returns SearchOptions with sensible defaults.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Limit: 20,
		Sort:  "relevance",
	}
}

// SearchableAdapter is an optional interface for adapters that support search.
type SearchableAdapter interface {
	Adapter
	Search(ctx SearchContext) (*NormalizedSearchResult, error)
}

// NormalizedSearchResult holds the output of a successful search operation.
type NormalizedSearchResult struct {
	Items         []SearchResultItem `json:"items"`
	Query         string             `json:"query"`
	TotalEstimate int                `json:"total_estimate,omitempty"`
	Backend       string             `json:"backend"`
	HasMore       bool               `json:"has_more"`
}

// SearchResultItem represents a single search result.
type SearchResultItem struct {
	Title   string         `json:"title"`
	URL     string         `json:"url"`
	Snippet string         `json:"snippet,omitempty"`
	Author  string         `json:"author,omitempty"`
	Date    string         `json:"date,omitempty"`
	Score   float64        `json:"score,omitempty"`
	Kind    WebxKind       `json:"kind"`
	Meta    map[string]any `json:"meta,omitempty"`
}
