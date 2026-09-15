// Package egolite is a transport that executes HTTP requests inside the
// user's ego lite browser (https://lite.ego.app/) through the `ego-browser
// nodejs` CLI. Requests run as page.fetch() on a same-origin page, so they
// carry the browser's cookies, TLS stack and login state. No tokens are
// exported from the browser.
//
// Adapters use it as a fallback when their direct HTTP backend is rejected
// (401/403). The site-specific request building and response parsing stay in
// the adapter's backend; this package only moves bytes.
package egolite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/types"
)

// BackendID is the value adapters put in source.backend when a result came
// through this transport.
const BackendID = "egolite"

// SpaceName is the fixed ego lite TaskSpace every webx run reuses. Each run
// opens its own Page in the space and closes it when done, so concurrent runs
// do not navigate each other's Page.
const SpaceName = "webx"

// resultMarker prefixes the single output line that carries the run result.
// Anything else in the output (ego-browser notices, stray logs) is ignored.
const resultMarker = "__WEBX_EGOLITE_RESULT__"

// DefaultTimeout bounds one `ego-browser nodejs` invocation. A browser that is
// not running takes about 30 s to fail its bootstrap connect.
const DefaultTimeout = 60 * time.Second

// Request is one HTTP request executed via page.fetch in the browser.
// Header values may contain {{cookie:<name>}} placeholders, substituted inside
// the browser from the site's cookie jar (e.g. Twitter's x-csrf-token = ct0).
type Request struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// Response is the page.fetch result for one Request.
type Response struct {
	Status int    `json:"status"`
	URL    string `json:"url"`
	Body   string `json:"body"`
	// Error is set when fetch itself threw (network, CORS, timeout).
	Error string `json:"error,omitempty"`
}

// Job describes one browser round: open Origin, check login cookies, run
// Requests in order.
type Job struct {
	// Site is a human-readable site name used in error messages ("x.com").
	Site string `json:"site"`
	// Origin is a cheap same-origin URL to open before fetching, e.g.
	// https://x.com/robots.txt. page.fetch obeys CORS, so it must share the
	// origin of the request URLs.
	Origin string `json:"origin"`
	// LoginCookies must all be present in the browser's cookie jar for
	// Origin; otherwise the run fails with LOGIN_REQUIRED before fetching.
	// Leave empty for sites that serve content without login.
	LoginCookies []string  `json:"login_cookies,omitempty"`
	Requests     []Request `json:"requests"`
	// FetchTimeoutMs bounds each page.fetch. Zero means 20 s.
	FetchTimeoutMs int `json:"fetch_timeout_ms,omitempty"`
}

// Result is what the browser script reports back.
type Result struct {
	Responses []Response `json:"responses"`
	LoggedIn  bool       `json:"logged_in"`
	Timing    Timing     `json:"timing"`
}

// Timing records where the time inside the browser went, in milliseconds.
type Timing struct {
	PageMs  int64 `json:"page_ms"`
	GotoMs  int64 `json:"goto_ms"`
	FetchMs int64 `json:"fetch_ms"`
}

// scriptOutput is the JSON payload after resultMarker.
type scriptOutput struct {
	OK     bool         `json:"ok"`
	Error  *scriptError `json:"error,omitempty"`
	Result *Result      `json:"result,omitempty"`
}

type scriptError struct {
	Kind    string `json:"kind"` // "login" | "goto" | "script"
	Message string `json:"message"`
}

// Runner executes a script with `ego-browser nodejs` and returns stdout,
// stderr and the process error. Replaced in tests.
type Runner func(ctx context.Context, script string) (stdout, stderr []byte, err error)

// DefaultRunner is the Runner used by Run. Tests may swap it.
var DefaultRunner Runner = execRunner

// Run executes job in ego lite and returns the fetch responses. It maps every
// failure to a WebxError: a missing CLI, an unreachable browser or missing
// login cookies are LOGIN_REQUIRED with a hint to sign in inside ego lite.
func Run(job Job) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	return RunWith(ctx, DefaultRunner, job)
}

// RunWith is Run with an explicit context and runner.
func RunWith(ctx context.Context, runner Runner, job Job) (*Result, error) {
	script, err := BuildScript(job)
	if err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "egolite: "+err.Error())
	}

	stdout, stderr, runErr := runner(ctx, script)
	// ego-browser forwards the script's console.log to stderr (verified
	// 2026-09-14: stdout stays empty when redirected), so scan both streams.
	out, parseErr := parseOutput(stderr, stdout)

	if out == nil {
		// The script never reported: the CLI is missing, the browser is not
		// reachable, or the runtime crashed before our try/catch.
		return nil, classifyProcessFailure(ctx, job, stdout, stderr, runErr, parseErr)
	}
	if !out.OK {
		return nil, scriptFailure(job, out.Error)
	}
	if out.Result == nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "egolite: script reported ok without result")
	}
	return out.Result, nil
}

// ErrorForStatus maps a non-2xx status of a browser-side fetch to a WebxError.
// A 401/403 inside the logged-in browser means the session itself is gone.
func ErrorForStatus(site string, status int) error {
	switch {
	case status == 401 || status == 403:
		return types.NewWebxError(types.ErrLoginRequired, fmt.Sprintf(
			"%s rejected the ego lite session (HTTP %d). Open ego lite, sign in to %s, then retry.",
			site, status, site))
	case status == 429:
		return types.NewWebxError(types.ErrRateLimited, fmt.Sprintf("%s rate limited the ego lite session (HTTP 429)", site))
	case status == 0:
		return types.NewWebxError(types.ErrFetchFailed, site+" request failed inside ego lite (no HTTP status)")
	default:
		return types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf("%s returned HTTP %d via ego lite", site, status))
	}
}

// BuildScript renders the fixed browser script with job embedded as a JSON
// literal. JSON is valid JavaScript, so no escaping beyond json.Marshal is
// needed; json.Marshal also escapes <, > and & which keeps the literal inert.
func BuildScript(job Job) (string, error) {
	if job.Origin == "" {
		return "", errors.New("job.Origin is required")
	}
	if len(job.Requests) == 0 {
		return "", errors.New("job.Requests is empty")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	return strings.Replace(scriptTemplate, "__JOB__", string(payload), 1), nil
}

// scriptTemplate runs inside ego-browser's embedded Node runtime. Lessons
// baked in from testing against ego-browser 0.5:
//   - goto waits for domcontentloaded; the default "load" times out on Reddit.
//   - cookies come from CDP Network.getCookies, which also sees httpOnly
//     cookies such as Twitter's auth_token (document.cookie does not).
//   - the Page is created per run and closed in finally; the space persists.
//   - the process cwd is "/" (read-only), so nothing is written to disk.
const scriptTemplate = `const job = __JOB__;
const MARK = "` + resultMarker + `";
const emit = (o) => console.log(MARK + JSON.stringify(o));
const t0 = Date.now();
let page = null;
try {
  const task = await taskSpace("` + SpaceName + `");
  page = await task.newPage();
  const t1 = Date.now();
  try {
    await page.goto(job.origin, { waitUntil: "domcontentloaded", timeout: 20000 });
  } catch (e) {
    emit({ ok: false, error: { kind: "goto", message: String(e && e.message || e) } });
    throw null;
  }
  const t2 = Date.now();
  const jar = (await page.cdp("Network.getCookies", { urls: [job.origin] })).cookies || [];
  const cookie = (name) => (jar.find((c) => c.name === name) || {}).value;
  const missing = (job.login_cookies || []).filter((n) => !cookie(n));
  if (missing.length > 0) {
    emit({ ok: false, error: { kind: "login", message: "missing cookies: " + missing.join(",") } });
    throw null;
  }
  const fill = (v) => String(v).replace(/\{\{cookie:([^}]+)\}\}/g, (_, n) => cookie(n) || "");
  const responses = [];
  for (const r of job.requests) {
    const headers = {};
    for (const [k, v] of Object.entries(r.headers || {})) headers[k] = fill(v);
    const opts = { method: r.method || "GET", headers, timeout: job.fetch_timeout_ms || 20000 };
    if (r.body) opts.body = r.body;
    try {
      const res = await page.fetch(r.url, opts);
      responses.push({ status: res.status, url: res.url, body: res.body || "" });
    } catch (e) {
      responses.push({ status: 0, url: r.url, body: "", error: String(e && e.message || e) });
    }
  }
  const t3 = Date.now();
  const loggedIn = (job.login_cookies || []).length > 0;
  emit({ ok: true, result: { responses, logged_in: loggedIn, timing: { page_ms: t1 - t0, goto_ms: t2 - t1, fetch_ms: t3 - t2 } } });
} catch (e) {
  if (e !== null) emit({ ok: false, error: { kind: "script", message: String(e && e.stack || e) } });
} finally {
  if (page) { try { await page.close(); } catch (_) {} }
}
`

func parseOutput(streams ...[]byte) (*scriptOutput, error) {
	line, err := lastResultLine(streams...)
	if err != nil {
		return nil, err
	}
	var out scriptOutput
	if err := json.Unmarshal(line, &out); err != nil {
		return nil, fmt.Errorf("invalid result JSON: %w", err)
	}
	return &out, nil
}

// lastResultLine returns the JSON payload after the last result marker.
func lastResultLine(streams ...[]byte) ([]byte, error) {
	var last []byte
	for _, stream := range streams {
		for _, line := range bytes.Split(stream, []byte("\n")) {
			if i := bytes.Index(line, []byte(resultMarker)); i >= 0 {
				last = line[i+len(resultMarker):]
			}
		}
	}
	if last == nil {
		return nil, errors.New("no result line in ego-browser output")
	}
	return bytes.TrimSpace(last), nil
}

func loginHint(job Job) string {
	return fmt.Sprintf("Open ego lite, sign in to %s, then retry.", job.Site)
}

func scriptFailure(job Job, se *scriptError) error {
	if se == nil {
		return types.NewWebxError(types.ErrBackendFailed, "egolite: script failed without detail")
	}
	switch se.Kind {
	case "login":
		return types.NewWebxError(types.ErrLoginRequired, fmt.Sprintf(
			"Not signed in to %s in ego lite (%s). %s", job.Site, se.Message, loginHint(job)))
	case "goto":
		return types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf(
			"ego lite could not open %s: %s", job.Origin, se.Message))
	default:
		return types.NewWebxError(types.ErrBackendFailed, "egolite script error: "+se.Message)
	}
}

// exitConnectFailed is the ego-browser exit code when it cannot reach the
// ego lite app (verified 2026-09-14 with --ego-server-name=<unknown>).
const exitConnectFailed = 5

func classifyProcessFailure(ctx context.Context, job Job, stdout, stderr []byte, runErr, parseErr error) error {
	detail := strings.TrimSpace(string(stderr) + "\n" + string(stdout))
	if len(detail) > 400 {
		detail = detail[:400] + "..."
	}

	var nf *cliNotFoundError
	if errors.As(runErr, &nf) {
		return types.NewWebxError(types.ErrLoginRequired, fmt.Sprintf(
			"ego-browser CLI not found (%s). Install ego lite from https://lite.ego.app/, sign in to %s, then retry.",
			nf.detail, job.Site))
	}

	var exitErr *exec.ExitError
	connectFailed := strings.Contains(detail, "Failed to connect") ||
		(errors.As(runErr, &exitErr) && exitErr.ExitCode() == exitConnectFailed)
	if connectFailed {
		return types.NewWebxError(types.ErrLoginRequired, fmt.Sprintf(
			"ego lite is not running or unreachable. %s Detail: %s", loginHint(job), firstLine(detail)))
	}

	if ctx.Err() != nil {
		return types.NewWebxError(types.ErrFetchTimeout, fmt.Sprintf(
			"ego lite did not answer within %s. Is ego lite open? Detail: %s", DefaultTimeout, firstLine(detail)))
	}

	msg := "egolite: " + parseErr.Error()
	if runErr != nil {
		msg = "egolite: ego-browser failed: " + runErr.Error()
	}
	if detail != "" {
		msg += ": " + detail
	}
	return types.NewWebxError(types.ErrBackendFailed, msg)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

type cliNotFoundError struct{ detail string }

func (e *cliNotFoundError) Error() string { return "ego-browser not found: " + e.detail }

// LookupCLI resolves the ego-browser executable: $WEBX_EGO_BROWSER, then PATH,
// then ~/.local/bin/ego-browser (where ego lite onboarding installs it; that
// directory is often missing from PATH in non-login shells and launchd jobs).
func LookupCLI() (string, error) {
	if p := os.Getenv("WEBX_EGO_BROWSER"); p != "" {
		return p, nil
	}
	if p, err := exec.LookPath("ego-browser"); err == nil {
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".local", "bin", "ego-browser")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", &cliNotFoundError{detail: "not on PATH, not at ~/.local/bin/ego-browser, WEBX_EGO_BROWSER unset"}
}

func execRunner(ctx context.Context, script string) ([]byte, []byte, error) {
	cli, err := LookupCLI()
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, cli, "nodejs")
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
