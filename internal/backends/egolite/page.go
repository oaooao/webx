package egolite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/oaooao/webx/internal/types"
)

// PageJob opens one URL in ego lite as a real navigation and reports the
// rendered page. It is the slow path (about 3 s): use it when no same-origin
// API exists, e.g. reading an article that only renders in a browser, or
// following a client-side redirect.
type PageJob struct {
	// Site is a human-readable site name used in error messages.
	Site string `json:"site"`
	URL  string `json:"url"`
	// WaitURLPattern, when set, is a JavaScript regular expression the
	// page's location.href must match before the page is read. A timeout
	// while waiting is not an error; the result then carries the URL the
	// page ended on.
	WaitURLPattern string `json:"wait_url_pattern,omitempty"`
	// WaitMs bounds the wait for WaitURLPattern, or for the body text to
	// settle when no pattern is set. Zero means 8 s.
	WaitMs int `json:"wait_ms,omitempty"`
	// WithHTML asks for document.documentElement.outerHTML.
	WithHTML bool `json:"with_html,omitempty"`
}

// Page is what the browser saw after the navigation settled.
type Page struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	HTML  string `json:"html,omitempty"`
	// TextLength is document.body.innerText.length.
	TextLength int    `json:"text_length"`
	Timing     Timing `json:"timing"`
}

type pageOutput struct {
	OK     bool         `json:"ok"`
	Error  *scriptError `json:"error,omitempty"`
	Result *Page        `json:"result,omitempty"`
}

// RenderPage runs job with the default runner and timeout.
func RenderPage(job PageJob) (*Page, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	return RenderPageWith(ctx, DefaultRunner, job)
}

// RenderPageWith is RenderPage with an explicit context and runner.
func RenderPageWith(ctx context.Context, runner Runner, job PageJob) (*Page, error) {
	script, err := BuildPageScript(job)
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "egolite: "+err.Error())
	}
	stdout, stderr, runErr := runner(ctx, script)
	jobInfo := Job{Site: job.Site, Origin: job.URL}

	line, lineErr := lastResultLine(stderr, stdout)
	if lineErr != nil {
		return nil, classifyProcessFailure(ctx, jobInfo, stdout, stderr, runErr, lineErr)
	}
	var out pageOutput
	if err := json.Unmarshal(line, &out); err != nil {
		return nil, classifyProcessFailure(ctx, jobInfo, stdout, stderr, runErr, fmt.Errorf("invalid result JSON: %w", err))
	}
	if !out.OK {
		return nil, scriptFailure(jobInfo, out.Error)
	}
	if out.Result == nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "egolite: page script reported ok without result")
	}
	return out.Result, nil
}

// BuildPageScript renders the page script with job embedded as JSON.
func BuildPageScript(job PageJob) (string, error) {
	if job.URL == "" {
		return "", errors.New("job.URL is required")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	return strings.Replace(pageScriptTemplate, "__JOB__", string(payload), 1), nil
}

// pageScriptTemplate: goto waits for domcontentloaded only (see
// scriptTemplate); the settle wait covers client-rendered content and
// client-side redirects without depending on the "load" event.
const pageScriptTemplate = `const job = __JOB__;
const MARK = "` + resultMarker + `";
const emit = (o) => console.log(MARK + JSON.stringify(o));
const t0 = Date.now();
let page = null;
try {
  const task = await taskSpace("` + SpaceName + `");
  page = await task.newPage();
  const t1 = Date.now();
  try {
    await page.goto(job.url, { waitUntil: "domcontentloaded", timeout: 20000 });
  } catch (e) {
    emit({ ok: false, error: { kind: "goto", message: String(e && e.message || e) } });
    throw null;
  }
  const t2 = Date.now();
  const waitMs = job.wait_ms || 8000;
  try {
    if (job.wait_url_pattern) {
      await page.waitForFunction((p) => new RegExp(p).test(location.href), job.wait_url_pattern, { timeout: waitMs });
    } else {
      // Settle: body text stops growing for 500 ms, or waitMs passes.
      await page.waitForFunction(() => {
        const n = (document.body && document.body.innerText || "").length;
        const w = window.__webxSettle || (window.__webxSettle = { n: -1, t: Date.now() });
        if (n !== w.n) { w.n = n; w.t = Date.now(); return false; }
        return n > 0 && Date.now() - w.t >= 500;
      }, undefined, { timeout: waitMs, polling: 100 });
    }
  } catch (_) {}
  const result = await page.evaluate((withHTML) => ({
    url: location.href,
    title: document.title || "",
    html: withHTML ? document.documentElement.outerHTML : "",
    text_length: (document.body && document.body.innerText || "").length,
  }), !!job.with_html);
  const t3 = Date.now();
  result.timing = { page_ms: t1 - t0, goto_ms: t2 - t1, fetch_ms: t3 - t2 };
  emit({ ok: true, result });
} catch (e) {
  if (e !== null) emit({ ok: false, error: { kind: "script", message: String(e && e.stack || e) } });
} finally {
  if (page) { try { await page.close(); } catch (_) {} }
}
`
