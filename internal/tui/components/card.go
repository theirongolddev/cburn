// Package components provides reusable TUI widgets for the cburn dashboard.
package components

import (
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// LayoutRow distributes totalWidth into n widths that sum to exactly totalWidth.
// First items absorb the remainder from integer division.
func LayoutRow(totalWidth, n int) []int {
	if n <= 0 {
		return nil
	}
	base := totalWidth / n
	remainder := totalWidth % n
	widths := make([]int, n)
	for i := range widths {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
	}
	return widths
}

// MetricCard renders a small metric card with label, value, and delta.
// outerWidth is the total rendered width including border.
func MetricCard(label, value, delta string, outerWidth int) string {
	t := theme.Active

	contentWidth := outerWidth - 2 // subtract border
	if contentWidth < 10 {
		contentWidth = 10
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(contentWidth).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	valueStyle := lipgloss.NewStyle().
		Foreground(t.TextPrimary).
		Bold(true)

	deltaStyle := lipgloss.NewStyle().
		Foreground(t.TextDim)

	content := labelStyle.Render(label) + "\n" +
		valueStyle.Render(value)
	if delta != "" {
		content += "\n" + deltaStyle.Render(delta)
	}

	return cardStyle.Render(content)
}

// MetricCardRow renders a row of metric cards side by side.
// totalWidth is the full row width; cards sum to exactly that.
func MetricCardRow(cards []struct{ Label, Value, Delta string }, totalWidth int) string {
	if len(cards) == 0 {
		return ""
	}

	widths := LayoutRow(totalWidth, len(cards))

	var rendered []string
	for i, c := range cards {
		rendered = append(rendered, MetricCard(c.Label, c.Value, c.Delta, widths[i]))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// ContentCard renders a bordered content card with an optional title.
// outerWidth controls the total rendered width including border.
func ContentCard(title, body string, outerWidth int) string {
	t := theme.Active

	contentWidth := outerWidth - 2 // subtract border chars
	if contentWidth < 10 {
		contentWidth = 10
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(contentWidth).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Bold(true)

	content := ""
	if title != "" {
		content = titleStyle.Render(title) + "\n"
	}
	content += body

	return cardStyle.Render(content)
}

// CardRow joins pre-rendered card strings horizontally.
func CardRow(cards []string) string {
	if len(cards) == 0 {
		return ""
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// CardInnerWidth returns the usable text width inside a ContentCard
// given its outer width (subtracts border + padding).
func CardInnerWidth(outerWidth int) int {
	w := outerWidth - 4 // 2 border + 2 padding
	if w < 10 {
		w = 10
	}
	return w
}
