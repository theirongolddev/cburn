package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// Sparkline renders a unicode sparkline from values.
func Sparkline(values []float64, color lipgloss.Color) string {
	if len(values) == 0 {
		return ""
	}
	t := theme.Active

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	peak := values[0]
	for _, v := range values[1:] {
		if v > peak {
			peak = v
		}
	}
	if peak == 0 {
		peak = 1
	}

	style := lipgloss.NewStyle().Foreground(color).Background(t.Surface)

	var buf strings.Builder
	buf.Grow(len(values) * 4) // UTF-8 block chars are up to 3 bytes
	for _, v := range values {
		idx := int(v / peak * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		buf.WriteRune(blocks[idx]) //nolint:gosec // bounds checked above
	}

	return style.Render(buf.String())
}

// BarChart renders a visually polished bar chart with gradient-style coloring.
func BarChart(values []float64, labels []string, color lipgloss.Color, width, height int) string {
	if len(values) == 0 {
		return ""
	}
	if width < 15 || height < 3 {
		return Sparkline(values, color)
	}

	t := theme.Active

	// Find max value
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	// Y-axis: compute tick step and ceiling
	tickStep := chartTickStep(maxVal)
	maxIntervals := height / 2
	if maxIntervals < 2 {
		maxIntervals = 2
	}
	for {
		n := int(math.Ceil(maxVal / tickStep))
		if n <= maxIntervals {
			break
		}
		tickStep *= 2
	}
	ceiling := math.Ceil(maxVal/tickStep) * tickStep
	numIntervals := int(math.Round(ceiling / tickStep))
	if numIntervals < 1 {
		numIntervals = 1
	}

	rowsPerTick := height / numIntervals
	if rowsPerTick < 2 {
		rowsPerTick = 2
	}
	chartH := rowsPerTick * numIntervals

	// Pre-compute tick labels
	yLabelW := len(formatChartLabel(ceiling)) + 1
	if yLabelW < 4 {
		yLabelW = 4
	}
	tickLabels := make(map[int]string)
	for i := 1; i <= numIntervals; i++ {
		row := i * rowsPerTick
		tickLabels[row] = formatChartLabel(tickStep * float64(i))
	}

	// Chart area width
	chartW := width - yLabelW - 1
	if chartW < 5 {
		chartW = 5
	}

	n := len(values)

	// Bar sizing
	gap := 1
	if n <= 1 {
		gap = 0
	}
	barW := 2
	if n > 1 {
		barW = (chartW - (n - 1)) / n
	} else if n == 1 {
		barW = chartW
	}
	if barW < 2 && n > 1 {
		maxN := (chartW + 1) / 3
		if maxN < 2 {
			maxN = 2
		}
		sampled := make([]float64, maxN)
		var sampledLabels []string
		if len(labels) == n {
			sampledLabels = make([]string, maxN)
		}
		for i := range sampled {
			srcIdx := i * (n - 1) / (maxN - 1)
			sampled[i] = values[srcIdx]
			if sampledLabels != nil {
				sampledLabels[i] = labels[srcIdx]
			}
		}
		values = sampled
		labels = sampledLabels
		n = maxN
		barW = 2
	}
	if barW > 6 {
		barW = 6
	}
	axisLen := n*barW + max(0, n-1)*gap

	blocks := []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Multi-color gradient for bars based on height
	axisStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)

	var b strings.Builder

	// Render rows top to bottom
	for row := chartH; row >= 1; row-- {
		rowTop := ceiling * float64(row) / float64(chartH)
		rowBottom := ceiling * float64(row-1) / float64(chartH)
		rowPct := float64(row) / float64(chartH) // How high in the chart (0=bottom, 1=top)

		// Choose bar color based on row height (gradient effect)
		var barColor lipgloss.Color
		switch {
		case rowPct > 0.8:
			barColor = t.AccentBright
		case rowPct > 0.5:
			barColor = color
		default:
			barColor = t.Accent
		}
		barStyle := lipgloss.NewStyle().Foreground(barColor).Background(t.Surface)

		label := tickLabels[row]
		b.WriteString(axisStyle.Render(fmt.Sprintf("%*s", yLabelW, label)))
		b.WriteString(axisStyle.Render("│"))

		for i, v := range values {
			if i > 0 && gap > 0 {
				b.WriteString(lipgloss.NewStyle().Background(t.Surface).Render(strings.Repeat(" ", gap)))
			}
			switch {
			case v >= rowTop:
				b.WriteString(barStyle.Render(strings.Repeat("█", barW)))
			case v > rowBottom:
				frac := (v - rowBottom) / (rowTop - rowBottom)
				idx := int(frac * 8)
				if idx > 8 {
					idx = 8
				}
				if idx < 1 {
					idx = 1
				}
				b.WriteString(barStyle.Render(strings.Repeat(string(blocks[idx]), barW)))
			default:
				b.WriteString(lipgloss.NewStyle().Background(t.Surface).Render(strings.Repeat(" ", barW)))
			}
		}
		b.WriteString("\n")
	}

	// X-axis line with 0 label
	b.WriteString(axisStyle.Render(fmt.Sprintf("%*s", yLabelW, "0")))
	b.WriteString(axisStyle.Render("└"))
	b.WriteString(axisStyle.Render(strings.Repeat("─", axisLen)))

	// X-axis labels
	if len(labels) == n && n > 0 {
		buf := make([]byte, axisLen)
		for i := range buf {
			buf[i] = ' '
		}

		minSpacing := 8
		labelStep := max(1, (n*minSpacing)/(axisLen+1))

		lastEnd := -1
		for i := 0; i < n; i += labelStep {
			pos := i * (barW + gap)
			lbl := labels[i]
			end := pos + len(lbl)
			if pos <= lastEnd {
				continue
			}
			if end > axisLen {
				end = axisLen
				if end-pos < 3 {
					continue
				}
				lbl = lbl[:end-pos]
			}
			copy(buf[pos:end], lbl)
			lastEnd = end + 1
		}
		if n > 1 {
			lbl := labels[n-1]
			pos := (n - 1) * (barW + gap)
			end := pos + len(lbl)
			if end > axisLen {
				pos = axisLen - len(lbl)
				end = axisLen
			}
			if pos >= 0 && pos > lastEnd {
				for j := pos; j < end; j++ {
					buf[j] = ' '
				}
				copy(buf[pos:end], lbl)
			}
		}

		b.WriteString("\n")
		labelStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
		b.WriteString(lipgloss.NewStyle().Background(t.Surface).Render(strings.Repeat(" ", yLabelW+1)))
		b.WriteString(labelStyle.Render(strings.TrimRight(string(buf), " ")))
	}

	return b.String()
}

// chartTickStep computes a nice tick interval targeting ~5 ticks.
func chartTickStep(maxVal float64) float64 {
	if maxVal <= 0 {
		return 1
	}
	rough := maxVal / 5
	exp := math.Floor(math.Log10(rough))
	base := math.Pow(10, exp)
	frac := rough / base

	switch {
	case frac < 1.5:
		return base
	case frac < 3.5:
		return 2 * base
	default:
		return 5 * base
	}
}

func formatChartLabel(v float64) string {
	switch {
	case v >= 1e9:
		if v == math.Trunc(v/1e9)*1e9 {
			return fmt.Sprintf("%.0fB", v/1e9)
		}
		return fmt.Sprintf("%.1fB", v/1e9)
	case v >= 1e6:
		if v == math.Trunc(v/1e6)*1e6 {
			return fmt.Sprintf("%.0fM", v/1e6)
		}
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e3:
		if v == math.Trunc(v/1e3)*1e3 {
			return fmt.Sprintf("%.0fk", v/1e3)
		}
		return fmt.Sprintf("%.1fk", v/1e3)
	case v >= 1:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}
