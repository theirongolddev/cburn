package components

import (
	"fmt"
	"strings"
	"time"

	"cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

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

	var b strings.Builder
	for i := 0; i < filled; i++ {
		b.WriteString(filledStyle.Render("\u2588"))
	}
	for i := filled; i < width; i++ {
		b.WriteString(emptyStyle.Render("\u2591"))
	}

	return fmt.Sprintf("%s %.1f%%", b.String(), pct*100)
}

// ColorForPct returns green/yellow/red based on utilization level.
func ColorForPct(pct float64) string {
	t := theme.Active
	switch {
	case pct >= 0.8:
		return string(t.Red)
	case pct >= 0.5:
		return string(t.Orange)
	default:
		return string(t.Green)
	}
}

// RateLimitBar renders a labeled progress bar with percentage and countdown.
// label: "5-hour", "Weekly", etc.  pct: 0.0-1.0.  resetsAt: zero means no countdown.
// barWidth: width allocated for the progress bar portion only.
func RateLimitBar(label string, pct float64, resetsAt time.Time, labelW, barWidth int) string {
	t := theme.Active

	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	bar := progress.New(
		progress.WithSolidFill(ColorForPct(pct)),
		progress.WithWidth(barWidth),
		progress.WithoutPercentage(),
	)
	bar.EmptyColor = string(t.TextDim)

	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	pctStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Bold(true)
	countdownStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	pctStr := fmt.Sprintf("%3.0f%%", pct*100)
	countdown := ""
	if !resetsAt.IsZero() {
		dur := time.Until(resetsAt)
		if dur > 0 {
			countdown = formatCountdown(dur)
		} else {
			countdown = "now"
		}
	}

	return fmt.Sprintf("%s %s %s  %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelW, label)),
		bar.ViewAs(pct),
		pctStyle.Render(pctStr),
		countdownStyle.Render(countdown),
	)
}

// CompactRateBar renders a tiny status-bar-sized rate indicator.
// Example output: "5h ████░░░░ 42%"
func CompactRateBar(label string, pct float64, width int) string {
	t := theme.Active

	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	// label + space + bar + space + pct(4 chars)
	barW := width - lipgloss.Width(label) - 6
	if barW < 4 {
		barW = 4
	}

	bar := progress.New(
		progress.WithSolidFill(ColorForPct(pct)),
		progress.WithWidth(barW),
		progress.WithoutPercentage(),
	)
	bar.EmptyColor = string(t.TextDim)

	pctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForPct(pct)))
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	return fmt.Sprintf("%s %s %s",
		labelStyle.Render(label),
		bar.ViewAs(pct),
		pctStyle.Render(fmt.Sprintf("%2.0f%%", pct*100)),
	)
}

func formatCountdown(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h >= 24 {
		days := h / 24
		hours := h % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
