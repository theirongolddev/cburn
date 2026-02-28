// Package components provides reusable TUI widgets for the cburn dashboard.
package components

import (
	"strings"

	"github.com/theirongolddev/cburn/internal/tui/theme"

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

// MetricCard renders a visually striking metric card with icon, colored value, and delta.
// outerWidth is the total rendered width including border.
func MetricCard(label, value, delta string, outerWidth int) string {
	t := theme.Active

	contentWidth := outerWidth - 2 // subtract border
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Determine accent color based on label for variety
	var valueColor lipgloss.Color
	var icon string
	switch {
	case strings.Contains(strings.ToLower(label), "token"):
		valueColor = t.Cyan
		icon = "◈"
	case strings.Contains(strings.ToLower(label), "session"):
		valueColor = t.Magenta
		icon = "◉"
	case strings.Contains(strings.ToLower(label), "cost"):
		valueColor = t.Green
		icon = "◆"
	case strings.Contains(strings.ToLower(label), "cache"):
		valueColor = t.Blue
		icon = "◇"
	default:
		valueColor = t.Accent
		icon = "●"
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		BorderBackground(t.Background).
		Background(t.Surface).
		Width(contentWidth).
		Padding(0, 1)

	iconStyle := lipgloss.NewStyle().
		Foreground(valueColor).
		Background(t.Surface)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface)

	valueStyle := lipgloss.NewStyle().
		Foreground(valueColor).
		Background(t.Surface).
		Bold(true)

	deltaStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.Surface)
	spaceStyle := lipgloss.NewStyle().
		Background(t.Surface)

	// Build content with icon
	content := iconStyle.Render(icon) + spaceStyle.Render(" ") + labelStyle.Render(label) + "\n" +
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

	// Use CardRow instead of JoinHorizontal to ensure proper background fill
	return CardRow(rendered)
}

// ContentCard renders a bordered content card with an optional title.
// outerWidth controls the total rendered width including border.
func ContentCard(title, body string, outerWidth int) string {
	t := theme.Active

	contentWidth := outerWidth - 2 // subtract border chars
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Use accent border for titled cards, subtle for untitled
	borderColor := t.Border
	if title != "" {
		borderColor = t.BorderBright
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(t.Background).
		Background(t.Surface).
		Width(contentWidth).
		Padding(0, 1)

	// Title with accent color and underline effect
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Surface).
		Bold(true)

	content := ""
	if title != "" {
		// Title with subtle separator
		titleLine := titleStyle.Render(title)
		separatorStyle := lipgloss.NewStyle().Foreground(t.Border).Background(t.Surface)
		separator := separatorStyle.Render(strings.Repeat("─", minInt(len(title)+2, contentWidth-2)))
		content = titleLine + "\n" + separator + "\n"
	}
	content += body

	return cardStyle.Render(content)
}

// PanelCard renders a full-width panel with prominent styling - used for main chart areas.
func PanelCard(title, body string, outerWidth int) string {
	t := theme.Active

	contentWidth := outerWidth - 2
	if contentWidth < 10 {
		contentWidth = 10
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderAccent).
		BorderBackground(t.Background).
		Background(t.Surface).
		Width(contentWidth).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.AccentBright).
		Background(t.Surface).
		Bold(true)

	content := ""
	if title != "" {
		content = titleStyle.Render(title) + "\n"
	}
	content += body

	return cardStyle.Render(content)
}

// CardRow joins pre-rendered card strings horizontally with matched heights.
// This manually joins cards line-by-line to ensure shorter cards are padded
// with proper background fill, avoiding black square artifacts.
func CardRow(cards []string) string {
	if len(cards) == 0 {
		return ""
	}

	t := theme.Active

	// Split each card into lines and track widths
	cardLines := make([][]string, len(cards))
	cardWidths := make([]int, len(cards))
	maxHeight := 0

	for i, card := range cards {
		lines := strings.Split(card, "\n")
		cardLines[i] = lines
		if len(lines) > maxHeight {
			maxHeight = len(lines)
		}
		// Determine card width from the first line (cards have consistent width)
		if len(lines) > 0 {
			cardWidths[i] = lipgloss.Width(lines[0])
		}
	}

	// Build background-filled padding style
	bgStyle := lipgloss.NewStyle().Background(t.Background)

	// Pad shorter cards with background-filled lines
	for i := range cardLines {
		for len(cardLines[i]) < maxHeight {
			// Add a line of spaces with the proper background
			padding := bgStyle.Render(strings.Repeat(" ", cardWidths[i]))
			cardLines[i] = append(cardLines[i], padding)
		}
	}

	// Join cards line by line
	var result strings.Builder
	for row := 0; row < maxHeight; row++ {
		for i := range cardLines {
			result.WriteString(cardLines[i][row])
		}
		if row < maxHeight-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
