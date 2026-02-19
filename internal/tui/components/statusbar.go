package components

import (
	"fmt"

	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// RenderStatusBar renders the bottom status bar.
func RenderStatusBar(width int, dataAge string) string {
	t := theme.Active

	style := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(width)

	left := " [?]help  [q]uit"
	right := ""
	if dataAge != "" {
		right = fmt.Sprintf("Data: %s ", dataAge)
	}

	// Pad middle
	padding := width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}

	bar := left
	for i := 0; i < padding; i++ {
		bar += " "
	}
	bar += right

	return style.Render(bar)
}
