package components

import (
	"strings"

	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single tab in the tab bar.
type Tab struct {
	Name    string
	Key     rune
	KeyPos  int // position of the shortcut letter in the name (-1 if not in name)
}

// Tabs defines all available tabs.
var Tabs = []Tab{
	{Name: "Dashboard", Key: 'd', KeyPos: 0},
	{Name: "Costs", Key: 'c', KeyPos: 0},
	{Name: "Sessions", Key: 's', KeyPos: 0},
	{Name: "Models", Key: 'm', KeyPos: 0},
	{Name: "Projects", Key: 'p', KeyPos: 0},
	{Name: "Trends", Key: 't', KeyPos: 0},
	{Name: "Efficiency", Key: 'e', KeyPos: 0},
	{Name: "Activity", Key: 'a', KeyPos: 0},
	{Name: "Budget", Key: 'b', KeyPos: 0},
	{Name: "Settings", Key: 'x', KeyPos: -1}, // x is not in "Settings"
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

	var parts []string
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

	// Split into two rows if needed
	row1 := strings.Join(parts[:5], "  ")
	row2 := strings.Join(parts[5:], "  ")

	return " " + row1 + "\n " + row2
}

// TabIdxByKey returns the tab index for a given key press, or -1.
func TabIdxByKey(key rune) int {
	for i, tab := range Tabs {
		if tab.Key == key {
			return i
		}
	}
	return -1
}
