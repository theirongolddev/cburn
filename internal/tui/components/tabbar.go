package components

import (
	"strings"

	"github.com/theirongolddev/cburn/internal/tui/theme"

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

// TabVisualWidth returns the rendered visual width of a tab.
// This must match RenderTabBar's rendering logic exactly for mouse hit detection.
func TabVisualWidth(tab Tab, isActive bool) int {
	// Active tabs: just the name with padding (1 on each side)
	if isActive {
		return len(tab.Name) + 2
	}

	// Inactive tabs: name with padding, plus "[k]" suffix if shortcut not in name
	w := len(tab.Name) + 2
	if tab.KeyPos < 0 {
		w += 3 // "[k]"
	}
	return w
}

// RenderTabBar renders a modern tab bar with underline-style active indicator.
func RenderTabBar(activeIdx int, width int) string {
	t := theme.Active

	// Container with bottom border
	barStyle := lipgloss.NewStyle().
		Background(t.Surface).
		Width(width)

	// Active tab: bright text with accent underline
	activeTabStyle := lipgloss.NewStyle().
		Foreground(t.AccentBright).
		Background(t.Surface).
		Bold(true).
		Padding(0, 1)

	// Inactive tab: muted text
	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface).
		Padding(0, 1)

	// Key highlight style
	keyStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Surface)

	dimStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.Surface)

	// Separator between tabs
	sepStyle := lipgloss.NewStyle().
		Foreground(t.Border).
		Background(t.Surface)

	var tabParts []string
	var underlineParts []string

	for i, tab := range Tabs {
		var tabContent string
		var underline string

		if i == activeIdx {
			// Active tab - full name, bright
			tabContent = activeTabStyle.Render(tab.Name)
			// Accent underline
			underline = lipgloss.NewStyle().
				Foreground(t.AccentBright).
				Background(t.Surface).
				Render(strings.Repeat("━", lipgloss.Width(tabContent)))
		} else {
			// Inactive tab - show key hint
			if tab.KeyPos >= 0 && tab.KeyPos < len(tab.Name) {
				before := tab.Name[:tab.KeyPos]
				key := string(tab.Name[tab.KeyPos])
				after := tab.Name[tab.KeyPos+1:]
				tabContent = lipgloss.NewStyle().Padding(0, 1).Background(t.Surface).Render(
					dimStyle.Render(before) + keyStyle.Render(key) + dimStyle.Render(after))
			} else {
				tabContent = inactiveTabStyle.Render(tab.Name) +
					dimStyle.Render("[") + keyStyle.Render(string(tab.Key)) + dimStyle.Render("]")
			}
			// Dim underline
			underline = lipgloss.NewStyle().
				Foreground(t.Border).
				Background(t.Surface).
				Render(strings.Repeat("─", lipgloss.Width(tabContent)))
		}

		tabParts = append(tabParts, tabContent)
		underlineParts = append(underlineParts, underline)

		// Add separator between tabs (not after last)
		if i < len(Tabs)-1 {
			tabParts = append(tabParts, sepStyle.Render(" "))
			underlineParts = append(underlineParts, sepStyle.Render(" "))
		}
	}

	// Combine tab row and underline row
	tabRow := strings.Join(tabParts, "")
	underlineRow := strings.Join(underlineParts, "")

	// Fill remaining width with border
	tabRowWidth := lipgloss.Width(tabRow)
	if tabRowWidth < width {
		padding := width - tabRowWidth
		tabRow += lipgloss.NewStyle().Background(t.Surface).Render(strings.Repeat(" ", padding))
		underlineRow += lipgloss.NewStyle().Foreground(t.Border).Background(t.Surface).Render(strings.Repeat("─", padding))
	}

	return barStyle.Render(tabRow + "\n" + underlineRow)
}
