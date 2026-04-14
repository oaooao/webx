package types

type WebxKind string

const (
	KindArticle      WebxKind = "article"
	KindConversation WebxKind = "conversation"
	KindThread       WebxKind = "thread"
	KindVideo        WebxKind = "video"
	KindComments     WebxKind = "comments"
	KindMetadata     WebxKind = "metadata"
	KindSearch       WebxKind = "search"
	KindWrite        WebxKind = "write"
)

var ValidKinds = []WebxKind{
	KindArticle, KindConversation, KindThread,
	KindVideo, KindComments, KindMetadata,
	KindSearch, KindWrite,
}

func (k WebxKind) IsValid() bool {
	for _, v := range ValidKinds {
		if k == v {
			return true
		}
	}
	return false
}
