package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Theme colors (Flexoki Dark)
var (
	ColorBg        = lipgloss.Color("#100F0F")
	ColorSurface   = lipgloss.Color("#1C1B1A")
	ColorBorder    = lipgloss.Color("#282726")
	ColorTextDim   = lipgloss.Color("#575653")
	ColorTextMuted = lipgloss.Color("#6F6E69")
	ColorText      = lipgloss.Color("#FFFCF0")
	ColorAccent    = lipgloss.Color("#3AA99F")
	ColorGreen     = lipgloss.Color("#879A39")
	ColorOrange    = lipgloss.Color("#DA702C")
	ColorRed       = lipgloss.Color("#D14D41")
	ColorBlue      = lipgloss.Color("#4385BE")
	ColorPurple    = lipgloss.Color("#8B7EC8")
	ColorYellow    = lipgloss.Color("#D0A215")
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

	mutedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	dimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)
)

// Table represents a bordered text table for CLI output.
type Table struct {
	Title   string
	Headers []string
	Rows    [][]string
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

// RenderTable renders a bordered table with headers and rows using lipgloss/table.
func RenderTable(t Table) string {
	if len(t.Rows) == 0 && len(t.Headers) == 0 {
		return ""
	}

	// Filter out "---" separator sentinels (not supported by lipgloss/table).
	rows := make([][]string, 0, len(t.Rows))
	for _, row := range t.Rows {
		if len(row) == 1 && row[0] == "---" {
			continue
		}
		rows = append(rows, row)
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(dimStyle).
		BorderColumn(true).
		BorderHeader(true).
		Headers(t.Headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(ColorAccent)
			}
			s = s.Foreground(ColorText)
			if col > 0 {
				s = s.Align(lipgloss.Right)
			}
			return s
		})

	var b strings.Builder
	if t.Title != "" {
		b.WriteString("  ")
		b.WriteString(headerStyle.Render(t.Title))
		b.WriteString("\n")
	}
	b.WriteString(tbl.Render())
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

	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	var b strings.Builder
	for _, v := range values {
		idx := int(v / maxVal * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx]) //nolint:gosec // bounds checked above
	}

	return b.String()
}

// RenderHorizontalBar renders a horizontal bar chart entry.
func RenderHorizontalBar(label string, value, maxValue float64, maxWidth int) string {
	if maxValue <= 0 {
		return "  " + label
	}
	barLen := int(value / maxValue * float64(maxWidth))
	if barLen < 0 {
		barLen = 0
	}
	bar := strings.Repeat("█", barLen)
	return "  " + bar
}
