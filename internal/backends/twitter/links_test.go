package twitter

import (
	"encoding/json"
	"testing"
)

func TestParseTweetResult_ExpandsLinksAndDropsMediaPlaceholder(t *testing.T) {
	raw := json.RawMessage(`{
	  "__typename": "Tweet",
	  "rest_id": "1",
	  "legacy": {
	    "full_text": "read this https://t.co/abc https://t.co/img",
	    "entities": {
	      "urls": [{"url": "https://t.co/abc", "expanded_url": "https://example.com/atlas/"}],
	      "media": [{"url": "https://t.co/img"}]
	    },
	    "extended_entities": {
	      "media": [{"url": "https://t.co/img", "type": "photo", "media_url_https": "https://pbs.twimg.com/media/HWp3.jpg"}]
	    }
	  }
	}`)
	tw := parseTweetResult(raw)
	if tw == nil {
		t.Fatal("nil tweet")
	}
	if tw.Text != "read this https://example.com/atlas/" {
		t.Errorf("text = %q", tw.Text)
	}
	if len(tw.Media) != 1 || tw.Media[0].URL != "https://pbs.twimg.com/media/HWp3.jpg?name=large" {
		t.Errorf("media = %+v", tw.Media)
	}
}

func TestParseTweetResult_NoteTweetUsesEntitySet(t *testing.T) {
	raw := json.RawMessage(`{
	  "__typename": "Tweet",
	  "rest_id": "2",
	  "legacy": {"full_text": "truncated…"},
	  "note_tweet": {"note_tweet_results": {"result": {
	    "text": "long text with https://t.co/x in it",
	    "entity_set": {"urls": [{"url": "https://t.co/x", "expanded_url": "https://example.org"}]}
	  }}}
	}`)
	tw := parseTweetResult(raw)
	if tw == nil || tw.Text != "long text with https://example.org in it" {
		t.Fatalf("got %+v", tw)
	}
}

func TestLargePhotoURL(t *testing.T) {
	cases := []struct{ typ, in, want string }{
		{"photo", "https://pbs.twimg.com/media/A.jpg", "https://pbs.twimg.com/media/A.jpg?name=large"},
		{"photo", "https://pbs.twimg.com/media/A.jpg?name=orig", "https://pbs.twimg.com/media/A.jpg?name=orig"},
		{"video", "https://pbs.twimg.com/ext_tw_video_thumb/1/pu/img/B.jpg", "https://pbs.twimg.com/ext_tw_video_thumb/1/pu/img/B.jpg"},
		{"photo", "https://example.com/media/C.jpg", "https://example.com/media/C.jpg"},
	}
	for _, c := range cases {
		if got := largePhotoURL(c.typ, c.in); got != c.want {
			t.Errorf("largePhotoURL(%q, %q) = %q, want %q", c.typ, c.in, got, c.want)
		}
	}
}
