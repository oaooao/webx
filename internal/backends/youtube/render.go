package youtube

import (
	"fmt"
	"strings"
)

// RenderMarkdown converts a FetchResult into readable Markdown.
func RenderMarkdown(result *FetchResult) string {
	var sb strings.Builder

	v := result.Video

	fmt.Fprintf(&sb, "# %s\n\n", v.Title)

	if v.Channel != "" {
		fmt.Fprintf(&sb, "Channel: %s\n", v.Channel)
	}
	if v.Duration != "" {
		fmt.Fprintf(&sb, "Duration: %s\n", v.Duration)
	}
	if v.ViewCount > 0 {
		fmt.Fprintf(&sb, "Views: %d\n", v.ViewCount)
	}
	if v.PublishDate != "" {
		fmt.Fprintf(&sb, "Published: %s\n", v.PublishDate)
	}
	fmt.Fprintf(&sb, "URL: https://www.youtube.com/watch?v=%s\n", v.ID)

	if v.Description != "" {
		fmt.Fprintf(&sb, "\n## Description\n\n%s\n", v.Description)
	}

	if len(result.Transcript) > 0 {
		sb.WriteString("\n## Transcript\n\n")
		renderTranscript(&sb, result.Transcript)
	}

	return sb.String()
}

func renderTranscript(sb *strings.Builder, segments []TranscriptSegment) {
	// Group text with a timestamp marker roughly every 30 seconds.
	lastTimestamp := -30.0

	for _, seg := range segments {
		if seg.Start-lastTimestamp >= 30.0 {
			fmt.Fprintf(sb, "\n**[%s]**\n", formatTimestamp(seg.Start))
			lastTimestamp = seg.Start
		}
		sb.WriteString(seg.Text)
		sb.WriteString(" ")
	}
	sb.WriteString("\n")
}

func formatTimestamp(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
