package tui

import (
	"fmt"
	"sort"
	"strings"

	"cburn/internal/cli"
	"cburn/internal/config"
	"cburn/internal/model"
	"cburn/internal/tui/components"
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// SessionsView modes — split is iota (0) so it's the default zero value.
const (
	sessViewSplit  = iota // List + full detail side by side (default)
	sessViewDetail        // Full-screen detail
)

// sessionsState holds the sessions tab state.
type sessionsState struct {
	cursor   int
	viewMode int
	offset   int // scroll offset for the list
}

func (a App) renderSessionsContent(filtered []model.SessionStats, cw, h int) string {
	t := theme.Active
	ss := a.sessState

	if len(filtered) == 0 {
		return components.ContentCard("Sessions", lipgloss.NewStyle().Foreground(t.TextMuted).Render("No sessions found"), cw)
	}

	switch ss.viewMode {
	case sessViewDetail:
		return a.renderSessionDetail(filtered, cw, h)
	default:
		return a.renderSessionsSplit(filtered, cw, h)
	}
}

func (a App) renderSessionsSplit(sessions []model.SessionStats, cw, h int) string {
	t := theme.Active
	ss := a.sessState

	if ss.cursor >= len(sessions) {
		return ""
	}

	leftW := cw / 3
	if leftW < 30 {
		leftW = 30
	}
	rightW := cw - leftW

	// Left pane: condensed session list
	leftInner := components.CardInnerWidth(leftW)

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	selectedStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	var leftBody strings.Builder
	visible := h - 6 // card border (2) + header row (2) + footer hint (2)
	if visible < 5 {
		visible = 5
	}

	offset := ss.offset
	if ss.cursor < offset {
		offset = ss.cursor
	}
	if ss.cursor >= offset+visible {
		offset = ss.cursor - visible + 1
	}

	end := offset + visible
	if end > len(sessions) {
		end = len(sessions)
	}

	for i := offset; i < end; i++ {
		s := sessions[i]
		startStr := ""
		if !s.StartTime.IsZero() {
			startStr = s.StartTime.Local().Format("Jan 02 15:04")
		}
		dur := cli.FormatDuration(s.DurationSecs)

		line := fmt.Sprintf("%-13s %s", startStr, dur)
		if len(line) > leftInner {
			line = line[:leftInner]
		}

		if i == ss.cursor {
			leftBody.WriteString(selectedStyle.Render(line))
		} else {
			leftBody.WriteString(rowStyle.Render(line))
		}
		leftBody.WriteString("\n")
	}

	leftCard := components.ContentCard(fmt.Sprintf("Sessions [%dd]", a.days), leftBody.String(), leftW)

	// Right pane: full session detail
	sel := sessions[ss.cursor]
	rightBody := a.renderDetailBody(sel, rightW, headerStyle, mutedStyle)

	titleStr := fmt.Sprintf("Session %s", shortID(sel.SessionID))
	rightCard := components.ContentCard(titleStr, rightBody, rightW)

	return components.CardRow([]string{leftCard, rightCard})
}

func (a App) renderSessionDetail(sessions []model.SessionStats, cw, h int) string {
	t := theme.Active
	ss := a.sessState

	if ss.cursor >= len(sessions) {
		return ""
	}
	sel := sessions[ss.cursor]

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	body := a.renderDetailBody(sel, cw, headerStyle, mutedStyle)

	title := fmt.Sprintf("Session %s", shortID(sel.SessionID))
	return components.ContentCard(title, body, cw)
}

// renderDetailBody generates the full detail content for a session.
// Used by both the split right pane and the full-screen detail view.
func (a App) renderDetailBody(sel model.SessionStats, w int, headerStyle, mutedStyle lipgloss.Style) string {
	t := theme.Active
	innerW := components.CardInnerWidth(w)

	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	greenStyle := lipgloss.NewStyle().Foreground(t.Green)

	var body strings.Builder
	body.WriteString(mutedStyle.Render(sel.Project))
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
	body.WriteString("\n\n")

	// Duration line
	if !sel.StartTime.IsZero() {
		durStr := cli.FormatDuration(sel.DurationSecs)
		timeStr := sel.StartTime.Local().Format("15:04:05")
		if !sel.EndTime.IsZero() {
			timeStr += " - " + sel.EndTime.Local().Format("15:04:05")
		}
		timeStr += " " + sel.StartTime.Local().Format("MST")
		body.WriteString(fmt.Sprintf("%s %s (%s)\n",
			labelStyle.Render("Duration:"),
			valueStyle.Render(durStr),
			mutedStyle.Render(timeStr)))
	}

	ratio := 0.0
	if sel.UserMessages > 0 {
		ratio = float64(sel.APICalls) / float64(sel.UserMessages)
	}
	body.WriteString(fmt.Sprintf("%s %s    %s %s    %s %.1fx\n\n",
		labelStyle.Render("Prompts:"), valueStyle.Render(cli.FormatNumber(int64(sel.UserMessages))),
		labelStyle.Render("API Calls:"), valueStyle.Render(cli.FormatNumber(int64(sel.APICalls))),
		labelStyle.Render("Ratio:"), ratio))

	// Token breakdown table
	body.WriteString(headerStyle.Render("TOKEN BREAKDOWN"))
	body.WriteString("\n")
	body.WriteString(headerStyle.Render(fmt.Sprintf("%-20s %12s %10s", "Type", "Tokens", "Cost")))
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render(strings.Repeat("─", 44)))
	body.WriteString("\n")

	// Calculate per-type costs (aggregate across models)
	inputCost := 0.0
	outputCost := 0.0
	cache5mCost := 0.0
	cache1hCost := 0.0
	cacheReadCost := 0.0
	savings := 0.0

	for modelName, mu := range sel.Models {
		p, ok := config.LookupPricing(modelName)
		if ok {
			inputCost += float64(mu.InputTokens) * p.InputPerMTok / 1e6
			outputCost += float64(mu.OutputTokens) * p.OutputPerMTok / 1e6
			cache5mCost += float64(mu.CacheCreation5mTokens) * p.CacheWrite5mPerMTok / 1e6
			cache1hCost += float64(mu.CacheCreation1hTokens) * p.CacheWrite1hPerMTok / 1e6
			cacheReadCost += float64(mu.CacheReadTokens) * p.CacheReadPerMTok / 1e6
			savings += config.CalculateCacheSavings(modelName, mu.CacheReadTokens)
		}
	}

	rows := []struct {
		typ    string
		tokens int64
		cost   float64
	}{
		{"Input", sel.InputTokens, inputCost},
		{"Output", sel.OutputTokens, outputCost},
		{"Cache Write (5m)", sel.CacheCreation5mTokens, cache5mCost},
		{"Cache Write (1h)", sel.CacheCreation1hTokens, cache1hCost},
		{"Cache Read", sel.CacheReadTokens, cacheReadCost},
	}

	for _, r := range rows {
		if r.tokens == 0 {
			continue
		}
		body.WriteString(valueStyle.Render(fmt.Sprintf("%-20s %12s %10s",
			r.typ,
			cli.FormatTokens(r.tokens),
			cli.FormatCost(r.cost))))
		body.WriteString("\n")
	}

	body.WriteString(mutedStyle.Render(strings.Repeat("─", 44)))
	body.WriteString("\n")
	body.WriteString(fmt.Sprintf("%-20s %12s %10s\n",
		valueStyle.Render("Net Cost"),
		"",
		greenStyle.Render(cli.FormatCost(sel.EstimatedCost))))
	body.WriteString(fmt.Sprintf("%-20s %12s %10s\n",
		labelStyle.Render("Cache Savings"),
		"",
		greenStyle.Render(cli.FormatCost(savings))))

	// Model breakdown
	if len(sel.Models) > 0 {
		body.WriteString("\n")
		body.WriteString(headerStyle.Render("API CALLS BY MODEL"))
		body.WriteString("\n")
		body.WriteString(headerStyle.Render(fmt.Sprintf("%-14s %7s %10s %10s %8s", "Model", "Calls", "Input", "Output", "Cost")))
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render(strings.Repeat("─", 52)))
		body.WriteString("\n")

		// Sort model names for deterministic display order
		modelNames := make([]string, 0, len(sel.Models))
		for name := range sel.Models {
			modelNames = append(modelNames, name)
		}
		sort.Strings(modelNames)

		for _, modelName := range modelNames {
			mu := sel.Models[modelName]
			body.WriteString(valueStyle.Render(fmt.Sprintf("%-14s %7s %10s %10s %8s",
				shortModel(modelName),
				cli.FormatNumber(int64(mu.APICalls)),
				cli.FormatTokens(mu.InputTokens),
				cli.FormatTokens(mu.OutputTokens),
				cli.FormatCost(mu.EstimatedCost))))
			body.WriteString("\n")
		}
	}

	if sel.IsSubagent {
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render("(subagent session)"))
		body.WriteString("\n")
	}

	body.WriteString("\n")
	body.WriteString(mutedStyle.Render("[Enter] expand  [j/k] navigate  [q] quit"))

	return body.String()
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
