package components

import (
	"fmt"
	"strings"

	"github.com/theirongolddev/cburn/internal/claudeai"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// RenderStatusBar renders the bottom status bar with optional rate limit indicators.
func RenderStatusBar(width int, dataAge string, subData *claudeai.SubscriptionData, refreshing, autoRefresh bool) string {
	t := theme.Active

	style := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(width)

	left := " [?]help  [r]efresh  [q]uit"

	// Build rate limit indicators for the middle section
	ratePart := renderStatusRateLimits(subData)

	// Build right side with refresh status
	var right string
	if refreshing {
		refreshStyle := lipgloss.NewStyle().Foreground(t.Accent)
		right = refreshStyle.Render("↻ refreshing ")
	} else if dataAge != "" {
		autoStr := ""
		if autoRefresh {
			autoStr = "↻ "
		}
		right = fmt.Sprintf("%sData: %s ", autoStr, dataAge)
	}

	// Layout: left + ratePart + right, with padding distributed
	usedWidth := lipgloss.Width(left) + lipgloss.Width(ratePart) + lipgloss.Width(right)
	padding := width - usedWidth
	if padding < 0 {
		padding = 0
	}

	// Split padding: more on the left side of rate indicators
	leftPad := padding / 2
	rightPad := padding - leftPad

	bar := left + strings.Repeat(" ", leftPad) + ratePart + strings.Repeat(" ", rightPad) + right

	return style.Render(bar)
}

// renderStatusRateLimits renders compact rate limit bars for the status bar.
func renderStatusRateLimits(subData *claudeai.SubscriptionData) string {
	if subData == nil || subData.Usage == nil {
		return ""
	}

	t := theme.Active
	sepStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	var parts []string

	if w := subData.Usage.FiveHour; w != nil {
		parts = append(parts, compactStatusBar("5h", w.Pct))
	}
	if w := subData.Usage.SevenDay; w != nil {
		parts = append(parts, compactStatusBar("Wk", w.Pct))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, sepStyle.Render(" | "))
}

// compactStatusBar renders a tiny inline progress indicator for the status bar.
// Format: "5h ████░░░░ 42%"
func compactStatusBar(label string, pct float64) string {
	t := theme.Active

	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	barW := 8
	bar := progress.New(
		progress.WithSolidFill(ColorForPct(pct)),
		progress.WithWidth(barW),
		progress.WithoutPercentage(),
	)
	bar.EmptyColor = string(t.TextDim)

	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	pctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForPct(pct)))

	return fmt.Sprintf("%s %s %s",
		labelStyle.Render(label),
		bar.ViewAs(pct),
		pctStyle.Render(fmt.Sprintf("%2.0f%%", pct*100)),
	)
}
