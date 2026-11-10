package twitter

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestRequestErrorString_DropsQuery(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://x.com/i/api/graphql/abc/TweetDetail?features=" + strings.Repeat("x", 2000),
		Err: errors.New("read tcp: connection reset by peer"),
	}
	got := requestErrorString(err)
	want := "Get https://x.com/i/api/graphql/abc/TweetDetail: read tcp: connection reset by peer"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
