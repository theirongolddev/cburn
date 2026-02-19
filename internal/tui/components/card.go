package components

import (
	"fmt"

	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// MetricCard renders a small metric card with label, value, and delta.
func MetricCard(label, value, delta string, width int) string {
	t := theme.Active

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(width).
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
func MetricCardRow(cards []struct{ Label, Value, Delta string }, totalWidth int) string {
	if len(cards) == 0 {
		return ""
	}

	cardWidth := (totalWidth - len(cards) - 1) / len(cards)
	if cardWidth < 10 {
		cardWidth = 10
	}

	var rendered []string
	for _, c := range cards {
		rendered = append(rendered, MetricCard(c.Label, c.Value, c.Delta, cardWidth))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// Sparkline renders a unicode sparkline from values.
func Sparkline(values []float64, color lipgloss.Color) string {
	if len(values) == 0 {
		return ""
	}

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}

	style := lipgloss.NewStyle().Foreground(color)

	var result string
	for _, v := range values {
		idx := int(v / max * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		result += string(blocks[idx])
	}

	return style.Render(result)
}

// ProgressBar renders a colored progress bar.
func ProgressBar(pct float64, width int) string {
	t := theme.Active
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledStyle := lipgloss.NewStyle().Foreground(t.Accent)
	emptyStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	bar := ""
	for i := 0; i < filled; i++ {
		bar += filledStyle.Render("█")
	}
	for i := filled; i < width; i++ {
		bar += emptyStyle.Render("░")
	}

	return fmt.Sprintf("%s %.1f%%", bar, pct*100)
}
