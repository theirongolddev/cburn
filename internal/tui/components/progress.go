package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// ProgressBar renders a visually appealing progress bar with percentage.
func ProgressBar(pct float64, width int) string {
	t := theme.Active
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	// Color gradient based on progress
	var barColor lipgloss.Color
	switch {
	case pct >= 0.8:
		barColor = t.AccentBright
	case pct >= 0.5:
		barColor = t.Accent
	default:
		barColor = t.Cyan
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor).Background(t.Surface)
	emptyStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
	pctStyle := lipgloss.NewStyle().Foreground(barColor).Background(t.Surface).Bold(true)
	spaceStyle := lipgloss.NewStyle().Background(t.Surface)

	var b strings.Builder
	b.WriteString(filledStyle.Render(strings.Repeat("█", filled)))
	b.WriteString(emptyStyle.Render(strings.Repeat("░", width-filled)))

	return b.String() + spaceStyle.Render(" ") + pctStyle.Render(fmt.Sprintf("%.0f%%", pct*100))
}

// ColorForPct returns green/yellow/orange/red based on utilization level.
func ColorForPct(pct float64) string {
	t := theme.Active
	switch {
	case pct >= 0.9:
		return string(t.Red)
	case pct >= 0.7:
		return string(t.Orange)
	case pct >= 0.5:
		return string(t.Yellow)
	default:
		return string(t.Green)
	}
}

// RateLimitBar renders a labeled progress bar with percentage and countdown.
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

	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	pctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForPct(pct))).Background(t.Surface).Bold(true)
	countdownStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
	spaceStyle := lipgloss.NewStyle().Background(t.Surface)

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

	return labelStyle.Render(fmt.Sprintf("%-*s", labelW, label)) +
		spaceStyle.Render(" ") +
		bar.ViewAs(pct) +
		spaceStyle.Render(" ") +
		pctStyle.Render(pctStr) +
		spaceStyle.Render("  ") +
		countdownStyle.Render(countdown)
}

// CompactRateBar renders a tiny status-bar-sized rate indicator.
func CompactRateBar(label string, pct float64, width int) string {
	t := theme.Active

	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

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

	pctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForPct(pct))).Background(t.Surface).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	spaceStyle := lipgloss.NewStyle().Background(t.Surface)

	return labelStyle.Render(label) +
		spaceStyle.Render(" ") +
		bar.ViewAs(pct) +
		spaceStyle.Render(" ") +
		pctStyle.Render(fmt.Sprintf("%2.0f%%", pct*100))
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
