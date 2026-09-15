package egolite

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

func TestBuildPageScript_EmbedsJob(t *testing.T) {
	script, err := BuildPageScript(PageJob{Site: "x.com", URL: "https://x.com/i/article/1", WaitURLPattern: `^https://x\.com/a`, WithHTML: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(script, `const job = {"site":"x.com","url":"https://x.com/i/article/1","wait_url_pattern":"^https://x\\.com/a","with_html":true};`) {
		t.Errorf("script does not start with the job literal:\n%s", script[:200])
	}
	if _, err := BuildPageScript(PageJob{}); err == nil {
		t.Error("empty URL must be rejected")
	}
}

func TestRenderPage_ParsesResult(t *testing.T) {
	out := resultMarker + `{"ok":true,"result":{"url":"https://x.com/a/article/5","title":"X","html":"<html></html>","text_length":12,"timing":{"page_ms":1,"goto_ms":2,"fetch_ms":3}}}`
	page, err := RenderPageWith(context.Background(), fixedRunner("", "notice\n"+out+"\n", nil), PageJob{Site: "x.com", URL: "https://x.com/i/article/1"})
	if err != nil {
		t.Fatal(err)
	}
	if page.URL != "https://x.com/a/article/5" || page.HTML != "<html></html>" || page.TextLength != 12 {
		t.Errorf("page = %+v", page)
	}
}

func TestRenderPage_GotoFailure_IsFetchFailed(t *testing.T) {
	out := resultMarker + `{"ok":false,"error":{"kind":"goto","message":"net::ERR_NAME_NOT_RESOLVED"}}`
	_, err := RenderPageWith(context.Background(), fixedRunner("", out, nil), PageJob{Site: "nowhere", URL: "https://nowhere.invalid/"})
	if code := codeOf(t, err); code != types.ErrFetchFailed {
		t.Errorf("code = %q", code)
	}
}

func TestRenderPage_BrowserNotRunning_IsLoginRequired(t *testing.T) {
	_, err := RenderPageWith(context.Background(), fixedRunner("", "Failed to connect to ego_cli bootstrap", errors.New("exit status 5")), PageJob{Site: "x.com", URL: "https://x.com/"})
	if code := codeOf(t, err); code != types.ErrLoginRequired {
		t.Errorf("code = %q (%v)", code, err)
	}
}
