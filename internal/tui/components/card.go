package components

import (
	"fmt"
	"math"
	"strings"

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

// BarChart renders a multi-row bar chart with anchored Y-axis and optional X-axis labels.
// labels (if non-nil) should correspond 1:1 with values for x-axis display.
// height is a target; actual height adjusts slightly so Y-axis ticks are evenly spaced.
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

	// Y-axis: compute tick step and ceiling, then fit within requested height.
	// Each interval needs at least 2 rows for readable spacing, so
	// maxIntervals = height/2. If the initial step gives too many intervals,
	// double it until they fit.
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

	// Each interval gets the same number of rows; chart height is an exact multiple.
	rowsPerTick := height / numIntervals
	if rowsPerTick < 2 {
		rowsPerTick = 2
	}
	chartH := rowsPerTick * numIntervals

	// Pre-compute tick labels at evenly-spaced row positions
	yLabelW := len(formatChartLabel(ceiling)) + 1
	if yLabelW < 4 {
		yLabelW = 4
	}
	tickLabels := make(map[int]string)
	for i := 1; i <= numIntervals; i++ {
		row := i * rowsPerTick
		tickLabels[row] = formatChartLabel(tickStep * float64(i))
	}

	// Chart area width (excluding y-axis label and axis line char)
	chartW := width - yLabelW - 1
	if chartW < 5 {
		chartW = 5
	}

	n := len(values)

	// Bar sizing: always use 1-char gaps, target barW >= 2.
	// If bars don't fit at width 2, subsample to fewer bars.
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
		// Subsample so bars fit at width 2 with 1-char gaps
		maxN := (chartW + 1) / 3 // each bar = 2 chars + 1 gap (last bar no gap)
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
	barStyle := lipgloss.NewStyle().Foreground(color)
	axisStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	var b strings.Builder

	// Render rows top to bottom using chartH (aligned to tick intervals)
	for row := chartH; row >= 1; row-- {
		rowTop := ceiling * float64(row) / float64(chartH)
		rowBottom := ceiling * float64(row-1) / float64(chartH)

		label := tickLabels[row]
		b.WriteString(axisStyle.Render(fmt.Sprintf("%*s", yLabelW, label)))
		b.WriteString(axisStyle.Render("│"))

		for i, v := range values {
			if i > 0 && gap > 0 {
				b.WriteString(strings.Repeat(" ", gap))
			}
			if v >= rowTop {
				b.WriteString(barStyle.Render(strings.Repeat("█", barW)))
			} else if v > rowBottom {
				frac := (v - rowBottom) / (rowTop - rowBottom)
				idx := int(frac * 8)
				if idx > 8 {
					idx = 8
				}
				if idx < 1 {
					idx = 1
				}
				b.WriteString(barStyle.Render(strings.Repeat(string(blocks[idx]), barW)))
			} else {
				b.WriteString(strings.Repeat(" ", barW))
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

		// Place labels at bar start positions, skip overlaps
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
		// Always place the last label, right-aligned to axis edge if needed.
		// Overwrites any truncated label underneath.
		if n > 1 && len(labels[n-1]) <= axisLen {
			lbl := labels[n-1]
			pos := axisLen - len(lbl)
			end := axisLen
			// Clear the area first in case a truncated label is there
			for j := pos; j < end; j++ {
				buf[j] = ' '
			}
			copy(buf[pos:end], lbl)
		}

		b.WriteString("\n")
		b.WriteString(strings.Repeat(" ", yLabelW+1))
		b.WriteString(axisStyle.Render(strings.TrimRight(string(buf), " ")))
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
