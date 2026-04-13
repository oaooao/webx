package types

import (
	"net/url"
	"time"
)

type WebxEnvelope struct {
	OK            bool            `json:"ok"`
	SchemaVersion string          `json:"schema_version"`
	Kind          WebxKind        `json:"kind"`
	Source        EnvelopeSource  `json:"source"`
	Content       EnvelopeContent `json:"content"`
	Data          any             `json:"data"`
	Meta          EnvelopeMeta    `json:"meta"`
	Trace         []TraceEvent    `json:"trace"`
	Error         *EnvelopeError  `json:"error"`
}

type EnvelopeSource struct {
	URL     string `json:"url"`
	Domain  string `json:"domain"`
	Adapter string `json:"adapter"`
	Backend string `json:"backend"`
}

type EnvelopeContent struct {
	Title    *string `json:"title"`
	Markdown *string `json:"markdown"`
	HTML     *string `json:"html"`
}

type EnvelopeMeta struct {
	FetchedAt     string `json:"fetched_at"`
	FallbackDepth int    `json:"fallback_depth"`
}

type EnvelopeInput struct {
	OK            bool
	Kind          WebxKind
	URL           string
	Adapter       string
	Backend       string
	Title         *string
	Markdown      *string
	HTML          *string
	Data          any
	Trace         []TraceEvent
	Error         *EnvelopeError
	FallbackDepth int
}

func StringPtr(s string) *string {
	return &s
}

func WordCount(s *string) int {
	if s == nil {
		return 0
	}
	count := 0
	inWord := false
	for _, r := range *s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

func MakeEnvelope(input EnvelopeInput) WebxEnvelope {
	parsed, _ := url.Parse(input.URL)
	domain := ""
	if parsed != nil {
		domain = parsed.Hostname()
	}

	trace := input.Trace
	if trace == nil {
		trace = []TraceEvent{}
	}

	return WebxEnvelope{
		OK:            input.OK,
		SchemaVersion: "1",
		Kind:          input.Kind,
		Source: EnvelopeSource{
			URL:     input.URL,
			Domain:  domain,
			Adapter: input.Adapter,
			Backend: input.Backend,
		},
		Content: EnvelopeContent{
			Title:    input.Title,
			Markdown: input.Markdown,
			HTML:     input.HTML,
		},
		Data: input.Data,
		Meta: EnvelopeMeta{
			FetchedAt:     time.Now().UTC().Format(time.RFC3339),
			FallbackDepth: input.FallbackDepth,
		},
		Trace: trace,
		Error: input.Error,
	}
}
