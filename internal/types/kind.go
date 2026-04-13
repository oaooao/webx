package types

type WebxKind string

const (
	KindArticle      WebxKind = "article"
	KindConversation WebxKind = "conversation"
	KindThread       WebxKind = "thread"
	KindVideo        WebxKind = "video"
	KindComments     WebxKind = "comments"
	KindMetadata     WebxKind = "metadata"
)

var ValidKinds = []WebxKind{
	KindArticle, KindConversation, KindThread,
	KindVideo, KindComments, KindMetadata,
}

func (k WebxKind) IsValid() bool {
	for _, v := range ValidKinds {
		if k == v {
			return true
		}
	}
	return false
}
