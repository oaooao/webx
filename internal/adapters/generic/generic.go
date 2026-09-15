// Package generic provides a catch-all adapter for arbitrary web articles.
// It extracts with go-defuddle over a direct fetch first. When that fails or
// the content does not look like an article (see quality.go), it renders the
// page in the user's ego lite browser, then tries the r.jina.ai reader. If no
// attempt yields an article it returns an honest error instead of the wall.
package generic

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

const adapterID = "generic-article"

type genericAdapter struct{}

// New returns a new generic article adapter.
func New() types.ExtractableAdapter {
	return &genericAdapter{}
}

func (a *genericAdapter) ID() string { return adapterID }

// Priority 10: lowest priority so specific adapters take precedence.
func (a *genericAdapter) Priority() int { return 10 }

func (a *genericAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindArticle}
}

// Match accepts any URL with an http/https scheme. Since Priority is 10,
// specific adapters will match first.
func (a *genericAdapter) Match(ctx types.MatchContext) bool {
	scheme := ctx.URL.Scheme
	return scheme == "http" || scheme == "https"
}

// Seams for tests.
var (
	runDefuddle    = backends.RunDefuddle
	readViaEgoLite = backends.ReadPageViaEgoLite
	fetchViaJina   = backends.FetchViaJina
)

// article is a read that passed the content check.
type article struct {
	res           *backends.DefuddleResult
	backend       string
	fallbackDepth int
}

// attempt records one backend's outcome for the final error.
type attempt struct {
	backend string
	err     error
	issue   *contentIssue
}

type strategy struct {
	backend string
	fetch   func(rawURL string) (*backends.DefuddleResult, error)
}

var strategies = []strategy{
	{backend: "defuddle", fetch: func(u string) (*backends.DefuddleResult, error) { return runDefuddle(u) }},
	{backend: egolite.BackendID, fetch: func(u string) (*backends.DefuddleResult, error) { return readViaEgoLite(u) }},
	{backend: "jina", fetch: func(u string) (*backends.DefuddleResult, error) {
		j, err := fetchViaJina(u)
		if err != nil {
			return nil, err
		}
		return &backends.DefuddleResult{Title: j.Title, Markdown: j.Markdown}, nil
	}},
}

// read runs the strategies in order and returns the first article that
// passes assessContent.
func (a *genericAdapter) read(ctx types.RunContext) (*article, error) {
	rawURL := ctx.URL.String()
	var attempts []attempt
	for depth, s := range strategies {
		step := "backend." + s.backend
		res, err := s.fetch(rawURL)
		if err != nil {
			ctx.Trace.Push(types.TraceEvent{
				Step:    step,
				Reason:  types.TraceReasonFromError(err),
				Adapter: adapterID,
				Backend: s.backend,
				Detail:  err.Error(),
			})
			attempts = append(attempts, attempt{backend: s.backend, err: err})
			continue
		}
		if issue := assessContent(res.Title, res.Markdown); issue != nil {
			ctx.Trace.Push(types.TraceEvent{
				Step:    step,
				Reason:  traceReasonForCode(issue.Code),
				Adapter: adapterID,
				Backend: s.backend,
				Detail:  "content check failed: " + issue.Reason,
			})
			attempts = append(attempts, attempt{backend: s.backend, issue: issue})
			continue
		}
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceRouteMatch,
			Adapter: adapterID,
			Backend: s.backend,
			Detail:  s.backend + " succeeded",
		})
		return &article{res: res, backend: s.backend, fallbackDepth: depth}, nil
	}
	return nil, readFailure(ctx.URL.Hostname(), attempts)
}

// readFailure picks the most telling attempt and lists the rest.
//
// A content check on what the real browser rendered is the strongest
// evidence, then the direct fetch, then jina. Without any content check, an
// actionable code (ANTI_BOT, LOGIN_REQUIRED) from any backend wins, else the
// direct fetch's error.
func readFailure(host string, attempts []attempt) error {
	if len(attempts) == 0 {
		return types.NewWebxError(types.ErrBackendFailed, "no backend attempted")
	}
	primary := -1
	for _, b := range []string{egolite.BackendID, "defuddle", "jina"} {
		for i, at := range attempts {
			if primary < 0 && at.backend == b && at.issue != nil {
				primary = i
			}
		}
	}
	if primary < 0 {
		for i, at := range attempts {
			if c := errCode(at.err); primary < 0 && (c == types.ErrAntiBot || c == types.ErrLoginRequired) && at.backend != egolite.BackendID {
				primary = i
			}
		}
	}
	if primary < 0 {
		primary = 0
	}

	p := attempts[primary]
	var code types.ErrorCode
	var reason string
	if p.issue != nil {
		code, reason = p.issue.Code, p.issue.Reason
	} else {
		code, reason = errCode(p.err), errMessage(p.err)
	}

	var tried []string
	for _, at := range attempts {
		if at.issue != nil {
			tried = append(tried, fmt.Sprintf("%s: %s %s", at.backend, at.issue.Code, at.issue.Reason))
		} else {
			tried = append(tried, fmt.Sprintf("%s: %s", at.backend, at.err.Error()))
		}
	}
	return types.NewWebxError(code, fmt.Sprintf("%s: %s. %s Tried %s.",
		host, reason, suggestion(code, host), strings.Join(tried, "; ")))
}

func errCode(err error) types.ErrorCode {
	var wxErr *types.WebxError
	if errors.As(err, &wxErr) {
		return wxErr.Code
	}
	return types.ErrBackendFailed
}

func errMessage(err error) string {
	var wxErr *types.WebxError
	if errors.As(err, &wxErr) {
		return wxErr.Message
	}
	return err.Error()
}

func traceReasonForCode(code types.ErrorCode) types.TraceReason {
	return types.TraceReasonFromError(types.NewWebxError(code, ""))
}

func (a *genericAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	art, err := a.read(ctx)
	if err != nil {
		return nil, err
	}
	return &types.NormalizedReadResult{
		Title:         ptrOf(art.res.Title),
		Markdown:      ptrOf(art.res.Markdown),
		HTML:          ptrOf(art.res.HTML),
		Backend:       art.backend,
		FallbackDepth: art.fallbackDepth,
	}, nil
}

// ArticleExtractData is the structured metadata returned by Extract.
type ArticleExtractData struct {
	Title       string `json:"title,omitempty"`
	Author      string `json:"author,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	Description string `json:"description,omitempty"`
	Published   string `json:"published,omitempty"`
}

func (a *genericAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	art, err := a.read(ctx)
	if err != nil {
		return nil, err
	}
	return &types.NormalizedExtractResult{
		Title:    ptrOf(art.res.Title),
		Markdown: ptrOf(art.res.Markdown),
		HTML:     ptrOf(art.res.HTML),
		Data: ArticleExtractData{
			Title:       art.res.Title,
			Author:      art.res.Author,
			SiteName:    art.res.SiteName,
			Description: art.res.Description,
			Published:   art.res.Published,
		},
		Backend:       art.backend,
		FallbackDepth: art.fallbackDepth,
	}, nil
}

func ptrOf(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
