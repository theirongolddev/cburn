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

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// SessionsView modes — split is iota (0) so it's the default zero value.
const (
	sessViewSplit  = iota // List + full detail side by side (default)
	sessViewDetail        // Full-screen detail
)

// sessionsState holds the sessions tab state.
type sessionsState struct {
	cursor       int
	viewMode     int
	offset       int // scroll offset for the list
	detailScroll int // scroll offset for the detail pane

	// Search/filter state
	searching   bool            // true when search input is active
	searchInput textinput.Model // the search text input
	searchQuery string          // the applied search filter
}

// newSearchInput creates a configured text input for session search.
func newSearchInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "search by project, cost, tokens..."
	ti.CharLimit = 100
	ti.Width = 40
	return ti
}

// filterSessionsBySearch returns sessions matching the search query.
// Matches against project name and formats cost/tokens for numeric searches.
func filterSessionsBySearch(sessions []model.SessionStats, query string) []model.SessionStats {
	if query == "" {
		return sessions
	}
	query = strings.ToLower(query)
	var result []model.SessionStats
	for _, s := range sessions {
		// Match project name
		if strings.Contains(strings.ToLower(s.Project), query) {
			result = append(result, s)
			continue
		}
		// Match session ID prefix
		if strings.Contains(strings.ToLower(s.SessionID), query) {
			result = append(result, s)
			continue
		}
		// Match cost (e.g., "$0.50" or "0.5")
		costStr := cli.FormatCost(s.EstimatedCost)
		if strings.Contains(strings.ToLower(costStr), query) {
			result = append(result, s)
			continue
		}
	}
	return result
}

func (a App) renderSessionsContent(filtered []model.SessionStats, cw, h int) string {
	t := theme.Active
	ss := a.sessState

	if len(filtered) == 0 {
		return components.ContentCard("Sessions", lipgloss.NewStyle().Foreground(t.TextMuted).Render("No sessions found"), cw)
	}

	// Force single-pane detail mode in compact layouts.
	if cw < compactWidth {
		return a.renderSessionDetail(filtered, cw, h)
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

	leftW := cw / 4
	if leftW < 36 {
		leftW = 36
	}
	minRightW := 50
	maxLeftW := cw - minRightW
	if maxLeftW < 20 {
		return a.renderSessionDetail(sessions, cw, h)
	}
	if leftW > maxLeftW {
		leftW = maxLeftW
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
		costStr := cli.FormatCost(s.EstimatedCost)

		// Build left portion (date + duration) and right-align cost
		leftPart := fmt.Sprintf("%-13s %s", startStr, dur)
		padN := leftInner - len(leftPart) - len(costStr)
		if padN < 1 {
			padN = 1
		}

		if i == ss.cursor {
			fullLine := leftPart + strings.Repeat(" ", padN) + costStr
			// Pad to full width for continuous highlight background
			if len(fullLine) < leftInner {
				fullLine += strings.Repeat(" ", leftInner-len(fullLine))
			}
			leftBody.WriteString(selectedStyle.Render(fullLine))
		} else {
			leftBody.WriteString(
				mutedStyle.Render(fmt.Sprintf("%-13s", startStr)) + " " +
					rowStyle.Render(dur) +
					strings.Repeat(" ", padN) +
					mutedStyle.Render(costStr))
		}
		leftBody.WriteString("\n")
	}

	leftCard := components.ContentCard(fmt.Sprintf("Sessions [%dd]", a.days), leftBody.String(), leftW)

	// Right pane: full session detail with scroll support
	sel := sessions[ss.cursor]
	rightBody := a.renderDetailBody(sel, rightW, headerStyle, mutedStyle)

	// Apply detail scroll offset
	rightBody = a.applyDetailScroll(rightBody, h-4) // card border (2) + title (1) + gap (1)

	titleStr := "Session " + shortID(sel.SessionID)
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
	body = a.applyDetailScroll(body, h-4)

	title := "Session " + shortID(sel.SessionID)
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
		fmt.Fprintf(&body, "%s %s (%s)\n",
			labelStyle.Render("Duration:"),
			valueStyle.Render(durStr),
			mutedStyle.Render(timeStr))
	}

	ratio := 0.0
	if sel.UserMessages > 0 {
		ratio = float64(sel.APICalls) / float64(sel.UserMessages)
	}
	fmt.Fprintf(&body, "%s %s    %s %s    %s %.1fx\n\n",
		labelStyle.Render("Prompts:"), valueStyle.Render(cli.FormatNumber(int64(sel.UserMessages))),
		labelStyle.Render("API Calls:"), valueStyle.Render(cli.FormatNumber(int64(sel.APICalls))),
		labelStyle.Render("Ratio:"), ratio)

	// Token breakdown table
	body.WriteString(headerStyle.Render("TOKEN BREAKDOWN"))
	body.WriteString("\n")
	typeW, tokW, costW, tableW := tokenTableLayout(innerW)
	body.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %*s %*s", typeW, "Type", tokW, "Tokens", costW, "Cost")))
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render(strings.Repeat("─", tableW)))
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
		body.WriteString(valueStyle.Render(fmt.Sprintf("%-*s %*s %*s",
			typeW,
			truncStr(r.typ, typeW),
			tokW,
			cli.FormatTokens(r.tokens),
			costW,
			cli.FormatCost(r.cost))))
		body.WriteString("\n")
	}

	body.WriteString(mutedStyle.Render(strings.Repeat("─", tableW)))
	body.WriteString("\n")
	fmt.Fprintf(&body, "%-*s %*s %*s\n",
		typeW,
		valueStyle.Render("Net Cost"),
		tokW,
		"",
		costW,
		greenStyle.Render(cli.FormatCost(sel.EstimatedCost)))
	fmt.Fprintf(&body, "%-*s %*s %*s\n",
		typeW,
		labelStyle.Render("Cache Savings"),
		tokW,
		"",
		costW,
		greenStyle.Render(cli.FormatCost(savings)))

	// Model breakdown
	if len(sel.Models) > 0 {
		body.WriteString("\n")
		body.WriteString(headerStyle.Render("API CALLS BY MODEL"))
		body.WriteString("\n")
		compactModelTable := innerW < 60
		if compactModelTable {
			modelW := innerW - 7 - 1 - 8
			if modelW < 8 {
				modelW = 8
			}
			body.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %7s %8s", modelW, "Model", "Calls", "Cost")))
			body.WriteString("\n")
			body.WriteString(mutedStyle.Render(strings.Repeat("─", modelW+7+8+2)))
		} else {
			modelW := 14
			body.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %7s %10s %10s %8s", modelW, "Model", "Calls", "Input", "Output", "Cost")))
			body.WriteString("\n")
			body.WriteString(mutedStyle.Render(strings.Repeat("─", modelW+7+10+10+8+4)))
		}
		body.WriteString("\n")

		// Sort model names for deterministic display order
		modelNames := make([]string, 0, len(sel.Models))
		for name := range sel.Models {
			modelNames = append(modelNames, name)
		}
		sort.Strings(modelNames)

		for _, modelName := range modelNames {
			mu := sel.Models[modelName]
			if innerW < 60 {
				modelW := innerW - 7 - 1 - 8
				if modelW < 8 {
					modelW = 8
				}
				body.WriteString(valueStyle.Render(fmt.Sprintf("%-*s %7s %8s",
					modelW,
					truncStr(shortModel(modelName), modelW),
					cli.FormatNumber(int64(mu.APICalls)),
					cli.FormatCost(mu.EstimatedCost))))
			} else {
				modelW := 14
				body.WriteString(valueStyle.Render(fmt.Sprintf("%-*s %7s %10s %10s %8s",
					modelW,
					truncStr(shortModel(modelName), modelW),
					cli.FormatNumber(int64(mu.APICalls)),
					cli.FormatTokens(mu.InputTokens),
					cli.FormatTokens(mu.OutputTokens),
					cli.FormatCost(mu.EstimatedCost))))
			}
			body.WriteString("\n")
		}
	}

	if sel.IsSubagent {
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render("(subagent session)"))
		body.WriteString("\n")
	}

	// Subagent drill-down
	if subs := a.subagentMap[sel.SessionID]; len(subs) > 0 {
		body.WriteString("\n")
		body.WriteString(headerStyle.Render(fmt.Sprintf("SUBAGENTS (%d)", len(subs))))
		body.WriteString("\n")

		nameW := innerW - 8 - 10 - 2
		if nameW < 10 {
			nameW = 10
		}
		body.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %8s %10s", nameW, "Agent", "Duration", "Cost")))
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+8+10+2)))
		body.WriteString("\n")

		var totalSubCost float64
		var totalSubDur int64
		for _, sub := range subs {
			// Extract short agent name from session ID (e.g., "uuid/agent-acompact-7b10e8" -> "acompact-7b10e8")
			agentName := sub.SessionID
			if idx := strings.LastIndex(agentName, "/"); idx >= 0 {
				agentName = agentName[idx+1:]
			}
			agentName = strings.TrimPrefix(agentName, "agent-")

			body.WriteString(valueStyle.Render(fmt.Sprintf("%-*s %8s %10s",
				nameW,
				truncStr(agentName, nameW),
				cli.FormatDuration(sub.DurationSecs),
				cli.FormatCost(sub.EstimatedCost))))
			body.WriteString("\n")
			totalSubCost += sub.EstimatedCost
			totalSubDur += sub.DurationSecs
		}

		body.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+8+10+2)))
		body.WriteString("\n")
		body.WriteString(valueStyle.Render(fmt.Sprintf("%-*s %8s %10s",
			nameW,
			"Combined",
			cli.FormatDuration(totalSubDur),
			cli.FormatCost(totalSubCost))))
		body.WriteString("\n")
	}

	body.WriteString("\n")
	if w < compactWidth {
		body.WriteString(mutedStyle.Render("[j/k] navigate  [J/K] scroll  [q] quit"))
	} else {
		body.WriteString(mutedStyle.Render("[Enter] expand  [j/k] navigate  [J/K/^d/^u] scroll  [q] quit"))
	}

	return body.String()
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// applyDetailScroll applies the detail pane scroll offset to a rendered body string.
// visibleH is the number of lines that fit in the card body area.
func (a App) applyDetailScroll(body string, visibleH int) string {
	if visibleH < 5 {
		visibleH = 5
	}

	lines := strings.Split(body, "\n")
	if len(lines) <= visibleH {
		return body
	}

	scrollOff := a.sessState.detailScroll
	maxScroll := len(lines) - visibleH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOff > maxScroll {
		scrollOff = maxScroll
	}
	if scrollOff < 0 {
		scrollOff = 0
	}

	endIdx := scrollOff + visibleH
	if endIdx > len(lines) {
		endIdx = len(lines)
	}
	visible := lines[scrollOff:endIdx]

	// Add scroll indicator if content continues below.
	// Count includes the line we're replacing + lines past the viewport.
	if endIdx < len(lines) {
		unseen := len(lines) - endIdx + 1
		dimStyle := lipgloss.NewStyle().Foreground(theme.Active.TextDim)
		visible[len(visible)-1] = dimStyle.Render(fmt.Sprintf("... %d more", unseen))
	}

	return strings.Join(visible, "\n")
}

func tokenTableLayout(innerW int) (typeW, tokenW, costW, tableW int) {
	tokenW = 12
	costW = 10
	typeW = innerW - tokenW - costW - 2
	if typeW < 8 {
		tokenW = 8
		costW = 8
		typeW = innerW - tokenW - costW - 2
	}
	if typeW < 6 {
		typeW = 6
	}
	tableW = typeW + tokenW + costW + 2
	return
}
