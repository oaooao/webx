package claude

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

// FormatModel returns a display-friendly model name for Claude models.
func FormatModel(slug string) string {
	known := map[string]string{
		"claude-3-opus-20240229":     "Claude 3 Opus",
		"claude-3-sonnet-20240229":   "Claude 3 Sonnet",
		"claude-3-haiku-20240307":    "Claude 3 Haiku",
		"claude-3-5-sonnet-20241022": "Claude 3.5 Sonnet",
		"claude-3-5-haiku-20241022":  "Claude 3.5 Haiku",
		"claude-sonnet-4-20250514":   "Claude Sonnet 4",
		"claude-opus-4-20250514":     "Claude Opus 4",
	}
	if name, ok := known[slug]; ok {
		return name
	}

	// Prefix-based matching for version variants.
	prefixes := []struct {
		prefix string
		name   string
	}{
		{"claude-opus-4", "Claude Opus 4"},
		{"claude-sonnet-4", "Claude Sonnet 4"},
		{"claude-3-5-sonnet", "Claude 3.5 Sonnet"},
		{"claude-3-5-haiku", "Claude 3.5 Haiku"},
		{"claude-3-opus", "Claude 3 Opus"},
		{"claude-3-sonnet", "Claude 3 Sonnet"},
		{"claude-3-haiku", "Claude 3 Haiku"},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(slug, p.prefix) {
			return fmt.Sprintf("%s (%s)", p.name, slug)
		}
	}

	return slug
}
