package chatgpt

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders a Conversation as readable markdown.
func RenderMarkdown(conv *Conversation) string {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(conv.Title)
	b.WriteString("\n\n")

	if conv.Model != "" {
		fmt.Fprintf(&b, "**Model:** %s\n\n", FormatModel(conv.Model))
	}

	if conv.CreatedAt != "" {
		fmt.Fprintf(&b, "**Created:** %s\n\n", conv.CreatedAt)
	}

	for i, msg := range conv.Messages {
		if i > 0 || conv.Model != "" || conv.CreatedAt != "" {
			b.WriteString("---\n\n")
		}

		role := strings.ToUpper(msg.Role[:1]) + msg.Role[1:]
		fmt.Fprintf(&b, "## %s\n\n", role)
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
