package egolite

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

func testJob() Job {
	return Job{
		Site:         "x.com",
		Origin:       "https://x.com/robots.txt",
		LoginCookies: []string{"auth_token", "ct0"},
		Requests: []Request{{
			URL:     "https://x.com/i/api/graphql/abc/TweetDetail?variables=%7B%7D",
			Headers: map[string]string{"x-csrf-token": "{{cookie:ct0}}"},
		}},
	}
}

func codeOf(t *testing.T, err error) types.ErrorCode {
	t.Helper()
	var wxErr *types.WebxError
	if !errors.As(err, &wxErr) {
		t.Fatalf("expected WebxError, got %T: %v", err, err)
	}
	return wxErr.Code
}

func fixedRunner(stdout, stderr string, err error) Runner {
	return func(context.Context, string) ([]byte, []byte, error) {
		return []byte(stdout), []byte(stderr), err
	}
}

func TestBuildScript_EmbedsJobAsJSONLiteral(t *testing.T) {
	job := testJob()
	job.Requests[0].Body = "</script> \" ` ${x}"
	script, err := BuildScript(job)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(script, "\n", 2)[0]
	literal := strings.TrimSuffix(strings.TrimPrefix(first, "const job = "), ";")
	var back Job
	if err := json.Unmarshal([]byte(literal), &back); err != nil {
		t.Fatalf("job literal is not valid JSON: %v\n%s", err, first)
	}
	if back.Requests[0].Body != job.Requests[0].Body {
		t.Errorf("body round-trip: got %q", back.Requests[0].Body)
	}
	for _, want := range []string{`taskSpace("webx")`, `waitUntil: "domcontentloaded"`, "Network.getCookies", "page.close()"} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q", want)
		}
	}
}

func TestBuildScript_RejectsEmptyJob(t *testing.T) {
	if _, err := BuildScript(Job{Origin: "https://x.com/"}); err == nil {
		t.Error("expected error for job without requests")
	}
	if _, err := BuildScript(Job{Requests: []Request{{URL: "https://x.com/"}}}); err == nil {
		t.Error("expected error for job without origin")
	}
}

func TestRun_ParsesResultFromStderr(t *testing.T) {
	// ego-browser forwards console.log to stderr.
	stderr := "[ego-browser:notice] something\n" + resultMarker +
		`{"ok":true,"result":{"responses":[{"status":200,"url":"u","body":"{\"data\":1}"}],"logged_in":true,"timing":{"page_ms":1,"goto_ms":2,"fetch_ms":3}}}` + "\n"
	res, err := RunWith(context.Background(), fixedRunner("", stderr, nil), testJob())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Responses) != 1 || res.Responses[0].Status != 200 || res.Responses[0].Body != `{"data":1}` {
		t.Errorf("unexpected responses: %+v", res.Responses)
	}
	if res.Timing.FetchMs != 3 {
		t.Errorf("timing not parsed: %+v", res.Timing)
	}
}

func TestRun_MissingLoginCookies_IsLoginRequired(t *testing.T) {
	stderr := resultMarker + `{"ok":false,"error":{"kind":"login","message":"missing cookies: auth_token"}}`
	_, err := RunWith(context.Background(), fixedRunner("", stderr, nil), testJob())
	if got := codeOf(t, err); got != types.ErrLoginRequired {
		t.Fatalf("code = %s, want LOGIN_REQUIRED", got)
	}
	if !strings.Contains(err.Error(), "sign in to x.com") {
		t.Errorf("message should tell the user to sign in inside ego lite: %v", err)
	}
}

func TestRun_BrowserNotRunning_IsLoginRequired(t *testing.T) {
	stderr := "Failed to connect to ego_cli bootstrap\nAgent note: launch ego lite.app manually."
	_, err := RunWith(context.Background(), fixedRunner("", stderr, errors.New("exit status 5")), testJob())
	if got := codeOf(t, err); got != types.ErrLoginRequired {
		t.Fatalf("code = %s, want LOGIN_REQUIRED", got)
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("message should say ego lite is not running: %v", err)
	}
}

func TestRun_CLIMissing_IsLoginRequired(t *testing.T) {
	runner := func(context.Context, string) ([]byte, []byte, error) {
		return nil, nil, &cliNotFoundError{detail: "not on PATH"}
	}
	_, err := RunWith(context.Background(), runner, testJob())
	if got := codeOf(t, err); got != types.ErrLoginRequired {
		t.Fatalf("code = %s, want LOGIN_REQUIRED", got)
	}
}

func TestRun_ScriptCrash_IsBackendFailed(t *testing.T) {
	var exitErr error = &exec.ExitError{}
	_, err := RunWith(context.Background(), fixedRunner("", "Error: boom at [eval]:1", exitErr), testJob())
	if got := codeOf(t, err); got != types.ErrBackendFailed {
		t.Fatalf("code = %s, want BACKEND_FAILED", got)
	}
}

func TestErrorForStatus(t *testing.T) {
	cases := map[int]types.ErrorCode{
		401: types.ErrLoginRequired,
		403: types.ErrLoginRequired,
		429: types.ErrRateLimited,
		500: types.ErrFetchFailed,
		0:   types.ErrFetchFailed,
	}
	for status, want := range cases {
		if got := codeOf(t, ErrorForStatus("x.com", status)); got != want {
			t.Errorf("status %d: code = %s, want %s", status, got, want)
		}
	}
}
