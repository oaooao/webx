package backends

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripHTMLTags_ParagraphBreak(t *testing.T) {
	input := "First paragraph.<p>Second paragraph."
	got := StripHTMLTags(input)
	if !strings.Contains(got, "\n\n") {
		t.Errorf("expected double newline from <p>, got: %q", got)
	}
	if !strings.Contains(got, "First paragraph.") || !strings.Contains(got, "Second paragraph.") {
		t.Errorf("paragraph text missing: %q", got)
	}
}

func TestStripHTMLTags_AnchorLink(t *testing.T) {
	input := `Check out <a href="https://example.com">Example</a> here.`
	got := StripHTMLTags(input)
	want := "[Example](https://example.com)"
	if !strings.Contains(got, want) {
		t.Errorf("anchor not converted: got %q, want to contain %q", got, want)
	}
}

func TestStripHTMLTags_InlineCode(t *testing.T) {
	input := "Use <code>fmt.Println</code> to print."
	got := StripHTMLTags(input)
	if !strings.Contains(got, "`fmt.Println`") {
		t.Errorf("inline code not converted: %q", got)
	}
}

func TestStripHTMLTags_FencedCode(t *testing.T) {
	input := "Example:<pre><code>func main() {}\n</code></pre>done."
	got := StripHTMLTags(input)
	if !strings.Contains(got, "```") {
		t.Errorf("fenced code block not created: %q", got)
	}
	if !strings.Contains(got, "func main()") {
		t.Errorf("code content missing: %q", got)
	}
}

func TestStripHTMLTags_Italic(t *testing.T) {
	input := "This is <i>italic</i> and <em>emphasised</em> text."
	got := StripHTMLTags(input)
	if !strings.Contains(got, "*italic*") {
		t.Errorf("italic not converted: %q", got)
	}
	if !strings.Contains(got, "*emphasised*") {
		t.Errorf("em not converted: %q", got)
	}
}

func TestStripHTMLTags_Entities(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", `"`},
		{"&#39;", "'"},
		{"&apos;", "'"},
	}
	for _, c := range cases {
		got := StripHTMLTags(c.input)
		if got != c.want {
			t.Errorf("entity %q: got %q, want %q", c.input, got, c.want)
		}
	}
}

func TestStripHTMLTags_StripUnknownTags(t *testing.T) {
	input := "<div>content<span>inside</span></div>"
	got := StripHTMLTags(input)
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("HTML tags not stripped: %q", got)
	}
	if !strings.Contains(got, "content") || !strings.Contains(got, "inside") {
		t.Errorf("text content missing: %q", got)
	}
}

func buildHNItemFixture() HNItem {
	text := "This is the <i>story text</i>.<p>With a second paragraph."
	points := 150
	c1text := "A top-level <a href=\"https://example.com\">comment</a>."
	c2text := "A nested reply with <code>code</code>."

	return HNItem{
		ID:        12345,
		Title:     "Ask HN: How does Go handle goroutines?",
		URL:       "https://news.ycombinator.com/item?id=12345",
		Text:      &text,
		Author:    "hnuser",
		Points:    &points,
		CreatedAt: "2024-01-01T00:00:00Z",
		Children: []HNItem{
			{
				ID:     12346,
				Author: "commenter1",
				Text:   &c1text,
				Children: []HNItem{
					{
						ID:     12347,
						Author: "commenter2",
						Text:   &c2text,
					},
				},
			},
		},
	}
}

func TestRenderHNItemMarkdown_Title(t *testing.T) {
	item := buildHNItemFixture()
	md := RenderHNItemMarkdown(&item)

	if !strings.HasPrefix(md, "# Ask HN: How does Go handle goroutines?") {
		t.Errorf("markdown should start with title: %q", md[:100])
	}
}

func TestRenderHNItemMarkdown_Metadata(t *testing.T) {
	item := buildHNItemFixture()
	md := RenderHNItemMarkdown(&item)

	checks := []string{
		"Author: hnuser",
		"Points: 150",
		"Created: 2024-01-01T00:00:00Z",
	}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
}

func TestRenderHNItemMarkdown_HTMLConversion(t *testing.T) {
	item := buildHNItemFixture()
	md := RenderHNItemMarkdown(&item)

	// <i> should become *italic*
	if !strings.Contains(md, "*story text*") {
		t.Errorf("italic not converted in post text: %q", md)
	}
	// <a href> should become markdown link
	if !strings.Contains(md, "[comment](https://example.com)") {
		t.Errorf("link not converted in comments: %q", md)
	}
	// <code> should become backtick
	if !strings.Contains(md, "`code`") {
		t.Errorf("inline code not converted in comments: %q", md)
	}
}

func TestRenderHNItemMarkdown_CommentTree(t *testing.T) {
	item := buildHNItemFixture()
	md := RenderHNItemMarkdown(&item)

	if !strings.Contains(md, "## Comments") {
		t.Errorf("missing comments section")
	}
	if !strings.Contains(md, "**commenter1**") {
		t.Errorf("top-level comment author missing")
	}
	if !strings.Contains(md, "**commenter2**") {
		t.Errorf("nested comment author missing")
	}

	// Nested comment should be more indented
	lines := strings.Split(md, "\n")
	for _, line := range lines {
		if strings.Contains(line, "**commenter2**") {
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("nested comment not indented: %q", line)
			}
		}
	}
}

func TestRenderHNItemMarkdown_NoComments(t *testing.T) {
	item := HNItem{
		ID:    99,
		Title: "Simple post",
	}
	md := RenderHNItemMarkdown(&item)
	if strings.Contains(md, "## Comments") {
		t.Error("should not have comments section when no children")
	}
}

func TestRenderHNItemMarkdown_EmptyTitle(t *testing.T) {
	item := HNItem{ID: 42}
	md := RenderHNItemMarkdown(&item)
	if !strings.Contains(md, "HN item 42") {
		t.Errorf("should use fallback title with item ID: %q", md)
	}
}

// --- HN Search tests ---

// buildHNSearchResponseFixture returns a minimal Algolia search JSON response.
func buildHNSearchResponseFixture() []byte {
	return []byte(`{
		"hits": [
			{
				"objectID": "11111",
				"title": "Go 1.24 Released",
				"url": "https://go.dev/blog/go1.24",
				"author": "gopher",
				"points": 250,
				"created_at": "2024-02-01T10:00:00Z",
				"num_comments": 42
			},
			{
				"objectID": "22222",
				"title": "Understanding goroutines",
				"url": "https://example.com/goroutines",
				"author": "devblog",
				"points": 100,
				"created_at": "2024-01-15T08:00:00Z",
				"num_comments": 10
			}
		],
		"nbHits": 2,
		"page": 0,
		"nbPages": 1,
		"hitsPerPage": 20
	}`)
}

func TestParseHNSearchResponse_WhenValid_ShouldParseHits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildHNSearchResponseFixture())
	}))
	defer ts.Close()

	result, err := SearchHNStories(ts.URL+"?query=golang", 20)
	if err != nil {
		t.Fatalf("SearchHNStories: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	// First item
	item := result.Items[0]
	if item.Title != "Go 1.24 Released" {
		t.Errorf("title: got %q, want %q", item.Title, "Go 1.24 Released")
	}
	// URL points to the HN item page (canonical), not the external URL
	if item.URL != "https://news.ycombinator.com/item?id=11111" {
		t.Errorf("URL: got %q, want HN item URL", item.URL)
	}
	if item.Author != "gopher" {
		t.Errorf("author: got %q", item.Author)
	}
	if item.Score != 250 {
		t.Errorf("score: got %f, want 250", item.Score)
	}
	// External URL should be in Meta
	if item.Meta == nil {
		t.Error("Meta should not be nil")
	}
	if externalURL, ok := item.Meta["external_url"]; !ok || externalURL != "https://go.dev/blog/go1.24" {
		t.Errorf("Meta[external_url]: got %v, want https://go.dev/blog/go1.24", item.Meta["external_url"])
	}
}

func TestParseHNSearchResponse_WhenEmpty_ShouldReturnEmptyItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hits": [], "nbHits": 0}`))
	}))
	defer ts.Close()

	result, err := SearchHNStories(ts.URL+"?query=xyznotexist", 20)
	if err != nil {
		t.Fatalf("SearchHNStories: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items for empty response, got %d", len(result.Items))
	}
}

func TestParseHNSearchResponse_WhenHTTPError_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	_, err := SearchHNStories(ts.URL+"?query=golang", 20)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestParseHNSearchResponse_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not valid json`))
	}))
	defer ts.Close()

	_, err := SearchHNStories(ts.URL+"?query=golang", 20)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
