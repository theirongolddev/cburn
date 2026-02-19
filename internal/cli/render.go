package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme colors (Flexoki Dark)
var (
	ColorBg       = lipgloss.Color("#100F0F")
	ColorSurface  = lipgloss.Color("#1C1B1A")
	ColorBorder   = lipgloss.Color("#282726")
	ColorTextDim  = lipgloss.Color("#575653")
	ColorTextMuted = lipgloss.Color("#6F6E69")
	ColorText     = lipgloss.Color("#FFFCF0")
	ColorAccent   = lipgloss.Color("#3AA99F")
	ColorGreen    = lipgloss.Color("#879A39")
	ColorOrange   = lipgloss.Color("#DA702C")
	ColorRed      = lipgloss.Color("#D14D41")
	ColorBlue     = lipgloss.Color("#4385BE")
	ColorPurple   = lipgloss.Color("#8B7EC8")
	ColorYellow   = lipgloss.Color("#D0A215")
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Align(lipgloss.Center)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	valueStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	mutedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	costStyle = lipgloss.NewStyle().
			Foreground(ColorGreen)

	tokenStyle = lipgloss.NewStyle().
			Foreground(ColorBlue)

	warnStyle = lipgloss.NewStyle().
			Foreground(ColorOrange)

	dimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)
)

// Table represents a bordered text table for CLI output.
type Table struct {
	Title   string
	Headers []string
	Rows    [][]string
	Widths  []int // optional column widths, auto-calculated if nil
}

// RenderTitle renders a centered title bar in a bordered box.
func RenderTitle(title string) string {
	width := 55
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Width(width).
		Align(lipgloss.Center).
		Padding(0, 1)

	return border.Render(titleStyle.Render(title))
}

// RenderTable renders a bordered table with headers and rows.
func RenderTable(t Table) string {
	if len(t.Rows) == 0 && len(t.Headers) == 0 {
		return ""
	}

	// Calculate column widths
	numCols := len(t.Headers)
	if numCols == 0 && len(t.Rows) > 0 {
		numCols = len(t.Rows[0])
	}

	widths := make([]int, numCols)
	if t.Widths != nil {
		copy(widths, t.Widths)
	} else {
		for i, h := range t.Headers {
			if len(h) > widths[i] {
				widths[i] = len(h)
			}
		}
		for _, row := range t.Rows {
			for i, cell := range row {
				if i < numCols && len(cell) > widths[i] {
					widths[i] = len(cell)
				}
			}
		}
	}

	var b strings.Builder

	// Title above table if present
	if t.Title != "" {
		b.WriteString("  ")
		b.WriteString(headerStyle.Render(t.Title))
		b.WriteString("\n")
	}

	totalWidth := 1 // left border
	for _, w := range widths {
		totalWidth += w + 3 // padding + separator
	}

	// Top border
	b.WriteString(dimStyle.Render("╭"))
	for i, w := range widths {
		b.WriteString(dimStyle.Render(strings.Repeat("─", w+2)))
		if i < numCols-1 {
			b.WriteString(dimStyle.Render("┬"))
		}
	}
	b.WriteString(dimStyle.Render("╮"))
	b.WriteString("\n")

	// Header row
	if len(t.Headers) > 0 {
		b.WriteString(dimStyle.Render("│"))
		for i, h := range t.Headers {
			w := widths[i]
			padded := fmt.Sprintf(" %-*s ", w, h)
			b.WriteString(headerStyle.Render(padded))
			if i < numCols-1 {
				b.WriteString(dimStyle.Render("│"))
			}
		}
		b.WriteString(dimStyle.Render("│"))
		b.WriteString("\n")

		// Header separator
		b.WriteString(dimStyle.Render("├"))
		for i, w := range widths {
			b.WriteString(dimStyle.Render(strings.Repeat("─", w+2)))
			if i < numCols-1 {
				b.WriteString(dimStyle.Render("┼"))
			}
		}
		b.WriteString(dimStyle.Render("┤"))
		b.WriteString("\n")
	}

	// Data rows
	for _, row := range t.Rows {
		if len(row) == 1 && row[0] == "---" {
			// Separator row
			b.WriteString(dimStyle.Render("├"))
			for i, w := range widths {
				b.WriteString(dimStyle.Render(strings.Repeat("─", w+2)))
				if i < numCols-1 {
					b.WriteString(dimStyle.Render("┼"))
				}
			}
			b.WriteString(dimStyle.Render("┤"))
			b.WriteString("\n")
			continue
		}

		b.WriteString(dimStyle.Render("│"))
		for i := 0; i < numCols; i++ {
			w := widths[i]
			cell := ""
			if i < len(row) {
				cell = row[i]
			}

			// Right-align numeric columns (all except first)
			var padded string
			if i == 0 {
				padded = fmt.Sprintf(" %-*s ", w, cell)
			} else {
				padded = fmt.Sprintf(" %*s ", w, cell)
			}
			b.WriteString(valueStyle.Render(padded))
			if i < numCols-1 {
				b.WriteString(dimStyle.Render("│"))
			}
		}
		b.WriteString(dimStyle.Render("│"))
		b.WriteString("\n")
	}

	// Bottom border
	b.WriteString(dimStyle.Render("╰"))
	for i, w := range widths {
		b.WriteString(dimStyle.Render(strings.Repeat("─", w+2)))
		if i < numCols-1 {
			b.WriteString(dimStyle.Render("┴"))
		}
	}
	b.WriteString(dimStyle.Render("╯"))
	b.WriteString("\n")

	return b.String()
}

// RenderProgressBar renders a simple text progress bar.
func RenderProgressBar(current, total int, width int) string {
	if total <= 0 {
		return ""
	}

	pct := float64(current) / float64(total)
	if pct > 1 {
		pct = 1
	}

	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %s/%s",
		mutedStyle.Render(bar),
		FormatNumber(int64(current)),
		FormatNumber(int64(total)),
	)
}

// RenderSparkline generates a unicode block sparkline from a series of values.
func RenderSparkline(values []float64) string {
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

	var b strings.Builder
	for _, v := range values {
		idx := int(v / max * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx])
	}

	return b.String()
}

// RenderHorizontalBar renders a horizontal bar chart entry.
func RenderHorizontalBar(label string, value, maxValue float64, maxWidth int) string {
	if maxValue <= 0 {
		return fmt.Sprintf("  %s", label)
	}
	barLen := int(value / maxValue * float64(maxWidth))
	if barLen < 0 {
		barLen = 0
	}
	bar := strings.Repeat("█", barLen)
	return fmt.Sprintf("  %s", bar)
}
