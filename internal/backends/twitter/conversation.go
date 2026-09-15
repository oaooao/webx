package twitter

// Conversation is a TweetDetail timeline split around the focal tweet.
type Conversation struct {
	// Ancestors are the tweets the focal tweet replies to, oldest first.
	Ancestors []Tweet
	Focal     Tweet
	// Thread holds the focal author's self-replies that continue the focal
	// tweet (a "thread" in X terms), in reading order. They are body text,
	// not comments.
	Thread []Tweet
	// Replies are everything else below the focal tweet, including the
	// author's answers to other people.
	Replies []Tweet
}

// SplitConversation splits parsed TweetDetail tweets around focalID.
//
// X puts the author's continuation tweets first, in the first
// conversationthread module right after the focal tweet: each one is written
// by the focal author and replies to the previous tweet of the chain. The
// chain ends at the first tweet that breaks either rule, so an author reply
// to a commenter (which replies to the commenter's tweet) stays a reply.
// When focalID is not found the first tweet is treated as focal.
func SplitConversation(tweets []Tweet, focalID string) Conversation {
	if len(tweets) == 0 {
		return Conversation{}
	}
	focal := 0
	for i, t := range tweets {
		if t.ID == focalID {
			focal = i
			break
		}
	}

	c := Conversation{
		Ancestors: tweets[:focal],
		Focal:     tweets[focal],
	}
	rest := tweets[focal+1:]
	author := c.Focal.Author.ScreenName
	last := c.Focal.ID
	n := 0
	for _, t := range rest {
		if author == "" || t.Author.ScreenName != author || t.InReplyToID != last {
			break
		}
		last = t.ID
		n++
	}
	c.Thread = rest[:n]
	c.Replies = rest[n:]
	return c
}
