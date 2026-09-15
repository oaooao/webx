package generic

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/oaooao/webx/internal/types"
)

// A successful fetch is not a successful read: sites answer bots and
// signed-out visitors with HTTP 200 pages that hold an error notice, a
// verification wall or a login shell. assessContent decides whether extracted
// content is the article or one of those pages.

const (
	// minTextRunes is the fewest letters/digits an article body may have.
	minTextRunes = 150
	// wallTextRunes bounds where a wall phrase is trusted: wall pages are
	// short, and a real article may quote the same phrase.
	wallTextRunes = 1500
	// weakWallTextRunes bounds generic login phrases ("登录/注册"), which
	// also appear in the header of normal pages.
	weakWallTextRunes = 400
	// navLinkShare is the share of text inside links above which a page
	// counts as navigation only.
	navLinkShare = 0.6
)

type wallPhrase struct {
	text string
	code types.ErrorCode
	// weak phrases only count on very short or link-only pages.
	weak bool
}

// wallPhrases are matched case-insensitively against title and text, in
// order: a missing-content notice beats a login prompt on the same page.
var wallPhrases = []wallPhrase{
	// The page says the content does not exist.
	{text: "parameter error", code: types.ErrContentEmpty},
	{text: "参数错误", code: types.ErrContentEmpty},
	{text: "该内容已被发布者删除", code: types.ErrContentEmpty},
	{text: "此内容因违规无法查看", code: types.ErrContentEmpty},
	{text: "内容已被删除", code: types.ErrContentEmpty},
	{text: "你似乎来到了没有知识存在的荒原", code: types.ErrContentEmpty},
	{text: "页面不存在", code: types.ErrContentEmpty},
	{text: "404 not found", code: types.ErrContentEmpty},
	{text: "page not found", code: types.ErrContentEmpty},
	// WeChat article chrome without an article: only the like/share bar
	// survived extraction.
	{text: "轻点两下取消赞", code: types.ErrContentEmpty, weak: true},
	// Anti-bot verification.
	{text: "环境异常", code: types.ErrAntiBot},
	{text: "完成验证后即可继续访问", code: types.ErrAntiBot},
	{text: "请完成安全验证", code: types.ErrAntiBot},
	{text: "访问过于频繁", code: types.ErrAntiBot},
	{text: "人机验证", code: types.ErrAntiBot},
	{text: "just a moment...", code: types.ErrAntiBot},
	{text: "verify you are human", code: types.ErrAntiBot},
	{text: "checking your browser", code: types.ErrAntiBot},
	{text: "unusual traffic", code: types.ErrAntiBot},
	// Login walls.
	{text: "登录后查看", code: types.ErrLoginRequired},
	{text: "登录后即可", code: types.ErrLoginRequired},
	{text: "sign in to continue", code: types.ErrLoginRequired},
	{text: "log in to continue", code: types.ErrLoginRequired},
	{text: "you must be logged in", code: types.ErrLoginRequired},
	{text: "登录/注册", code: types.ErrLoginRequired, weak: true},
	{text: "验证码登录", code: types.ErrLoginRequired, weak: true},
	{text: "扫码登录", code: types.ErrLoginRequired, weak: true},
}

// contentIssue explains why extracted content is not the article.
type contentIssue struct {
	Code   types.ErrorCode
	Reason string
}

var (
	mdImageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLinkRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	bareURLRe = regexp.MustCompile(`https?://\S+`)
)

// assessContent returns nil when title+markdown look like a real article.
func assessContent(title, markdown string) *contentIssue {
	noImages := mdImageRe.ReplaceAllString(markdown, " ")
	linkRunes := 0
	for _, m := range mdLinkRe.FindAllStringSubmatch(noImages, -1) {
		linkRunes += countTextRunes(m[1])
	}
	text := bareURLRe.ReplaceAllString(mdLinkRe.ReplaceAllString(noImages, "$1"), " ")
	n := countTextRunes(text)
	linkOnly := n > 0 && float64(linkRunes)/float64(n) > navLinkShare

	haystack := strings.ToLower(title + "\n" + text)
	if n < wallTextRunes {
		for _, p := range wallPhrases {
			if p.weak && n >= weakWallTextRunes && !linkOnly {
				continue
			}
			if strings.Contains(haystack, p.text) {
				return &contentIssue{Code: p.code, Reason: wallReason(p, n)}
			}
		}
	}

	switch {
	case n == 0:
		return &contentIssue{Code: types.ErrContentEmpty, Reason: "extracted content has no text"}
	case linkOnly && n < wallTextRunes:
		return &contentIssue{Code: types.ErrContentEmpty, Reason: fmt.Sprintf(
			"extracted content is navigation only (%d%% of %d text characters are links)", linkRunes*100/n, n)}
	case n < minTextRunes:
		return &contentIssue{Code: types.ErrContentEmpty, Reason: fmt.Sprintf(
			"extracted text is too short to be an article (%d characters)", n)}
	}
	return nil
}

func wallReason(p wallPhrase, n int) string {
	var kind string
	switch p.code {
	case types.ErrAntiBot:
		kind = "an anti-bot verification page"
	case types.ErrLoginRequired:
		kind = "a login wall"
	default:
		kind = "an error or missing-content page"
	}
	return fmt.Sprintf("page is %s (found %q, %d text characters)", kind, p.text, n)
}

// countTextRunes counts letters and digits, so punctuation, markdown syntax
// and whitespace do not make a wall page look long.
func countTextRunes(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

// suggestion is the next step for a failed read, by error code.
func suggestion(code types.ErrorCode, host string) string {
	switch code {
	case types.ErrAntiBot:
		return "Open the URL in ego lite, pass any verification, then retry."
	case types.ErrLoginRequired:
		return fmt.Sprintf("Sign in to %s in ego lite (keep ego lite running), then retry.", host)
	default:
		return "Check that the link is complete and the content still exists; if the page needs clicks or scrolling, read it with ego lite directly."
	}
}
