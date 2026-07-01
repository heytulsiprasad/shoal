package ui

import (
	"fmt"
	"strings"
)

// truncate shortens s to at most n runes, adding an ellipsis when it cuts.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:max(0, n)])
	}
	return string(r[:n-1]) + "…"
}

// formatBytes renders a byte count as a compact human string (e.g. "1.4 GiB").
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// asMagnet returns s if it looks like a magnet link, else "".
func asMagnet(s string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "magnet:?") {
		return strings.TrimSpace(s)
	}
	return ""
}

// padOrTrim forces s to exactly w display columns (simple rune-based).
func padOrTrim(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > w {
		return truncate(s, w)
	}
	return s + strings.Repeat(" ", w-len(r))
}
