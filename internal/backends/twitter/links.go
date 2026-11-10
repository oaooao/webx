package twitter

import (
	"encoding/json"
	"net/url"
	"strings"
)

// linkSet maps the t.co short links in a tweet's text to what they stand
// for. X shortens every link in full_text (and in note-tweet text) to t.co;
// the original target is in the entities that come with the text, so no
// extra request is needed to expand them.
type linkSet struct {
	// expand maps a t.co URL to its expanded_url.
	expand map[string]string
	// media holds the t.co URLs that point at the tweet's own attached
	// media. X appends one to the text; the media are rendered separately.
	media map[string]bool
}

func newLinkSet() *linkSet {
	return &linkSet{expand: map[string]string{}, media: map[string]bool{}}
}

// collect reads an entities-shaped object: legacy.entities,
// legacy.extended_entities, or a note tweet's entity_set.
func (ls *linkSet) collect(raw json.RawMessage) {
	if raw == nil {
		return
	}
	var ents struct {
		URLs []struct {
			URL         string `json:"url"`
			ExpandedURL string `json:"expanded_url"`
		} `json:"urls"`
		Media []struct {
			URL string `json:"url"`
		} `json:"media"`
	}
	if json.Unmarshal(raw, &ents) != nil {
		return
	}
	for _, u := range ents.URLs {
		if u.URL != "" && u.ExpandedURL != "" {
			ls.expand[u.URL] = u.ExpandedURL
		}
	}
	for _, m := range ents.Media {
		if m.URL != "" {
			ls.media[m.URL] = true
		}
	}
}

// apply expands t.co links in text and removes the media placeholders.
func (ls *linkSet) apply(text string) string {
	for short, long := range ls.expand {
		text = strings.ReplaceAll(text, short, long)
	}
	if len(ls.media) == 0 {
		return text
	}
	for short := range ls.media {
		text = strings.ReplaceAll(text, short, "")
	}
	return strings.TrimRight(text, " \n")
}

// largePhotoURL asks X's image host for the large rendition (up to
// 2048px on the long side) instead of the default, which is too small for
// reading text in screenshots and diagrams. Other URLs pass through.
func largePhotoURL(mediaType, raw string) string {
	if mediaType != "photo" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host != "pbs.twimg.com" || !strings.HasPrefix(u.Path, "/media/") {
		return raw
	}
	q := u.Query()
	if q.Get("name") != "" {
		return raw
	}
	q.Set("name", "large")
	u.RawQuery = q.Encode()
	return u.String()
}
