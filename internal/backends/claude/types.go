package claude

// Conversation is the normalized representation of a Claude shared conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Model     string    `json:"model,omitempty"`
	CreatedAt string    `json:"created_at,omitempty"`
	Messages  []Message `json:"messages"`
}

// Message is a single turn in the conversation.
type Message struct {
	UUID      string `json:"uuid"`
	Role      string `json:"role"` // "human" or "assistant"
	Content   string `json:"content"`
	CreatedAt string `json:"created_at,omitempty"`
}
