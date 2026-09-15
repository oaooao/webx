package twitter

import (
	"strings"
	"testing"
)

func tw(id, author, replyTo, text string) Tweet {
	return Tweet{ID: id, Author: Author{Name: author + " name", ScreenName: author}, InReplyToID: replyTo, Text: text}
}

func ids(tweets []Tweet) string {
	var out []string
	for _, t := range tweets {
		out = append(out, t.ID)
	}
	return strings.Join(out, ",")
}

func TestSplitConversation_SelfThreadIsBodyNotReplies(t *testing.T) {
	tweets := []Tweet{
		tw("1", "neo", "", "intro 🧵"),
		tw("2", "neo", "1", "part 2"),
		tw("3", "neo", "2", "part 3"),
		tw("10", "raul", "1", "question"),
		tw("11", "neo", "10", "answer to raul"),
		tw("12", "taimur", "1", "comment"),
	}
	c := SplitConversation(tweets, "1")
	if c.Focal.ID != "1" || len(c.Ancestors) != 0 {
		t.Fatalf("focal=%q ancestors=%d", c.Focal.ID, len(c.Ancestors))
	}
	if got := ids(c.Thread); got != "2,3" {
		t.Errorf("thread = %q, want 2,3", got)
	}
	if got := ids(c.Replies); got != "10,11,12" {
		t.Errorf("replies = %q, want 10,11,12 (author answering others stays a reply)", got)
	}
}

func TestSplitConversation_ChainStopsAtFirstBreak(t *testing.T) {
	tweets := []Tweet{
		tw("1", "neo", "", "intro"),
		tw("2", "neo", "1", "part 2"),
		tw("9", "bob", "2", "reply to part 2"),
		tw("4", "neo", "2", "late self-reply"),
	}
	c := SplitConversation(tweets, "1")
	if got := ids(c.Thread); got != "2" {
		t.Errorf("thread = %q, want 2", got)
	}
	if got := ids(c.Replies); got != "9,4" {
		t.Errorf("replies = %q, want 9,4", got)
	}
}

func TestSplitConversation_FocalMidThreadKeepsAncestors(t *testing.T) {
	tweets := []Tweet{
		tw("1", "neo", "", "part 1"),
		tw("2", "neo", "1", "part 2"),
		tw("3", "neo", "2", "part 3"),
		tw("8", "bob", "2", "reply"),
	}
	c := SplitConversation(tweets, "2")
	if ids(c.Ancestors) != "1" || c.Focal.ID != "2" || ids(c.Thread) != "3" || ids(c.Replies) != "8" {
		t.Errorf("got ancestors=%q focal=%q thread=%q replies=%q", ids(c.Ancestors), c.Focal.ID, ids(c.Thread), ids(c.Replies))
	}
}

func TestSplitConversation_SingleTweetHasNoThread(t *testing.T) {
	c := SplitConversation([]Tweet{tw("1", "neo", "", "only")}, "1")
	if len(c.Thread) != 0 || len(c.Replies) != 0 {
		t.Errorf("thread=%d replies=%d", len(c.Thread), len(c.Replies))
	}
	if SplitConversation(nil, "1").Focal.ID != "" {
		t.Error("empty input must give empty conversation")
	}
}

func TestRenderConversationMarkdown_ThreadContinuesBody(t *testing.T) {
	c := SplitConversation([]Tweet{
		tw("1", "neo", "", "intro"),
		tw("2", "neo", "1", "part two"),
		tw("10", "raul", "1", "a comment"),
	}, "1")
	c.Focal.Metrics = map[string]int{"favorite_count": 5}
	md := RenderConversationMarkdown(c)

	if strings.Count(md, "## Reply by") != 1 || !strings.Contains(md, "## Reply by @raul") {
		t.Errorf("only the comment should be a reply:\n%s", md)
	}
	intro, part, likes, reply := strings.Index(md, "intro"), strings.Index(md, "part two"), strings.Index(md, "5 likes"), strings.Index(md, "## Reply by @raul")
	if !(intro < part && part < likes && likes < reply) {
		t.Errorf("want intro, part two, focal metrics, then replies:\n%s", md)
	}
	if strings.Contains(md, "@neo\n\n---") || strings.Count(md, "---") != 1 {
		t.Errorf("thread continuation must not get its own block:\n%s", md)
	}
}

func TestRenderArticleMarkdown(t *testing.T) {
	tweet := Tweet{
		ID: "5", Author: Author{Name: "Oji", ScreenName: "ojisan"}, CreatedAt: "Tue Sep 15 10:12:16 +0000 2026",
		ArticleTitle: "Skills attendance", ArticleText: "## Section\n\nBody text",
	}
	md := RenderArticleMarkdown(tweet)
	for _, want := range []string{"# Skills attendance\n", "**Oji** (@ojisan) · Tue Sep 15", "## Section\n\nBody text"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}
