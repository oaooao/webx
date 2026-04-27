package twitter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// extractArticle pulls the X Article payload (long-form Draft.js content) from
// a tweet result node. Returns ("", "") when the tweet has no article.
//
// Path inside result: article.article_results.result
//
//	├── title                   string
//	└── content_state
//	    ├── blocks[]            Draft.js content blocks
//	    │   ├── type            "header-one" | "blockquote" | "unordered-list-item" |
//	    │   │                   "ordered-list-item" | "code-block" | "atomic" | "unstyled" | ...
//	    │   ├── text            string
//	    │   └── entityRanges[]  inline references into entityMap (LINK, IMAGE, MARKDOWN, ...)
//	    └── entityMap           dict-or-list of entity definitions
//
// Reference: jackwener/twitter-cli parser.py:_parse_article (MIT).
func extractArticle(result map[string]json.RawMessage) (title, text string) {
	articleRaw, ok := result["article"]
	if !ok {
		return "", ""
	}
	var article map[string]json.RawMessage
	if json.Unmarshal(articleRaw, &article) != nil {
		return "", ""
	}
	resultsRaw, ok := article["article_results"]
	if !ok {
		return "", ""
	}
	var results map[string]json.RawMessage
	if json.Unmarshal(resultsRaw, &results) != nil {
		return "", ""
	}
	innerRaw, ok := results["result"]
	if !ok {
		return "", ""
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(innerRaw, &inner) != nil {
		return "", ""
	}

	title = jsonString(inner["title"])

	// TweetDetail returns only {title, preview_text} for articles. The full
	// Draft.js content_state is only present in TweetResultByRestId responses.
	// When ArticleTitle is set but ArticleText is empty, the adapter layer
	// triggers a follow-up FetchArticleByTweetID request to enrich it.
	csRaw, ok := inner["content_state"]
	if !ok {
		return title, ""
	}
	var contentState struct {
		Blocks    []articleBlock         `json:"blocks"`
		EntityMap json.RawMessage        `json:"entityMap"`
	}
	if json.Unmarshal(csRaw, &contentState) != nil || len(contentState.Blocks) == 0 {
		return title, ""
	}

	entityMap := normalizeEntityMap(contentState.EntityMap)
	mediaURLMap := extractArticleMediaURLMap(inner)

	var parts []string
	orderedCounter := 0
	for _, block := range contentState.Blocks {
		if block.Type == "atomic" {
			parts = append(parts, extractAtomicMarkdown(block, entityMap)...)
			parts = append(parts, extractArticleImages(block, entityMap, mediaURLMap)...)
			orderedCounter = 0
			continue
		}

		rendered := renderArticleTextBlock(block, entityMap)
		if rendered == "" {
			continue
		}
		if block.Type != "ordered-list-item" {
			orderedCounter = 0
		}
		switch block.Type {
		case "header-one":
			parts = append(parts, "# "+rendered)
		case "header-two":
			parts = append(parts, "## "+rendered)
		case "header-three":
			parts = append(parts, "### "+rendered)
		case "blockquote":
			parts = append(parts, "> "+rendered)
		case "unordered-list-item":
			parts = append(parts, "- "+rendered)
		case "ordered-list-item":
			orderedCounter++
			parts = append(parts, fmt.Sprintf("%d. %s", orderedCounter, rendered))
		case "code-block":
			parts = append(parts, "```\n"+rendered+"\n```")
		default:
			parts = append(parts, rendered)
		}
	}

	if len(parts) == 0 {
		return title, ""
	}
	return title, strings.Join(parts, "\n\n")
}

type articleBlock struct {
	Type          string             `json:"type"`
	Text          string             `json:"text"`
	EntityRanges  []articleEntityRef `json:"entityRanges"`
}

type articleEntityRef struct {
	Key    int `json:"key"`
	Offset int `json:"offset"`
	Length int `json:"length"`
}

// normalizeEntityMap accepts both the dict form ({"0": {...}}) and the list
// form ([{"key": 0, "value": {...}}]) seen across X API versions.
func normalizeEntityMap(raw json.RawMessage) map[string]map[string]json.RawMessage {
	out := map[string]map[string]json.RawMessage{}
	if len(raw) == 0 {
		return out
	}

	// Try dict shape first.
	var asDict map[string]json.RawMessage
	if json.Unmarshal(raw, &asDict) == nil {
		for k, v := range asDict {
			var entity map[string]json.RawMessage
			if json.Unmarshal(v, &entity) == nil {
				out[k] = entity
			}
		}
		return out
	}

	// Fallback to list shape.
	var asList []struct {
		Key   json.RawMessage `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if json.Unmarshal(raw, &asList) != nil {
		return out
	}
	for _, item := range asList {
		var key string
		if json.Unmarshal(item.Key, &key) != nil {
			var n int
			if json.Unmarshal(item.Key, &n) != nil {
				continue
			}
			key = fmt.Sprintf("%d", n)
		}
		var entity map[string]json.RawMessage
		if json.Unmarshal(item.Value, &entity) == nil {
			out[key] = entity
		}
	}
	return out
}

func entityType(entity map[string]json.RawMessage) string {
	return strings.ToUpper(jsonString(entity["type"]))
}

// renderArticleTextBlock substitutes inline LINK entities with markdown link
// syntax in reverse order to avoid offset shifts.
func renderArticleTextBlock(block articleBlock, entityMap map[string]map[string]json.RawMessage) string {
	if block.Text == "" {
		return ""
	}
	if len(block.EntityRanges) == 0 {
		return block.Text
	}

	type linkRange struct {
		offset, length int
		url            string
	}
	var ranges []linkRange
	for _, ref := range block.EntityRanges {
		entity, ok := entityMap[fmt.Sprintf("%d", ref.Key)]
		if !ok || entityType(entity) != "LINK" {
			continue
		}
		url := strings.TrimSpace(extractEntityURL(entity))
		if url == "" || ref.Length <= 0 {
			continue
		}
		ranges = append(ranges, linkRange{ref.Offset, ref.Length, url})
	}
	if len(ranges) == 0 {
		return block.Text
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].offset > ranges[j].offset })
	rendered := block.Text
	for _, r := range ranges {
		if r.offset < 0 || r.offset+r.length > len(rendered) {
			continue
		}
		label := rendered[r.offset : r.offset+r.length]
		if label == "" {
			continue
		}
		safeLabel := strings.NewReplacer("[", `\[`, "]", `\]`).Replace(label)
		safeURL := strings.ReplaceAll(r.url, ")", "%29")
		rendered = rendered[:r.offset] + "[" + safeLabel + "](" + safeURL + ")" + rendered[r.offset+r.length:]
	}
	return rendered
}

func extractEntityURL(entity map[string]json.RawMessage) string {
	dataRaw, ok := entity["data"]
	if !ok {
		return ""
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(dataRaw, &data) != nil {
		return ""
	}
	return jsonString(data["url"])
}

// extractAtomicMarkdown surfaces embedded MARKDOWN entities (used for code
// snippets, embeds) verbatim.
func extractAtomicMarkdown(block articleBlock, entityMap map[string]map[string]json.RawMessage) []string {
	var parts []string
	for _, ref := range block.EntityRanges {
		entity, ok := entityMap[fmt.Sprintf("%d", ref.Key)]
		if !ok || entityType(entity) != "MARKDOWN" {
			continue
		}
		dataRaw, ok := entity["data"]
		if !ok {
			continue
		}
		var data map[string]json.RawMessage
		if json.Unmarshal(dataRaw, &data) != nil {
			continue
		}
		md := strings.TrimSpace(jsonString(data["markdown"]))
		if md != "" {
			parts = append(parts, md)
		}
	}
	return parts
}

// extractArticleImages converts atomic image entities to Markdown image lines.
// Caption is best-effort; falls back to empty alt text.
func extractArticleImages(block articleBlock, entityMap map[string]map[string]json.RawMessage, mediaURLMap map[string]string) []string {
	var parts []string
	for _, ref := range block.EntityRanges {
		entity, ok := entityMap[fmt.Sprintf("%d", ref.Key)]
		if !ok {
			continue
		}
		imageURL := findArticleImageURL(entity)
		if imageURL == "" {
			imageURL = lookupMediaItemURL(entity, mediaURLMap)
		}
		if imageURL == "" {
			continue
		}
		caption := findArticleCaption(entity)
		parts = append(parts, fmt.Sprintf("![%s](%s)", caption, imageURL))
	}
	return parts
}

func lookupMediaItemURL(entity map[string]json.RawMessage, mediaURLMap map[string]string) string {
	dataRaw, ok := entity["data"]
	if !ok {
		return ""
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(dataRaw, &data) != nil {
		return ""
	}
	itemsRaw, ok := data["mediaItems"]
	if !ok {
		return ""
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(itemsRaw, &items) != nil {
		return ""
	}
	for _, item := range items {
		id := jsonString(item["mediaId"])
		if url, ok := mediaURLMap[id]; ok && url != "" {
			return url
		}
	}
	return ""
}

// findArticleImageURL walks an entity dict tree looking for a plausible
// image URL field. Conservative on what counts as an image (twimg host or
// known image extensions).
func findArticleImageURL(value any) string {
	switch v := value.(type) {
	case map[string]json.RawMessage:
		for _, k := range []string{
			"original_img_url", "originalImgUrl",
			"original_url", "originalUrl",
			"media_url_https", "mediaUrlHttps",
			"media_url", "mediaUrl",
			"url", "src", "uri",
		} {
			if raw, ok := v[k]; ok {
				if s := jsonString(raw); looksLikeImageURL(s) {
					return s
				}
			}
		}
		for _, raw := range v {
			var nested any
			if json.Unmarshal(raw, &nested) == nil {
				if found := findArticleImageURL(nested); found != "" {
					return found
				}
			}
		}
	case map[string]any:
		for _, k := range []string{
			"original_img_url", "originalImgUrl",
			"original_url", "originalUrl",
			"media_url_https", "mediaUrlHttps",
			"media_url", "mediaUrl",
			"url", "src", "uri",
		} {
			if s, ok := v[k].(string); ok && looksLikeImageURL(s) {
				return s
			}
		}
		for _, nested := range v {
			if found := findArticleImageURL(nested); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range v {
			if found := findArticleImageURL(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func looksLikeImageURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "https://pbs.twimg.com/") {
		return true
	}
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
		if strings.HasSuffix(lower, ext) || strings.Contains(lower, ext+"?") {
			return true
		}
	}
	return false
}

func findArticleCaption(entity map[string]json.RawMessage) string {
	for _, k := range []string{"caption", "alt", "alt_text", "altText", "title", "name"} {
		if raw, ok := entity[k]; ok {
			if s := strings.TrimSpace(jsonString(raw)); s != "" {
				return s
			}
		}
	}
	dataRaw, ok := entity["data"]
	if !ok {
		return ""
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(dataRaw, &data) != nil {
		return ""
	}
	for _, k := range []string{"caption", "alt", "alt_text", "altText", "title", "name"} {
		if raw, ok := data[k]; ok {
			if s := strings.TrimSpace(jsonString(raw)); s != "" {
				return s
			}
		}
	}
	return ""
}

// extractArticleMediaURLMap maps media ids/keys to original image URLs.
// Used when entityRanges reference media by id only.
func extractArticleMediaURLMap(inner map[string]json.RawMessage) map[string]string {
	out := map[string]string{}

	candidates := [][]byte{}
	if cm, ok := inner["cover_media"]; ok {
		candidates = append(candidates, cm)
	}
	if me, ok := inner["media_entities"]; ok {
		var list []json.RawMessage
		if json.Unmarshal(me, &list) == nil {
			for _, item := range list {
				candidates = append(candidates, item)
			}
		}
	}

	for _, raw := range candidates {
		var media map[string]json.RawMessage
		if json.Unmarshal(raw, &media) != nil {
			continue
		}
		var info any
		if mi, ok := media["media_info"]; ok {
			_ = json.Unmarshal(mi, &info)
		}
		imageURL := findArticleImageURL(info)
		if imageURL == "" {
			var whole any
			if json.Unmarshal(raw, &whole) == nil {
				imageURL = findArticleImageURL(whole)
			}
		}
		if imageURL == "" {
			continue
		}
		for _, k := range []string{"media_id", "media_key", "id"} {
			if id := jsonString(media[k]); id != "" {
				out[id] = imageURL
			}
		}
	}
	return out
}
