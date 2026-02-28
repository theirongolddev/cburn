package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/model"
	"github.com/theirongolddev/cburn/internal/tui/components"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// SessionsView modes — split is iota (0) so it's the default zero value.
const (
	sessViewSplit  = iota // List + full detail side by side (default)
	sessViewDetail        // Full-screen detail
)

// Layout constants for sessions tab height calculations.
const (
	sessListOverhead   = 6 // card border (2) + header row (2) + footer hint (2)
	sessDetailOverhead = 4 // card border (2) + title (1) + gap (1)
	sessMinVisible     = 5 // minimum visible rows in any pane
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

	// Show search input when in search mode
	if ss.searching {
		var b strings.Builder
		searchStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface).Bold(true)
		spaceStyle := lipgloss.NewStyle().Background(t.Surface)
		b.WriteString(searchStyle.Render("  Search: "))
		b.WriteString(ss.searchInput.View())
		b.WriteString("\n")
		hintStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
		keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface)
		b.WriteString(spaceStyle.Render("  ") + hintStyle.Render("[") + keyStyle.Render("Enter") + hintStyle.Render("] apply  [") +
			keyStyle.Render("Esc") + hintStyle.Render("] cancel"))
		b.WriteString("\n\n")

		// Show preview of filtered results
		previewFiltered := filterSessionsBySearch(a.filtered, ss.searchInput.Value())
		countStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
		b.WriteString(countStyle.Render(fmt.Sprintf("  %d sessions match", len(previewFiltered))))

		return b.String()
	}

	// Build title with search indicator
	title := fmt.Sprintf("Sessions [%dd]", a.days)
	if ss.searchQuery != "" {
		title = fmt.Sprintf("Sessions [%dd] / %q (%d)", a.days, ss.searchQuery, len(filtered))
	}

	if len(filtered) == 0 {
		var body strings.Builder
		body.WriteString(lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface).Render("No sessions found"))
		if ss.searchQuery != "" {
			body.WriteString("\n\n")
			body.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface).Render("[Esc] clear search  [/] new search"))
		}
		return components.ContentCard(title, body.String(), cw)
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

	// Clamp cursor to valid range
	cursor := ss.cursor
	if cursor >= len(sessions) {
		cursor = len(sessions) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	if len(sessions) == 0 {
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

	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)
	selectedStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.SurfaceBright).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	costStyle := lipgloss.NewStyle().Foreground(t.Green).Background(t.Surface)

	var leftBody strings.Builder
	visible := h - sessListOverhead
	if visible < sessMinVisible {
		visible = sessMinVisible
	}

	offset := ss.offset
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
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

		if i == cursor {
			// Selected row with bright background and accent marker
			selectedCostStyle := lipgloss.NewStyle().Foreground(t.GreenBright).Background(t.SurfaceBright).Bold(true)
			marker := lipgloss.NewStyle().Foreground(t.AccentBright).Background(t.SurfaceBright).Render("▸ ")
			leftBody.WriteString(marker + selectedStyle.Render(leftPart) +
				lipgloss.NewStyle().Background(t.SurfaceBright).Render(strings.Repeat(" ", max(1, padN-2))) +
				selectedCostStyle.Render(costStr) +
				lipgloss.NewStyle().Background(t.SurfaceBright).Render(strings.Repeat(" ", max(0, leftInner-len(leftPart)-padN-len(costStr)))))
		} else {
			// Normal row
			leftBody.WriteString(
				lipgloss.NewStyle().Background(t.Surface).Render("  ") +
					mutedStyle.Render(fmt.Sprintf("%-13s", startStr)) +
					lipgloss.NewStyle().Background(t.Surface).Render(" ") +
					rowStyle.Render(dur) +
					lipgloss.NewStyle().Background(t.Surface).Render(strings.Repeat(" ", padN-2)) +
					costStyle.Render(costStr))
		}
		leftBody.WriteString("\n")
	}

	// Build title with search indicator
	leftTitle := fmt.Sprintf("Sessions [%dd]", a.days)
	if ss.searchQuery != "" {
		leftTitle = fmt.Sprintf("Search: %q (%d)", ss.searchQuery, len(sessions))
	}
	leftCard := components.ContentCard(leftTitle, leftBody.String(), leftW)

	// Right pane: full session detail with scroll support
	sel := sessions[cursor]
	rightBody := a.renderDetailBody(sel, rightW, mutedStyle)

	// Apply detail scroll offset
	rightBody = a.applyDetailScroll(rightBody, h-sessDetailOverhead)

	titleStr := "Session " + shortID(sel.SessionID)
	rightCard := components.ContentCard(titleStr, rightBody, rightW)

	return components.CardRow([]string{leftCard, rightCard})
}

func (a App) renderSessionDetail(sessions []model.SessionStats, cw, h int) string {
	t := theme.Active
	ss := a.sessState

	// Clamp cursor to valid range
	cursor := ss.cursor
	if cursor >= len(sessions) {
		cursor = len(sessions) - 1
	}
	if cursor < 0 || len(sessions) == 0 {
		return ""
	}
	sel := sessions[cursor]

	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)

	body := a.renderDetailBody(sel, cw, mutedStyle)
	body = a.applyDetailScroll(body, h-sessDetailOverhead)

	title := "Session " + shortID(sel.SessionID)
	return components.ContentCard(title, body, cw)
}

// renderDetailBody generates the full detail content for a session.
// Used by both the split right pane and the full-screen detail view.
func (a App) renderDetailBody(sel model.SessionStats, w int, mutedStyle lipgloss.Style) string {
	t := theme.Active
	innerW := components.CardInnerWidth(w)

	// Rich color palette for different data types
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)
	tokenStyle := lipgloss.NewStyle().Foreground(t.Cyan).Background(t.Surface)
	costStyle := lipgloss.NewStyle().Foreground(t.GreenBright).Background(t.Surface)
	savingsStyle := lipgloss.NewStyle().Foreground(t.Green).Background(t.Surface).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(t.Magenta).Background(t.Surface)
	modelStyle := lipgloss.NewStyle().Foreground(t.BlueBright).Background(t.Surface)
	accentStyle := lipgloss.NewStyle().Foreground(t.AccentBright).Background(t.Surface).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)

	var body strings.Builder
	body.WriteString(accentStyle.Render(sel.Project))
	body.WriteString("\n")
	body.WriteString(dimStyle.Render(strings.Repeat("─", innerW)))
	body.WriteString("\n\n")

	// Duration line with colored values
	if !sel.StartTime.IsZero() {
		durStr := cli.FormatDuration(sel.DurationSecs)
		timeStr := sel.StartTime.Local().Format("15:04:05")
		if !sel.EndTime.IsZero() {
			timeStr += " - " + sel.EndTime.Local().Format("15:04:05")
		}
		timeStr += " " + sel.StartTime.Local().Format("MST")
		body.WriteString(labelStyle.Render("Duration: "))
		body.WriteString(timeStyle.Render(durStr))
		body.WriteString(dimStyle.Render(" ("))
		body.WriteString(mutedStyle.Render(timeStr))
		body.WriteString(dimStyle.Render(")"))
		body.WriteString("\n")
	}

	ratio := 0.0
	if sel.UserMessages > 0 {
		ratio = float64(sel.APICalls) / float64(sel.UserMessages)
	}
	body.WriteString(labelStyle.Render("Prompts: "))
	body.WriteString(valueStyle.Render(cli.FormatNumber(int64(sel.UserMessages))))
	body.WriteString(dimStyle.Render("    "))
	body.WriteString(labelStyle.Render("API Calls: "))
	body.WriteString(tokenStyle.Render(cli.FormatNumber(int64(sel.APICalls))))
	body.WriteString(dimStyle.Render("    "))
	body.WriteString(labelStyle.Render("Ratio: "))
	body.WriteString(accentStyle.Render(fmt.Sprintf("%.1fx", ratio)))
	body.WriteString("\n\n")

	// Token breakdown table with section header
	sectionStyle := lipgloss.NewStyle().Foreground(t.AccentBright).Background(t.Surface).Bold(true)
	tableHeaderStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface)

	body.WriteString(sectionStyle.Render("TOKEN BREAKDOWN"))
	body.WriteString("\n")
	typeW, tokW, costW, tableW := tokenTableLayout(innerW)
	body.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-*s", typeW, "Type")))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%*s", tokW, "Tokens")))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%*s", costW, "Cost")))
	body.WriteString("\n")
	body.WriteString(dimStyle.Render(strings.Repeat("─", tableW)))
	body.WriteString("\n")

	// Calculate per-type costs (aggregate across models)
	inputCost := 0.0
	outputCost := 0.0
	cache5mCost := 0.0
	cache1hCost := 0.0
	cacheReadCost := 0.0
	savings := 0.0

	for modelName, mu := range sel.Models {
		p, ok := config.LookupPricingAt(modelName, sel.StartTime)
		if ok {
			inputCost += float64(mu.InputTokens) * p.InputPerMTok / 1e6
			outputCost += float64(mu.OutputTokens) * p.OutputPerMTok / 1e6
			cache5mCost += float64(mu.CacheCreation5mTokens) * p.CacheWrite5mPerMTok / 1e6
			cache1hCost += float64(mu.CacheCreation1hTokens) * p.CacheWrite1hPerMTok / 1e6
			cacheReadCost += float64(mu.CacheReadTokens) * p.CacheReadPerMTok / 1e6
			savings += config.CalculateCacheSavingsAt(modelName, sel.StartTime, mu.CacheReadTokens)
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
		body.WriteString(labelStyle.Render(fmt.Sprintf("%-*s", typeW, truncStr(r.typ, typeW))))
		body.WriteString(dimStyle.Render(" "))
		body.WriteString(tokenStyle.Render(fmt.Sprintf("%*s", tokW, cli.FormatTokens(r.tokens))))
		body.WriteString(dimStyle.Render(" "))
		body.WriteString(costStyle.Render(fmt.Sprintf("%*s", costW, cli.FormatCost(r.cost))))
		body.WriteString("\n")
	}

	body.WriteString(dimStyle.Render(strings.Repeat("─", tableW)))
	body.WriteString("\n")
	// Net Cost row - highlighted
	body.WriteString(accentStyle.Render(fmt.Sprintf("%-*s", typeW, "Net Cost")))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(dimStyle.Render(fmt.Sprintf("%*s", tokW, "")))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(savingsStyle.Render(fmt.Sprintf("%*s", costW, cli.FormatCost(sel.EstimatedCost))))
	body.WriteString("\n")
	// Cache Savings row
	body.WriteString(labelStyle.Render(fmt.Sprintf("%-*s", typeW, "Cache Savings")))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(dimStyle.Render(fmt.Sprintf("%*s", tokW, "")))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(savingsStyle.Render(fmt.Sprintf("%*s", costW, cli.FormatCost(savings))))
	body.WriteString("\n")

	// Model breakdown with colored data
	if len(sel.Models) > 0 {
		body.WriteString("\n")
		body.WriteString(sectionStyle.Render("API CALLS BY MODEL"))
		body.WriteString("\n")
		compactModelTable := innerW < 60
		if compactModelTable {
			modelW := innerW - 7 - 1 - 8
			if modelW < 8 {
				modelW = 8
			}
			body.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-*s %7s %8s", modelW, "Model", "Calls", "Cost")))
			body.WriteString("\n")
			body.WriteString(dimStyle.Render(strings.Repeat("─", modelW+7+8+2)))
		} else {
			modelW := 14
			body.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-*s %7s %10s %10s %8s", modelW, "Model", "Calls", "Input", "Output", "Cost")))
			body.WriteString("\n")
			body.WriteString(dimStyle.Render(strings.Repeat("─", modelW+7+10+10+8+4)))
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
				body.WriteString(modelStyle.Render(fmt.Sprintf("%-*s", modelW, truncStr(shortModel(modelName), modelW))))
				body.WriteString(dimStyle.Render(" "))
				body.WriteString(valueStyle.Render(fmt.Sprintf("%7s", cli.FormatNumber(int64(mu.APICalls)))))
				body.WriteString(dimStyle.Render(" "))
				body.WriteString(costStyle.Render(fmt.Sprintf("%8s", cli.FormatCost(mu.EstimatedCost))))
			} else {
				modelW := 14
				body.WriteString(modelStyle.Render(fmt.Sprintf("%-*s", modelW, truncStr(shortModel(modelName), modelW))))
				body.WriteString(dimStyle.Render(" "))
				body.WriteString(valueStyle.Render(fmt.Sprintf("%7s", cli.FormatNumber(int64(mu.APICalls)))))
				body.WriteString(dimStyle.Render(" "))
				body.WriteString(tokenStyle.Render(fmt.Sprintf("%10s", cli.FormatTokens(mu.InputTokens))))
				body.WriteString(dimStyle.Render(" "))
				body.WriteString(tokenStyle.Render(fmt.Sprintf("%10s", cli.FormatTokens(mu.OutputTokens))))
				body.WriteString(dimStyle.Render(" "))
				body.WriteString(costStyle.Render(fmt.Sprintf("%8s", cli.FormatCost(mu.EstimatedCost))))
			}
			body.WriteString("\n")
		}
	}

	if sel.IsSubagent {
		body.WriteString("\n")
		body.WriteString(dimStyle.Render("(subagent session)"))
		body.WriteString("\n")
	}

	// Subagent drill-down with colors
	if subs := a.subagentMap[sel.SessionID]; len(subs) > 0 {
		body.WriteString("\n")
		body.WriteString(sectionStyle.Render(fmt.Sprintf("SUBAGENTS (%d)", len(subs))))
		body.WriteString("\n")

		nameW := innerW - 8 - 10 - 2
		if nameW < 10 {
			nameW = 10
		}
		body.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-*s %8s %10s", nameW, "Agent", "Duration", "Cost")))
		body.WriteString("\n")
		body.WriteString(dimStyle.Render(strings.Repeat("─", nameW+8+10+2)))
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

			body.WriteString(modelStyle.Render(fmt.Sprintf("%-*s", nameW, truncStr(agentName, nameW))))
			body.WriteString(dimStyle.Render(" "))
			body.WriteString(timeStyle.Render(fmt.Sprintf("%8s", cli.FormatDuration(sub.DurationSecs))))
			body.WriteString(dimStyle.Render(" "))
			body.WriteString(costStyle.Render(fmt.Sprintf("%10s", cli.FormatCost(sub.EstimatedCost))))
			body.WriteString("\n")
			totalSubCost += sub.EstimatedCost
			totalSubDur += sub.DurationSecs
		}

		body.WriteString(dimStyle.Render(strings.Repeat("─", nameW+8+10+2)))
		body.WriteString("\n")
		body.WriteString(accentStyle.Render(fmt.Sprintf("%-*s", nameW, "Combined")))
		body.WriteString(dimStyle.Render(" "))
		body.WriteString(timeStyle.Render(fmt.Sprintf("%8s", cli.FormatDuration(totalSubDur))))
		body.WriteString(dimStyle.Render(" "))
		body.WriteString(savingsStyle.Render(fmt.Sprintf("%10s", cli.FormatCost(totalSubCost))))
		body.WriteString("\n")
	}

	// Footer hints with styled keys
	body.WriteString("\n")
	hintKeyStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface)
	hintTextStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
	if w < compactWidth {
		body.WriteString(hintTextStyle.Render("[") + hintKeyStyle.Render("/") + hintTextStyle.Render("] search  [") +
			hintKeyStyle.Render("j/k") + hintTextStyle.Render("] navigate  [") +
			hintKeyStyle.Render("J/K") + hintTextStyle.Render("] scroll  [") +
			hintKeyStyle.Render("q") + hintTextStyle.Render("] quit"))
	} else {
		body.WriteString(hintTextStyle.Render("[") + hintKeyStyle.Render("/") + hintTextStyle.Render("] search  [") +
			hintKeyStyle.Render("Enter") + hintTextStyle.Render("] expand  [") +
			hintKeyStyle.Render("j/k") + hintTextStyle.Render("] navigate  [") +
			hintKeyStyle.Render("J/K/^d/^u") + hintTextStyle.Render("] scroll  [") +
			hintKeyStyle.Render("q") + hintTextStyle.Render("] quit"))
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
	if visibleH < sessMinVisible {
		visibleH = sessMinVisible
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
		dimStyle := lipgloss.NewStyle().Foreground(theme.Active.TextDim).Background(theme.Active.Surface)
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
