package components

import (
	"strings"

	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single tab in the tab bar.
type Tab struct {
	Name   string
	Key    rune
	KeyPos int // position of the shortcut letter in the name (-1 if not in name)
}

// Tabs defines all available tabs.
var Tabs = []Tab{
	{Name: "Overview", Key: 'o', KeyPos: 0},
	{Name: "Costs", Key: 'c', KeyPos: 0},
	{Name: "Sessions", Key: 's', KeyPos: 0},
	{Name: "Breakdown", Key: 'b', KeyPos: 0},
	{Name: "Settings", Key: 'x', KeyPos: -1},
}

// RenderTabBar renders the tab bar with the given active index.
func RenderTabBar(activeIdx int, width int) string {
	t := theme.Active

	activeStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	dimKeyStyle := lipgloss.NewStyle().
		Foreground(t.TextDim)

	parts := make([]string, 0, len(Tabs))
	for i, tab := range Tabs {
		var rendered string
		if i == activeIdx {
			rendered = activeStyle.Render(tab.Name)
		} else {
			// Render with highlighted shortcut key
			if tab.KeyPos >= 0 && tab.KeyPos < len(tab.Name) {
				before := tab.Name[:tab.KeyPos]
				key := string(tab.Name[tab.KeyPos])
				after := tab.Name[tab.KeyPos+1:]
				rendered = inactiveStyle.Render(before) +
					dimKeyStyle.Render("[") + keyStyle.Render(key) + dimKeyStyle.Render("]") +
					inactiveStyle.Render(after)
			} else {
				// Key not in name (e.g., "Settings" with 'x')
				rendered = inactiveStyle.Render(tab.Name) +
					dimKeyStyle.Render("[") + keyStyle.Render(string(tab.Key)) + dimKeyStyle.Render("]")
			}
		}
		parts = append(parts, rendered)
	}

	bar := " " + strings.Join(parts, "  ")
	if lipgloss.Width(bar) <= width {
		return bar
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(bar)
}
