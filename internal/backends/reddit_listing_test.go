package backends

import (
	"strings"
	"testing"
)

// A subreddit listing (/r/<sub>/top.json) is one Listing object, not the
// [post, comments] pair a thread returns. On 2026-11-21, after the ego lite
// fetch started returning 200, `webx read https://www.reddit.com/r/ExperiencedDevs/top/?t=day`
// still failed with "cannot unmarshal object into Go value of type
// []backends.RedditListing".
const subredditListingFixture = `{"kind":"Listing","data":{"after":"t3_b","children":[
 {"kind":"t3","data":{"id":"a","title":"First post","author":"alice","subreddit":"ExperiencedDevs","permalink":"/r/ExperiencedDevs/comments/a/first_post/","score":512,"num_comments":88,"created_utc":1763700000,"selftext":"body a"}},
 {"kind":"t3","data":{"id":"b","title":"Second post","author":"bob","subreddit":"ExperiencedDevs","permalink":"/r/ExperiencedDevs/comments/b/second_post/","score":40,"num_comments":3,"created_utc":1763701000,"url":"https://example.com/x"}}
]}}`

func TestDecodeRedditListings_AcceptsSubredditListing(t *testing.T) {
	listings, err := decodeRedditListings([]byte(subredditListingFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 || len(listings[0].Data.Children) != 2 {
		t.Fatalf("listings = %+v, want one listing with two posts", listings)
	}
}

func TestParseRedditPostListing_RendersPosts(t *testing.T) {
	listings, err := decodeRedditListings([]byte(subredditListingFixture))
	if err != nil {
		t.Fatal(err)
	}
	posts := ParseRedditPostListing(listings)
	if len(posts) != 2 || posts[0].Title != "First post" || posts[1].NumComments != 3 {
		t.Fatalf("posts = %+v", posts)
	}
	md := RenderRedditPostListingMarkdown("r/ExperiencedDevs", posts)
	for _, want := range []string{"# r/ExperiencedDevs", "## 1. First post", "https://www.reddit.com/r/ExperiencedDevs/comments/a/first_post/", "Score: 512", "Comments: 88", "## 2. Second post"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}
