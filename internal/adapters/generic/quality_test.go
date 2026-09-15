package generic

import (
	"errors"
	"strings"
	"testing"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

// realArticle is long enough and mostly prose.
var realArticle = strings.Repeat("Go 1.22 changes loop variables so each iteration has its own copy. ", 12)

func TestAssessContent(t *testing.T) {
	cases := []struct {
		name, title, markdown string
		want                  types.ErrorCode // "" means article
	}{
		{"real article", "Go 1.22", realArticle, ""},
		{"real article quoting a wall phrase", "Errors", realArticle + strings.Repeat("The server said parameter error. ", 50), ""},
		{"wechat parameter error fragment", "", "Parameter error\n\n: ， ， ， ， ， ，. Video Mini Program Like ，轻点两下取消赞 Wow ，轻点两下取消在看 Share", types.ErrContentEmpty},
		{"wechat anti-bot", "", "环境异常\n\n当前环境异常，完成验证后即可继续访问。\n\n[去验证](https://mp.weixin.qq.com/)", types.ErrAntiBot},
		{"zhihu 404 shell", "你似乎来到了没有知识存在的荒原 - 知乎", "[关注](https://www.zhihu.com/follow)\n[推荐](https://www.zhihu.com/)\n登录/注册\n\n5 秒后自动跳转至知乎首页", types.ErrContentEmpty},
		{"login shell", "知乎", "[首页](https://www.zhihu.com/) [发现](https://www.zhihu.com/explore) [等你来答](https://www.zhihu.com/question/waiting)\n\n验证码登录 密码登录\n\n获取短信验证码\n\n登录/注册", types.ErrLoginRequired},
		{"medium sign-in wall", "Member-only story", "Sign in to continue reading this story and get unlimited access.", types.ErrLoginRequired},
		{"navigation only", "Site", strings.Repeat("[Section name here](https://example.com/section) ", 20), types.ErrContentEmpty},
		{"too short", "Hi", "Hello world, short page.", types.ErrContentEmpty},
		{"empty", "", "![logo](https://example.com/logo.png)", types.ErrContentEmpty},
	}
	for _, c := range cases {
		issue := assessContent(c.title, c.markdown)
		got := types.ErrorCode("")
		if issue != nil {
			got = issue.Code
		}
		if got != c.want {
			t.Errorf("%s: got %q (%v), want %q", c.name, got, issue, c.want)
		}
	}
}

type fakeBackends struct {
	defuddle, egolite, jina func() (*backends.DefuddleResult, error)
	calls                   []string
}

func stubBackends(t *testing.T, f *fakeBackends) {
	t.Helper()
	oldD, oldE, oldJ := runDefuddle, readViaEgoLite, fetchViaJina
	t.Cleanup(func() { runDefuddle, readViaEgoLite, fetchViaJina = oldD, oldE, oldJ })
	runDefuddle = func(string) (*backends.DefuddleResult, error) {
		f.calls = append(f.calls, "defuddle")
		return f.defuddle()
	}
	readViaEgoLite = func(string) (*backends.DefuddleResult, error) {
		f.calls = append(f.calls, "egolite")
		return f.egolite()
	}
	fetchViaJina = func(string) (*backends.JinaResult, error) {
		f.calls = append(f.calls, "jina")
		r, err := f.jina()
		if err != nil {
			return nil, err
		}
		return &backends.JinaResult{Title: r.Title, Markdown: r.Markdown}, nil
	}
}

func ok(title, md string) func() (*backends.DefuddleResult, error) {
	return func() (*backends.DefuddleResult, error) {
		return &backends.DefuddleResult{Title: title, Markdown: md}, nil
	}
}

func fail(code types.ErrorCode, msg string) func() (*backends.DefuddleResult, error) {
	return func() (*backends.DefuddleResult, error) { return nil, types.NewWebxError(code, msg) }
}

func runCtx(t *testing.T, raw string) types.RunContext {
	return types.RunContext{MatchContext: types.MatchContext{URL: mustParseURL(t, raw)}, Trace: types.NewTraceBuffer()}
}

func TestRead_GoodDirectContent_DoesNotFallBack(t *testing.T) {
	f := &fakeBackends{defuddle: ok("Go", realArticle)}
	stubBackends(t, f)
	res, err := New().Read(runCtx(t, "https://go.dev/blog/go1.22"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "defuddle" || strings.Join(f.calls, ",") != "defuddle" {
		t.Errorf("backend=%q calls=%v", res.Backend, f.calls)
	}
}

func TestRead_WallFromDirect_FallsBackToEgoLite(t *testing.T) {
	f := &fakeBackends{
		defuddle: ok("", "Parameter error. 轻点两下取消赞"),
		egolite:  ok("Real title", realArticle),
	}
	stubBackends(t, f)
	res, err := New().Read(runCtx(t, "https://mp.weixin.qq.com/s/abc"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "egolite" || res.FallbackDepth != 1 || *res.Title != "Real title" {
		t.Errorf("backend=%q depth=%d title=%v", res.Backend, res.FallbackDepth, res.Title)
	}
}

func TestRead_AllWalls_ReturnsHonestError(t *testing.T) {
	f := &fakeBackends{
		defuddle: fail(types.ErrFetchFailed, "HTTP 403"),
		egolite:  ok("知乎", "[首页](https://www.zhihu.com/) 验证码登录 密码登录 登录/注册"),
		jina:     ok("知乎", "Parameter error"),
	}
	stubBackends(t, f)
	_, err := New().Read(runCtx(t, "https://zhuanlan.zhihu.com/p/42"))
	var wxErr *types.WebxError
	if !errors.As(err, &wxErr) {
		t.Fatalf("want WebxError, got %v", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("code = %q, want LOGIN_REQUIRED from the browser's view: %s", wxErr.Code, wxErr.Message)
	}
	for _, want := range []string{"zhuanlan.zhihu.com", "Sign in to zhuanlan.zhihu.com in ego lite", "defuddle:", "egolite:", "jina:"} {
		if !strings.Contains(wxErr.Message, want) {
			t.Errorf("message missing %q: %s", want, wxErr.Message)
		}
	}
}

func TestRead_EgoLiteUnavailable_UsesDirectWallVerdict(t *testing.T) {
	f := &fakeBackends{
		defuddle: ok("", "环境异常 完成验证后即可继续访问"),
		egolite:  fail(types.ErrLoginRequired, "ego lite is not running"),
		jina:     fail(types.ErrFetchTimeout, "timeout"),
	}
	stubBackends(t, f)
	_, err := New().Read(runCtx(t, "https://mp.weixin.qq.com/s/abc"))
	var wxErr *types.WebxError
	if !errors.As(err, &wxErr) || wxErr.Code != types.ErrAntiBot {
		t.Fatalf("want ANTI_BOT, got %v", err)
	}
}

func TestExtract_UsesSameChain(t *testing.T) {
	f := &fakeBackends{
		defuddle: fail(types.ErrFetchFailed, "HTTP 403"),
		egolite:  ok("Title", realArticle),
	}
	stubBackends(t, f)
	res, err := New().Extract(runCtx(t, "https://zhuanlan.zhihu.com/p/42"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != "egolite" || res.Data.(ArticleExtractData).Title != "Title" {
		t.Errorf("backend=%q data=%+v", res.Backend, res.Data)
	}
}
