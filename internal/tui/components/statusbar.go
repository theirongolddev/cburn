package components

import (
	"fmt"
	"strings"

	"github.com/theirongolddev/cburn/internal/claudeai"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// RenderStatusBar renders a polished bottom status bar with rate limits and controls.
func RenderStatusBar(width int, dataAge string, subData *claudeai.SubscriptionData, refreshing, autoRefresh bool) string {
	t := theme.Active

	// Main container
	barStyle := lipgloss.NewStyle().
		Background(t.SurfaceHover).
		Width(width)

	// Build left section: keyboard hints
	keyStyle := lipgloss.NewStyle().
		Foreground(t.AccentBright).
		Background(t.SurfaceHover).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.SurfaceHover)

	bracketStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.SurfaceHover)
	spaceStyle := lipgloss.NewStyle().
		Background(t.SurfaceHover)

	left := spaceStyle.Render(" ") +
		bracketStyle.Render("[") + keyStyle.Render("?") + bracketStyle.Render("]") + hintStyle.Render("help") + spaceStyle.Render("  ") +
		bracketStyle.Render("[") + keyStyle.Render("r") + bracketStyle.Render("]") + hintStyle.Render("efresh") + spaceStyle.Render("  ") +
		bracketStyle.Render("[") + keyStyle.Render("q") + bracketStyle.Render("]") + hintStyle.Render("uit")

	// Build middle section: rate limit indicators
	middle := renderStatusRateLimits(subData)

	// Build right section: refresh status
	var right string
	if refreshing {
		spinnerStyle := lipgloss.NewStyle().
			Foreground(t.AccentBright).
			Background(t.SurfaceHover).
			Bold(true)
		right = spinnerStyle.Render("↻ refreshing")
	} else if dataAge != "" {
		refreshIcon := ""
		if autoRefresh {
			refreshIcon = lipgloss.NewStyle().
				Foreground(t.Green).
				Background(t.SurfaceHover).
				Render("↻ ")
		}
		dataStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Background(t.SurfaceHover)
		right = refreshIcon + dataStyle.Render("Data: "+dataAge)
	}
	right += spaceStyle.Render(" ")

	// Calculate padding
	leftWidth := lipgloss.Width(left)
	middleWidth := lipgloss.Width(middle)
	rightWidth := lipgloss.Width(right)

	totalUsed := leftWidth + middleWidth + rightWidth
	padding := width - totalUsed
	if padding < 0 {
		padding = 0
	}

	leftPad := padding / 2
	rightPad := padding - leftPad

	paddingStyle := lipgloss.NewStyle().Background(t.SurfaceHover)
	bar := left +
		paddingStyle.Render(strings.Repeat(" ", leftPad)) +
		middle +
		paddingStyle.Render(strings.Repeat(" ", rightPad)) +
		right

	return barStyle.Render(bar)
}

// renderStatusRateLimits renders compact rate limit pills for the status bar.
func renderStatusRateLimits(subData *claudeai.SubscriptionData) string {
	if subData == nil || subData.Usage == nil {
		return ""
	}

	t := theme.Active

	var parts []string

	if w := subData.Usage.FiveHour; w != nil {
		parts = append(parts, renderRatePill("5h", w.Pct))
	}
	if w := subData.Usage.SevenDay; w != nil {
		parts = append(parts, renderRatePill("Wk", w.Pct))
	}

	if len(parts) == 0 {
		return ""
	}

	sepStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.SurfaceHover)

	return strings.Join(parts, sepStyle.Render(" │ "))
}

// renderRatePill renders a compact, colored rate indicator pill.
func renderRatePill(label string, pct float64) string {
	t := theme.Active

	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	// Choose color based on usage level
	var barColor, pctColor lipgloss.Color
	switch {
	case pct >= 0.9:
		barColor = t.Red
		pctColor = t.Red
	case pct >= 0.7:
		barColor = t.Orange
		pctColor = t.Orange
	case pct >= 0.5:
		barColor = t.Yellow
		pctColor = t.Yellow
	default:
		barColor = t.Green
		pctColor = t.Green
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.SurfaceHover)

	barStyle := lipgloss.NewStyle().
		Foreground(barColor).
		Background(t.SurfaceHover)

	emptyStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.SurfaceHover)

	pctStyle := lipgloss.NewStyle().
		Foreground(pctColor).
		Background(t.SurfaceHover).
		Bold(true)

	// Render mini bar (8 chars)
	barW := 8
	filled := int(pct * float64(barW))
	if filled > barW {
		filled = barW
	}

	bar := barStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", barW-filled))

	spaceStyle := lipgloss.NewStyle().
		Background(t.SurfaceHover)

	return labelStyle.Render(label+" ") + bar + spaceStyle.Render(" ") + pctStyle.Render(fmt.Sprintf("%2.0f%%", pct*100))
}
