package backends

import (
	"fmt"

	defuddle "github.com/vaayne/go-defuddle"

	"github.com/oaooao/webx/internal/types"
)

// DefuddleResult holds the output of a successful defuddle parse.
type DefuddleResult struct {
	Title       string
	Markdown    string
	HTML        string
	Author      string
	SiteName    string
	Description string
	Published   string
}

// RunDefuddle fetches the given URL as HTML and extracts main content using
// the go-defuddle library (QuickJS-embedded Defuddle JS + html-to-markdown).
//
// The Parser is NOT safe for concurrent goroutines; we create one per call.
// For high-throughput use, callers should pool parsers externally.
func RunDefuddle(rawURL string) (*DefuddleResult, error) {
	html, err := FetchHTML(rawURL)
	if err != nil {
		return nil, err // already a *WebxError
	}

	parser, err := defuddle.NewParser()
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("defuddle: failed to create parser: %s", err))
	}
	defer parser.Close()

	boolTrue := true
	opts := &defuddle.Options{
		Markdown:    true,
		Standardize: &boolTrue,
	}

	result, err := parser.Parse(html, rawURL, opts)
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("defuddle: parse failed: %s", err))
	}

	// Treat near-empty content as failure: defuddle sometimes returns a
	// bare <body></body> shell when fed compressed/garbled HTML.
	if result.Markdown == "" && len(result.Content) < 50 {
		return nil, types.NewWebxError(types.ErrContentEmpty, "defuddle: extracted empty or near-empty content")
	}

	return &DefuddleResult{
		Title:       result.Title,
		Markdown:    result.Markdown,
		HTML:        result.Content,
		Author:      result.Author,
		SiteName:    result.Site,
		Description: result.Description,
		Published:   result.Published,
	}, nil
}
