package components

import "strings"

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TruncateLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}

	return strings.Join(lines[:maxLines], "\n") + "…"
}
