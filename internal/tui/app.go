package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cburn/internal/cli"
	"cburn/internal/config"
	"cburn/internal/model"
	"cburn/internal/pipeline"
	"cburn/internal/store"
	"cburn/internal/tui/components"
	"cburn/internal/tui/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DataLoadedMsg is sent when the data pipeline finishes.
type DataLoadedMsg struct {
	Sessions     []model.SessionStats
	ProjectCount int
	LoadTime     time.Duration
	FileErrors   int
}

// ProgressMsg reports file parsing progress.
type ProgressMsg struct {
	Current int
	Total   int
}

// App is the root Bubble Tea model.
type App struct {
	// Data
	sessions     []model.SessionStats
	projectCount int
	loaded       bool
	loadTime     time.Duration

	// Pre-computed for current filter
	filtered []model.SessionStats
	stats    model.SummaryStats
	prevStats model.SummaryStats // previous period for comparison
	days_    []model.DailyStats
	models   []model.ModelStats
	projects []model.ProjectStats

	// UI state
	width     int
	height    int
	activeTab int
	showHelp  bool

	// Filter state
	days    int
	project string
	model_  string

	// Per-tab state
	sessState sessionsState
	settings  settingsState

	// First-run setup
	setup     setupState
	needSetup bool

	// Loading — channel-based progress subscription
	loadingDots int
	progress    int
	progressMax int
	loadSub     chan tea.Msg // progress + completion messages from loader goroutine

	// Data dir for pipeline
	claudeDir        string
	includeSubagents bool
}

// NewApp creates a new TUI app model.
func NewApp(claudeDir string, days int, project, modelFilter string, includeSubagents bool) App {
	needSetup := !config.Exists()

	return App{
		claudeDir:        claudeDir,
		days:             days,
		needSetup:        needSetup,
		setup:            newSetupState(),
		project:          project,
		model_:           modelFilter,
		includeSubagents: includeSubagents,
		loadSub:          make(chan tea.Msg, 1),
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		loadDataCmd(a.claudeDir, a.includeSubagents, a.loadSub),
		tickCmd(),
	)
}

func (a *App) recompute() {
	now := time.Now()
	since := now.AddDate(0, 0, -a.days)

	filtered := a.sessions
	if a.project != "" {
		filtered = pipeline.FilterByProject(filtered, a.project)
	}
	if a.model_ != "" {
		filtered = pipeline.FilterByModel(filtered, a.model_)
	}

	a.filtered = pipeline.FilterByTime(filtered, since, now)
	a.stats = pipeline.Aggregate(filtered, since, now)
	a.days_ = pipeline.AggregateDays(filtered, since, now)
	a.models = pipeline.AggregateModels(filtered, since, now)
	a.projects = pipeline.AggregateProjects(filtered, since, now)

	// Previous period for comparison (same duration, immediately before)
	prevSince := since.AddDate(0, 0, -a.days)
	a.prevStats = pipeline.Aggregate(filtered, prevSince, since)

	// Sort filtered sessions for the sessions tab (most recent first)
	sort.Slice(a.filtered, func(i, j int) bool {
		return a.filtered[i].StartTime.After(a.filtered[j].StartTime)
	})

	// Clamp sessions cursor to the new filtered list bounds
	if a.sessState.cursor >= len(a.filtered) {
		a.sessState.cursor = len(a.filtered) - 1
	}
	if a.sessState.cursor < 0 {
		a.sessState.cursor = 0
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		key := msg.String()

		// Global: quit
		if key == "ctrl+c" {
			return a, tea.Quit
		}

		if !a.loaded {
			return a, nil
		}

		// First-run setup wizard intercepts all keys
		if a.needSetup && a.setup.active {
			return a.updateSetup(msg)
		}

		// Settings tab has its own keybindings (text input)
		if a.activeTab == 9 && a.settings.editing {
			return a.updateSettingsInput(msg)
		}

		// Help toggle
		if key == "?" {
			a.showHelp = !a.showHelp
			return a, nil
		}

		// Dismiss help
		if a.showHelp {
			a.showHelp = false
			return a, nil
		}

		// Sessions tab has its own keybindings
		if a.activeTab == 2 {
			switch key {
			case "q":
				if a.sessState.viewMode == sessViewDetail {
					a.sessState.viewMode = sessViewSplit
					return a, nil
				}
				return a, tea.Quit
			case "enter", "f":
				if a.sessState.viewMode == sessViewSplit {
					a.sessState.viewMode = sessViewDetail
				}
				return a, nil
			case "esc":
				if a.sessState.viewMode == sessViewDetail {
					a.sessState.viewMode = sessViewSplit
				}
				return a, nil
			case "j", "down":
				if a.sessState.cursor < len(a.filtered)-1 {
					a.sessState.cursor++
				}
				return a, nil
			case "k", "up":
				if a.sessState.cursor > 0 {
					a.sessState.cursor--
				}
				return a, nil
			case "g":
				a.sessState.cursor = 0
				a.sessState.offset = 0
				return a, nil
			case "G":
				a.sessState.cursor = len(a.filtered) - 1
				return a, nil
			}
		}

		// Settings tab navigation (non-editing mode)
		if a.activeTab == 9 {
			switch key {
			case "j", "down":
				if a.settings.cursor < settingsFieldCount-1 {
					a.settings.cursor++
				}
				return a, nil
			case "k", "up":
				if a.settings.cursor > 0 {
					a.settings.cursor--
				}
				return a, nil
			case "enter":
				return a.settingsStartEdit()
			}
		}

		// Global quit from non-sessions tabs
		if key == "q" {
			return a, tea.Quit
		}

		// Tab navigation
		switch key {
		case "d":
			a.activeTab = 0
		case "c":
			a.activeTab = 1
		case "s":
			a.activeTab = 2
		case "m":
			a.activeTab = 3
		case "p":
			a.activeTab = 4
		case "t":
			a.activeTab = 5
		case "e":
			a.activeTab = 6
		case "a":
			a.activeTab = 7
		case "b":
			a.activeTab = 8
		case "x":
			a.activeTab = 9
		case "left":
			a.activeTab = (a.activeTab - 1 + len(components.Tabs)) % len(components.Tabs)
		case "right":
			a.activeTab = (a.activeTab + 1) % len(components.Tabs)
		}
		return a, nil

	case DataLoadedMsg:
		a.sessions = msg.Sessions
		a.projectCount = msg.ProjectCount
		a.loaded = true
		a.loadTime = msg.LoadTime
		a.recompute()

		// Activate first-run setup after data loads
		if a.needSetup {
			a.setup.active = true
		}

		return a, nil

	case ProgressMsg:
		a.progress = msg.Current
		a.progressMax = msg.Total
		return a, waitForLoadMsg(a.loadSub)

	case tickMsg:
		a.loadingDots = (a.loadingDots + 1) % 4
		if !a.loaded {
			return a, tickCmd()
		}
		return a, nil
	}

	return a, nil
}

func (a App) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch a.setup.step {
	case 0: // Welcome
		if key == "enter" {
			a.setup.step = 1
			a.setup.apiKeyIn.Focus()
			return a, a.setup.apiKeyIn.Cursor.BlinkCmd()
		}

	case 1: // API key input
		if key == "enter" {
			a.setup.step = 2
			a.setup.apiKeyIn.Blur()
			return a, nil
		}
		// Forward to text input
		var cmd tea.Cmd
		a.setup.apiKeyIn, cmd = a.setup.apiKeyIn.Update(msg)
		return a, cmd

	case 2: // Days choice
		switch key {
		case "j", "down":
			if a.setup.daysChoice < len(daysOptions)-1 {
				a.setup.daysChoice++
			}
		case "k", "up":
			if a.setup.daysChoice > 0 {
				a.setup.daysChoice--
			}
		case "enter":
			a.setup.step = 3
		}
		return a, nil

	case 3: // Theme choice
		switch key {
		case "j", "down":
			if a.setup.themeChoice < len(theme.All)-1 {
				a.setup.themeChoice++
			}
		case "k", "up":
			if a.setup.themeChoice > 0 {
				a.setup.themeChoice--
			}
		case "enter":
			// Save and show done
			a.saveSetupConfig()
			a.setup.step = 4
			a.recompute()
		}
		return a, nil

	case 4: // Done
		if key == "enter" {
			a.needSetup = false
			a.setup.active = false
		}
		return a, nil
	}

	return a, nil
}

func (a App) contentWidth() int {
	cw := a.width
	if cw > 200 {
		cw = 200
	}
	return cw
}

func (a App) View() string {
	if a.width == 0 {
		return ""
	}

	if !a.loaded {
		return a.viewLoading()
	}

	// First-run setup wizard
	if a.needSetup && a.setup.active {
		return a.renderSetup()
	}

	if a.showHelp {
		return a.viewHelp()
	}

	return a.viewMain()
}

func (a App) viewLoading() string {
	t := theme.Active
	w := a.width
	h := a.height

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("  cburn"))
	b.WriteString(mutedStyle.Render(" - Claude Usage Metrics"))
	b.WriteString("\n\n")

	dots := strings.Repeat(".", a.loadingDots)
	if a.progressMax > 0 {
		barW := w - 20
		if barW < 20 {
			barW = 20
		}
		if barW > 60 {
			barW = 60
		}
		pct := float64(a.progress) / float64(a.progressMax)
		b.WriteString(fmt.Sprintf("  Parsing sessions%s\n", dots))
		b.WriteString(fmt.Sprintf("  %s  %s/%s\n",
			components.ProgressBar(pct, barW),
			cli.FormatNumber(int64(a.progress)),
			cli.FormatNumber(int64(a.progressMax))))
	} else {
		b.WriteString(fmt.Sprintf("  Scanning sessions%s\n", dots))
	}

	content := b.String()
	return padHeight(truncateHeight(content, h), h)
}

func (a App) viewHelp() string {
	t := theme.Active
	h := a.height

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(t.TextPrimary).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  Keybindings"))
	b.WriteString("\n\n")

	bindings := []struct{ key, desc string }{
		{"d/c/s/m/p", "Dashboard / Costs / Sessions / Models / Projects"},
		{"t/e/a/b/x", "Trends / Efficiency / Activity / Budget / Settings"},
		{"<- / ->", "Previous / Next tab"},
		{"j / k", "Navigate sessions"},
		{"Enter / f", "Expand session full-screen"},
		{"Esc", "Back to split view"},
		{"?", "Toggle this help"},
		{"q", "Quit (or back from full-screen)"},
	}

	for _, bind := range bindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			keyStyle.Render(fmt.Sprintf("%-12s", bind.key)),
			descStyle.Render(bind.desc)))
	}

	b.WriteString(fmt.Sprintf("\n  %s\n", descStyle.Render("Press any key to close")))

	content := b.String()
	return padHeight(truncateHeight(content, h), h)
}

func (a App) viewMain() string {
	t := theme.Active
	w := a.width
	cw := a.contentWidth()
	h := a.height

	// 1. Render header (tab bar + filter line)
	filterStyle := lipgloss.NewStyle().Foreground(t.TextDim)
	filterStr := fmt.Sprintf(" [%dd", a.days)
	if a.project != "" {
		filterStr += " | " + a.project
	}
	if a.model_ != "" {
		filterStr += " | " + a.model_
	}
	filterStr += "]"
	header := components.RenderTabBar(a.activeTab, w) + "\n" +
		filterStyle.Render(filterStr) + "\n"

	// 2. Render status bar
	dataAge := fmt.Sprintf("%.1fs", a.loadTime.Seconds())
	statusBar := components.RenderStatusBar(w, dataAge)

	// 3. Calculate content zone height
	headerH := lipgloss.Height(header)
	statusH := lipgloss.Height(statusBar)
	contentH := h - headerH - statusH
	if contentH < 5 {
		contentH = 5
	}

	// 4. Render tab content (pass contentH to sessions)
	var content string
	switch a.activeTab {
	case 0:
		content = a.renderDashboardTab(cw)
	case 1:
		content = a.renderCostsTab(cw)
	case 2:
		content = a.renderSessionsContent(a.filtered, cw, contentH)
	case 3:
		content = a.renderModelsTab(cw)
	case 4:
		content = a.renderProjectsTab(cw)
	case 5:
		content = a.renderTrendsTab(cw)
	case 6:
		content = a.renderEfficiencyTab(cw)
	case 7:
		content = a.renderActivityTab(cw)
	case 8:
		content = a.renderBudgetTab(cw)
	case 9:
		content = a.renderSettingsTab(cw)
	}

	// 5. Truncate + pad to exactly contentH lines
	content = padHeight(truncateHeight(content, contentH), contentH)

	// 6. Center horizontally if terminal wider than content cap
	if w > cw {
		content = lipgloss.Place(w, contentH, lipgloss.Center, lipgloss.Top, content)
	}

	// 7. Stack vertically
	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar)
}

// ─── Dashboard Tab ──────────────────────────────────────────────

func (a App) renderDashboardTab(cw int) string {
	t := theme.Active
	stats := a.stats
	prev := a.prevStats
	days := a.days_
	models := a.models
	projects := a.projects
	var b strings.Builder

	// Row 1: Metric cards
	costDelta := ""
	if prev.CostPerDay > 0 {
		costDelta = fmt.Sprintf("%s/day (%s)", cli.FormatCost(stats.CostPerDay), cli.FormatDelta(stats.CostPerDay, prev.CostPerDay))
	} else {
		costDelta = fmt.Sprintf("%s/day", cli.FormatCost(stats.CostPerDay))
	}

	sessDelta := ""
	if prev.SessionsPerDay > 0 {
		pctChange := (stats.SessionsPerDay - prev.SessionsPerDay) / prev.SessionsPerDay * 100
		sessDelta = fmt.Sprintf("%.1f/day (%+.0f%%)", stats.SessionsPerDay, pctChange)
	} else {
		sessDelta = fmt.Sprintf("%.1f/day", stats.SessionsPerDay)
	}

	cacheDelta := ""
	if prev.CacheHitRate > 0 {
		ppDelta := (stats.CacheHitRate - prev.CacheHitRate) * 100
		cacheDelta = fmt.Sprintf("saved %s (%+.1fpp)", cli.FormatCost(stats.CacheSavings), ppDelta)
	} else {
		cacheDelta = fmt.Sprintf("saved %s", cli.FormatCost(stats.CacheSavings))
	}

	cards := []struct{ Label, Value, Delta string }{
		{"Tokens", cli.FormatTokens(stats.TotalBilledTokens), fmt.Sprintf("%s/day", cli.FormatTokens(stats.TokensPerDay))},
		{"Sessions", cli.FormatNumber(int64(stats.TotalSessions)), sessDelta},
		{"Cost", cli.FormatCost(stats.EstimatedCost), costDelta},
		{"Cache", cli.FormatPercent(stats.CacheHitRate), cacheDelta},
	}
	b.WriteString(components.MetricCardRow(cards, cw))
	b.WriteString("\n")

	// Row 2: Daily token usage chart
	if len(days) > 0 {
		chartVals := make([]float64, len(days))
		chartLabels := chartDateLabels(days)
		for i, d := range days {
			chartVals[len(days)-1-i] = float64(d.InputTokens + d.OutputTokens + d.CacheCreation5m + d.CacheCreation1h)
		}
		chartInnerW := components.CardInnerWidth(cw)
		b.WriteString(components.ContentCard(
			fmt.Sprintf("Daily Token Usage (%dd)", a.days),
			components.BarChart(chartVals, chartLabels, t.Blue, chartInnerW, 10),
			cw,
		))
		b.WriteString("\n")
	}

	// Row 3: Model Split + Top Projects
	halves := components.LayoutRow(cw, 2)
	innerW := components.CardInnerWidth(halves[0])

	nameStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	barStyle := lipgloss.NewStyle().Foreground(t.Accent)
	pctStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	var modelBody strings.Builder
	limit := 5
	if len(models) < limit {
		limit = len(models)
	}
	maxShare := 0.0
	for _, ms := range models[:limit] {
		if ms.SharePercent > maxShare {
			maxShare = ms.SharePercent
		}
	}
	nameW := innerW / 3
	if nameW < 10 {
		nameW = 10
	}
	barMaxLen := innerW - nameW - 8
	if barMaxLen < 5 {
		barMaxLen = 5
	}
	for _, ms := range models[:limit] {
		barLen := 0
		if maxShare > 0 {
			barLen = int(ms.SharePercent / maxShare * float64(barMaxLen))
		}
		modelBody.WriteString(fmt.Sprintf("%s %s %s\n",
			nameStyle.Render(fmt.Sprintf("%-*s", nameW, shortModel(ms.Model))),
			barStyle.Render(strings.Repeat("█", barLen)),
			pctStyle.Render(fmt.Sprintf("%.0f%%", ms.SharePercent))))
	}

	var projBody strings.Builder
	projLimit := 5
	if len(projects) < projLimit {
		projLimit = len(projects)
	}
	projNameW := innerW / 3
	if projNameW < 10 {
		projNameW = 10
	}
	for i, ps := range projects[:projLimit] {
		projBody.WriteString(fmt.Sprintf("%s %s %s %s\n",
			pctStyle.Render(fmt.Sprintf("%d.", i+1)),
			nameStyle.Render(fmt.Sprintf("%-*s", projNameW, truncStr(ps.Project, projNameW))),
			lipgloss.NewStyle().Foreground(t.Blue).Render(cli.FormatTokens(ps.TotalTokens)),
			lipgloss.NewStyle().Foreground(t.Green).Render(cli.FormatCost(ps.EstimatedCost))))
	}

	b.WriteString(components.CardRow([]string{
		components.ContentCard("Model Split", modelBody.String(), halves[0]),
		components.ContentCard("Top Projects", projBody.String(), halves[1]),
	}))

	return b.String()
}

// ─── Costs Tab ──────────────────────────────────────────────────

func (a App) renderCostsTab(cw int) string {
	t := theme.Active
	stats := a.stats
	models := a.models

	innerW := components.CardInnerWidth(cw)

	// Flex model name column: total - fixed numeric cols - gaps
	fixedCols := 10 + 10 + 10 + 10 // Input, Output, Cache, Total
	gaps := 4
	nameW := innerW - fixedCols - gaps
	if nameW < 14 {
		nameW = 14
	}

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	var tableBody strings.Builder
	tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %10s %10s %10s %10s", nameW, "Model", "Input", "Output", "Cache", "Total")))
	tableBody.WriteString("\n")
	tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
	tableBody.WriteString("\n")

	for _, ms := range models {
		inputCost := 0.0
		outputCost := 0.0
		if p, ok := config.LookupPricing(ms.Model); ok {
			inputCost = float64(ms.InputTokens) * p.InputPerMTok / 1e6
			outputCost = float64(ms.OutputTokens) * p.OutputPerMTok / 1e6
		}
		cacheCost := ms.EstimatedCost - inputCost - outputCost

		tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %10s %10s %10s %10s",
			nameW,
			shortModel(ms.Model),
			cli.FormatCost(inputCost),
			cli.FormatCost(outputCost),
			cli.FormatCost(cacheCost),
			cli.FormatCost(ms.EstimatedCost))))
		tableBody.WriteString("\n")
	}

	tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))

	title := fmt.Sprintf("Cost Breakdown  %s (%dd)", cli.FormatCost(stats.EstimatedCost), a.days)

	var b strings.Builder
	b.WriteString(components.ContentCard(title, tableBody.String(), cw))
	b.WriteString("\n")

	// Row 2: Cache Savings + Cost Rate summary cards
	halves := components.LayoutRow(cw, 2)

	savingsBody := fmt.Sprintf("%s\n%s",
		lipgloss.NewStyle().Foreground(t.Green).Bold(true).Render(cli.FormatCost(stats.CacheSavings)),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("cache read savings"))

	rateBody := fmt.Sprintf("%s/day\n%s/mo projected",
		lipgloss.NewStyle().Foreground(t.TextPrimary).Bold(true).Render(cli.FormatCost(stats.CostPerDay)),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render(cli.FormatCost(stats.CostPerDay*30)))

	b.WriteString(components.CardRow([]string{
		components.ContentCard("Cache Savings", savingsBody, halves[0]),
		components.ContentCard("Cost Rate", rateBody, halves[1]),
	}))

	return b.String()
}

// ─── Models Tab ─────────────────────────────────────────────────

func (a App) renderModelsTab(cw int) string {
	t := theme.Active
	models := a.models

	innerW := components.CardInnerWidth(cw)
	fixedCols := 8 + 10 + 10 + 10 + 6 // Calls, Input, Output, Cost, Share
	gaps := 5
	nameW := innerW - fixedCols - gaps
	if nameW < 14 {
		nameW = 14
	}

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

	var tableBody strings.Builder
	tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %8s %10s %10s %10s %6s", nameW, "Model", "Calls", "Input", "Output", "Cost", "Share")))
	tableBody.WriteString("\n")

	for _, ms := range models {
		tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %8s %10s %10s %10s %5.1f%%",
			nameW,
			shortModel(ms.Model),
			cli.FormatNumber(int64(ms.APICalls)),
			cli.FormatTokens(ms.InputTokens),
			cli.FormatTokens(ms.OutputTokens),
			cli.FormatCost(ms.EstimatedCost),
			ms.SharePercent)))
		tableBody.WriteString("\n")
	}

	return components.ContentCard("Model Usage", tableBody.String(), cw)
}

// ─── Projects Tab ───────────────────────────────────────────────

func (a App) renderProjectsTab(cw int) string {
	t := theme.Active
	projects := a.projects

	innerW := components.CardInnerWidth(cw)
	fixedCols := 6 + 8 + 10 + 10 // Sess, Prompts, Tokens, Cost
	gaps := 4
	nameW := innerW - fixedCols - gaps
	if nameW < 18 {
		nameW = 18
	}

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

	var tableBody strings.Builder
	tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %6s %8s %10s %10s", nameW, "Project", "Sess.", "Prompts", "Tokens", "Cost")))
	tableBody.WriteString("\n")

	for _, ps := range projects {
		tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %6d %8s %10s %10s",
			nameW,
			truncStr(ps.Project, nameW),
			ps.Sessions,
			cli.FormatNumber(int64(ps.Prompts)),
			cli.FormatTokens(ps.TotalTokens),
			cli.FormatCost(ps.EstimatedCost))))
		tableBody.WriteString("\n")
	}

	return components.ContentCard("Projects", tableBody.String(), cw)
}

// ─── Trends Tab ─────────────────────────────────────────────────

func (a App) renderTrendsTab(cw int) string {
	t := theme.Active
	days := a.days_

	if len(days) == 0 {
		return components.ContentCard("Trends", "No data", cw)
	}

	halves := components.LayoutRow(cw, 2)

	// Build shared date labels (chronological: oldest left, newest right)
	dateLabels := chartDateLabels(days)

	// Row 1: Daily Tokens + Daily Cost side by side
	tokVals := make([]float64, len(days))
	for i, d := range days {
		tokVals[len(days)-1-i] = float64(d.InputTokens + d.OutputTokens + d.CacheCreation5m + d.CacheCreation1h)
	}
	halfInnerW := components.CardInnerWidth(halves[0])
	tokCard := components.ContentCard("Daily Tokens", components.BarChart(tokVals, dateLabels, t.Blue, halfInnerW, 8), halves[0])

	costVals := make([]float64, len(days))
	for i, d := range days {
		costVals[len(days)-1-i] = d.EstimatedCost
	}
	costInnerW := components.CardInnerWidth(halves[1])
	costCard := components.ContentCard("Daily Cost", components.BarChart(costVals, dateLabels, t.Green, costInnerW, 8), halves[1])

	// Row 2: Daily Sessions full width
	sessVals := make([]float64, len(days))
	for i, d := range days {
		sessVals[len(days)-1-i] = float64(d.Sessions)
	}
	sessInnerW := components.CardInnerWidth(cw)
	sessCard := components.ContentCard("Daily Sessions", components.BarChart(sessVals, dateLabels, t.Accent, sessInnerW, 8), cw)

	var b strings.Builder
	b.WriteString(components.CardRow([]string{tokCard, costCard}))
	b.WriteString("\n")
	b.WriteString(sessCard)

	return b.String()
}

// ─── Efficiency Tab ─────────────────────────────────────────────

func (a App) renderEfficiencyTab(cw int) string {
	t := theme.Active
	stats := a.stats

	var b strings.Builder

	// Row 1: Metric cards
	savingsMultiplier := 0.0
	if stats.EstimatedCost > 0 {
		savingsMultiplier = stats.CacheSavings / stats.EstimatedCost
	}
	cards := []struct{ Label, Value, Delta string }{
		{"Cache Rate", cli.FormatPercent(stats.CacheHitRate), ""},
		{"Savings", cli.FormatCost(stats.CacheSavings), fmt.Sprintf("%.1fx cost", savingsMultiplier)},
		{"Net Cost", cli.FormatCost(stats.EstimatedCost), fmt.Sprintf("%s/day", cli.FormatCost(stats.CostPerDay))},
	}
	b.WriteString(components.MetricCardRow(cards, cw))
	b.WriteString("\n")

	// Row 2: Efficiency metrics card
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

	tokPerPrompt := int64(0)
	outPerPrompt := int64(0)
	if stats.TotalPrompts > 0 {
		tokPerPrompt = (stats.InputTokens + stats.OutputTokens) / int64(stats.TotalPrompts)
		outPerPrompt = stats.OutputTokens / int64(stats.TotalPrompts)
	}
	promptsPerSess := 0.0
	if stats.TotalSessions > 0 {
		promptsPerSess = float64(stats.TotalPrompts) / float64(stats.TotalSessions)
	}

	metrics := []struct{ name, value string }{
		{"Tokens/Prompt", cli.FormatTokens(tokPerPrompt)},
		{"Output/Prompt", cli.FormatTokens(outPerPrompt)},
		{"Prompts/Session", fmt.Sprintf("%.1f", promptsPerSess)},
		{"Minutes/Day", fmt.Sprintf("%.0f", stats.MinutesPerDay)},
	}

	var metricsBody strings.Builder
	for _, m := range metrics {
		metricsBody.WriteString(rowStyle.Render(fmt.Sprintf("%-20s %10s", m.name, m.value)))
		metricsBody.WriteString("\n")
	}

	b.WriteString(components.ContentCard("Efficiency Metrics", metricsBody.String(), cw))

	return b.String()
}

// ─── Activity Tab ───────────────────────────────────────────────

func (a App) renderActivityTab(cw int) string {
	t := theme.Active

	now := time.Now()
	since := now.AddDate(0, 0, -a.days)
	hours := pipeline.AggregateHourly(a.filtered, since, now)

	maxPrompts := 0
	for _, h := range hours {
		if h.Prompts > maxPrompts {
			maxPrompts = h.Prompts
		}
	}

	innerW := components.CardInnerWidth(cw)
	barWidth := innerW - 15 // space for "HH:00 NNNNN "
	if barWidth < 20 {
		barWidth = 20
	}

	numStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	var body strings.Builder
	for _, h := range hours {
		barLen := 0
		if maxPrompts > 0 {
			barLen = h.Prompts * barWidth / maxPrompts
		}

		var barColor lipgloss.Color
		switch {
		case h.Hour >= 9 && h.Hour < 17:
			barColor = t.Green
		case h.Hour >= 6 && h.Hour < 22:
			barColor = t.Yellow
		default:
			barColor = t.Red
		}

		bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", barLen))
		body.WriteString(fmt.Sprintf("%s %s %s\n",
			numStyle.Render(fmt.Sprintf("%02d:00", h.Hour)),
			numStyle.Render(fmt.Sprintf("%5s", cli.FormatNumber(int64(h.Prompts)))),
			bar))
	}

	return components.ContentCard("Activity Patterns", body.String(), cw)
}

// ─── Budget Tab ─────────────────────────────────────────────────

func (a App) renderBudgetTab(cw int) string {
	t := theme.Active
	stats := a.stats
	days := a.days_

	halves := components.LayoutRow(cw, 2)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

	// Row 1: Plan Info + Progress
	planBody := fmt.Sprintf("%s %s\n%s %s",
		labelStyle.Render("Plan:"),
		valueStyle.Render("Max ($200/mo unlimited)"),
		labelStyle.Render("API Equivalent:"),
		lipgloss.NewStyle().Foreground(t.Green).Bold(true).Render(cli.FormatCost(stats.EstimatedCost)))
	planCard := components.ContentCard("Plan Info", planBody, halves[0])

	ceiling := 200.0
	pct := stats.EstimatedCost / ceiling
	progressInnerW := components.CardInnerWidth(halves[1])
	progressBody := components.ProgressBar(pct, progressInnerW-10) + "\n" +
		labelStyle.Render("(of plan ceiling - flat rate)")
	progressCard := components.ContentCard("Progress", progressBody, halves[1])

	// Row 2: Burn Rate + Top Spend Days
	burnBody := fmt.Sprintf("%s %s/day\n%s %s/mo",
		labelStyle.Render("Daily:"),
		valueStyle.Render(cli.FormatCost(stats.CostPerDay)),
		labelStyle.Render("Projected:"),
		valueStyle.Render(cli.FormatCost(stats.CostPerDay*30)))
	burnCard := components.ContentCard("Burn Rate", burnBody, halves[0])

	var spendBody strings.Builder
	if len(days) > 0 {
		limit := 5
		if len(days) < limit {
			limit = len(days)
		}
		sorted := make([]model.DailyStats, len(days))
		copy(sorted, days)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].EstimatedCost > sorted[j].EstimatedCost
		})
		for _, d := range sorted[:limit] {
			spendBody.WriteString(fmt.Sprintf("%s  %s\n",
				valueStyle.Render(d.Date.Format("Jan 02")),
				lipgloss.NewStyle().Foreground(t.Green).Render(cli.FormatCost(d.EstimatedCost))))
		}
	} else {
		spendBody.WriteString("No data\n")
	}
	spendCard := components.ContentCard("Top Spend Days", spendBody.String(), halves[1])

	var b strings.Builder
	b.WriteString(components.CardRow([]string{planCard, progressCard}))
	b.WriteString("\n")
	b.WriteString(components.CardRow([]string{burnCard, spendCard}))

	return b.String()
}

// ─── Helpers ────────────────────────────────────────────────────

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// loadDataCmd starts the data loading pipeline in a background goroutine.
// It streams ProgressMsg updates and a final DataLoadedMsg through sub.
func loadDataCmd(claudeDir string, includeSubagents bool, sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			start := time.Now()

			// Progress callback: non-blocking send so workers aren't stalled.
			// If the channel is full, we skip this update — the next one catches up.
			progressFn := func(current, total int) {
				select {
				case sub <- ProgressMsg{Current: current, Total: total}:
				default:
				}
			}

			// Try cached load
			cache, err := storeOpen()
			if err == nil {
				cr, loadErr := pipeline.LoadWithCache(claudeDir, includeSubagents, cache, progressFn)
				cache.Close()
				if loadErr == nil {
					sub <- DataLoadedMsg{
						Sessions:     cr.Sessions,
						ProjectCount: cr.ProjectCount,
						LoadTime:     time.Since(start),
						FileErrors:   cr.FileErrors,
					}
					return
				}
			}

			// Fallback: uncached load
			result, err := pipeline.Load(claudeDir, includeSubagents, progressFn)
			if err != nil {
				sub <- DataLoadedMsg{LoadTime: time.Since(start)}
				return
			}
			sub <- DataLoadedMsg{
				Sessions:     result.Sessions,
				ProjectCount: result.ProjectCount,
				LoadTime:     time.Since(start),
				FileErrors:   result.FileErrors,
			}
		}()

		// Block until the first message (either ProgressMsg or DataLoadedMsg)
		return <-sub
	}
}

// waitForLoadMsg blocks until the next message arrives from the loader goroutine.
func waitForLoadMsg(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

func storeOpen() (*store.Cache, error) {
	return store.Open(pipeline.CachePath())
}

// chartDateLabels builds compact X-axis labels for a chronological date series.
// First label: month abbreviation (e.g. "Jan"). Month boundaries: "Feb 1".
// Everything else (including last): just the day number.
func chartDateLabels(days []model.DailyStats) []string {
	n := len(days)
	labels := make([]string, n)
	prevMonth := time.Month(0)
	for i, d := range days {
		idx := n - 1 - i // reverse: days are newest-first, labels are oldest-left
		m := d.Date.Month()
		day := d.Date.Day()
		if idx == 0 {
			// First label: just the month name
			labels[idx] = d.Date.Format("Jan")
		} else if m != prevMonth {
			// Month boundary: "Feb 1"
			labels[idx] = fmt.Sprintf("%s %d", d.Date.Format("Jan"), day)
		} else {
			labels[idx] = fmt.Sprintf("%d", day)
		}
		prevMonth = m
	}
	return labels
}

func shortModel(name string) string {
	if len(name) > 7 && name[:7] == "claude-" {
		return name[7:]
	}
	return name
}

func truncStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func truncateHeight(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[:max], "\n")
}

func padHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) >= h {
		return s
	}
	padding := strings.Repeat("\n", h-len(lines))
	return s + padding
}
