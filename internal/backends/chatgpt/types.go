package chatgpt

// Conversation is the normalized representation of a ChatGPT shared conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Model     string    `json:"model,omitempty"`
	CreatedAt string    `json:"created_at,omitempty"`
	Messages  []Message `json:"messages"`
}

// Message is a single turn in the conversation.
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"` // "user", "assistant", "system"
	Content   string `json:"content"`
	CreatedAt string `json:"created_at,omitempty"`
}
