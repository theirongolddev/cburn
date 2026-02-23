// Package tui provides the interactive Bubble Tea dashboard for cburn.
package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cburn/internal/claudeai"
	"cburn/internal/cli"
	"cburn/internal/config"
	"cburn/internal/model"
	"cburn/internal/pipeline"
	"cburn/internal/store"
	"cburn/internal/tui/components"
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// DataLoadedMsg is sent when the data pipeline finishes.
type DataLoadedMsg struct {
	Sessions []model.SessionStats
	LoadTime time.Duration
}

// ProgressMsg reports file parsing progress.
type ProgressMsg struct {
	Current int
	Total   int
}

// SubDataMsg is sent when the claude.ai subscription data fetch completes.
type SubDataMsg struct {
	Data *claudeai.SubscriptionData
}

// RefreshDataMsg is sent when a background data refresh completes.
type RefreshDataMsg struct {
	Sessions []model.SessionStats
	LoadTime time.Duration
}

// App is the root Bubble Tea model.
type App struct {
	// Data
	sessions []model.SessionStats
	loaded   bool
	loadTime time.Duration

	// Auto-refresh state
	autoRefresh     bool
	refreshInterval time.Duration
	lastRefresh     time.Time
	refreshing      bool

	// Subscription data from claude.ai
	subData     *claudeai.SubscriptionData
	subFetching bool
	subTicks    int // counts ticks for periodic refresh

	// Pre-computed for current filter
	filtered   []model.SessionStats
	stats      model.SummaryStats
	prevStats  model.SummaryStats // previous period for comparison
	dailyStats []model.DailyStats
	models     []model.ModelStats
	projects   []model.ProjectStats

	// Live activity charts (today + last hour)
	todayHourly []model.HourlyStats
	lastHour    []model.MinuteStats

	// Subagent grouping: parent session ID -> subagent sessions
	subagentMap map[string][]model.SessionStats

	// UI state
	width     int
	height    int
	activeTab int
	showHelp  bool

	// Filter state
	days        int
	project     string
	modelFilter string

	// Per-tab state
	sessState sessionsState
	settings  settingsState

	// First-run setup (huh form)
	setupForm *huh.Form
	setupVals setupValues
	needSetup bool

	// Loading — channel-based progress subscription
	spinner     spinner.Model
	progress    int
	progressMax int
	loadSub     chan tea.Msg // progress + completion messages from loader goroutine

	// Data dir for pipeline
	claudeDir        string
	includeSubagents bool
}

const (
	minTerminalWidth = 80
	compactWidth     = 120
	maxContentWidth  = 180
)

// NewApp creates a new TUI app model.
func NewApp(claudeDir string, days int, project, modelFilter string, includeSubagents bool) App {
	needSetup := !config.Exists()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3AA99F"))

	// Load refresh settings from config
	cfg, _ := config.Load()
	refreshInterval := time.Duration(cfg.TUI.RefreshIntervalSec) * time.Second
	if refreshInterval < 10*time.Second {
		refreshInterval = 30 * time.Second // minimum 10s, default 30s
	}

	return App{
		claudeDir:        claudeDir,
		days:             days,
		needSetup:        needSetup,
		project:          project,
		modelFilter:      modelFilter,
		includeSubagents: includeSubagents,
		autoRefresh:      cfg.TUI.AutoRefresh,
		refreshInterval:  refreshInterval,
		spinner:          sp,
		loadSub:          make(chan tea.Msg, 1),
	}
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnableMouseCellMotion, // Enable mouse support
		loadDataCmd(a.claudeDir, a.includeSubagents, a.loadSub),
		a.spinner.Tick,
		tickCmd(),
	}

	// Start subscription data fetch if session key is configured
	cfg, _ := config.Load()
	if sessionKey := config.GetSessionKey(cfg); sessionKey != "" {
		cmds = append(cmds, fetchSubDataCmd(sessionKey))
	}

	return tea.Batch(cmds...)
}

func (a *App) recompute() {
	now := time.Now()
	since := now.AddDate(0, 0, -a.days)

	filtered := a.sessions
	if a.project != "" {
		filtered = pipeline.FilterByProject(filtered, a.project)
	}
	if a.modelFilter != "" {
		filtered = pipeline.FilterByModel(filtered, a.modelFilter)
	}

	timeFiltered := pipeline.FilterByTime(filtered, since, now)
	a.stats = pipeline.Aggregate(filtered, since, now)
	a.dailyStats = pipeline.AggregateDays(filtered, since, now)
	a.models = pipeline.AggregateModels(filtered, since, now)
	a.projects = pipeline.AggregateProjects(filtered, since, now)

	// Live activity charts
	a.todayHourly = pipeline.AggregateTodayHourly(filtered)
	a.lastHour = pipeline.AggregateLastHour(filtered)

	// Previous period for comparison (same duration, immediately before)
	prevSince := since.AddDate(0, 0, -a.days)
	a.prevStats = pipeline.Aggregate(filtered, prevSince, since)

	// Group subagents under their parent sessions for the sessions tab.
	// Other tabs (overview, costs, breakdown) still use full aggregations above.
	a.filtered, a.subagentMap = groupSubagents(timeFiltered)

	// Filter out empty sessions (0 API calls — user started Claude but did nothing)
	n := 0
	for _, s := range a.filtered {
		if s.APICalls > 0 {
			a.filtered[n] = s
			n++
		}
	}
	a.filtered = a.filtered[:n]

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
	a.sessState.detailScroll = 0
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward to setup form if active
		if a.setupForm != nil {
			a.setupForm = a.setupForm.WithWidth(msg.Width).WithHeight(msg.Height)
		}
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
		if a.needSetup && a.setupForm != nil {
			return a.updateSetupForm(msg)
		}

		// Settings tab has its own keybindings (text input)
		if a.activeTab == 4 && a.settings.editing {
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
			compactSessions := a.isCompactLayout()
			switch key {
			case "q":
				if !compactSessions && a.sessState.viewMode == sessViewDetail {
					a.sessState.viewMode = sessViewSplit
					return a, nil
				}
				return a, tea.Quit
			case "enter", "f":
				if compactSessions {
					return a, nil
				}
				if a.sessState.viewMode == sessViewSplit {
					a.sessState.viewMode = sessViewDetail
				}
				return a, nil
			case "esc":
				if compactSessions {
					return a, nil
				}
				if a.sessState.viewMode == sessViewDetail {
					a.sessState.viewMode = sessViewSplit
				}
				return a, nil
			case "j", "down":
				if a.sessState.cursor < len(a.filtered)-1 {
					a.sessState.cursor++
					a.sessState.detailScroll = 0
				}
				return a, nil
			case "k", "up":
				if a.sessState.cursor > 0 {
					a.sessState.cursor--
					a.sessState.detailScroll = 0
				}
				return a, nil
			case "g":
				a.sessState.cursor = 0
				a.sessState.offset = 0
				a.sessState.detailScroll = 0
				return a, nil
			case "G":
				a.sessState.cursor = len(a.filtered) - 1
				a.sessState.detailScroll = 0
				return a, nil
			case "J":
				a.sessState.detailScroll++
				return a, nil
			case "K":
				if a.sessState.detailScroll > 0 {
					a.sessState.detailScroll--
				}
				return a, nil
			case "ctrl+d":
				halfPage := (a.height - 10) / 2
				if halfPage < 1 {
					halfPage = 1
				}
				a.sessState.detailScroll += halfPage
				return a, nil
			case "ctrl+u":
				halfPage := (a.height - 10) / 2
				if halfPage < 1 {
					halfPage = 1
				}
				a.sessState.detailScroll -= halfPage
				if a.sessState.detailScroll < 0 {
					a.sessState.detailScroll = 0
				}
				return a, nil
			}
		}

		// Settings tab navigation (non-editing mode)
		if a.activeTab == 4 {
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

		// Manual refresh
		if key == "r" && !a.refreshing {
			a.refreshing = true
			return a, refreshDataCmd(a.claudeDir, a.includeSubagents)
		}

		// Toggle auto-refresh
		if key == "R" {
			a.autoRefresh = !a.autoRefresh
			// Persist to config
			cfg, _ := config.Load()
			cfg.TUI.AutoRefresh = a.autoRefresh
			_ = config.Save(cfg)
			return a, nil
		}

		// Tab navigation
		switch key {
		case "o":
			a.activeTab = 0
		case "c":
			a.activeTab = 1
		case "s":
			a.activeTab = 2
		case "b":
			a.activeTab = 3
		case "x":
			a.activeTab = 4
		case "left":
			a.activeTab = (a.activeTab - 1 + len(components.Tabs)) % len(components.Tabs)
		case "right":
			a.activeTab = (a.activeTab + 1) % len(components.Tabs)
		}
		return a, nil

	case DataLoadedMsg:
		a.sessions = msg.Sessions
		a.loaded = true
		a.loadTime = msg.LoadTime
		a.lastRefresh = time.Now()
		a.recompute()

		// Activate first-run setup after data loads
		if a.needSetup {
			a.setupForm = newSetupForm(len(a.sessions), a.claudeDir, &a.setupVals)
			if a.width > 0 {
				a.setupForm = a.setupForm.WithWidth(a.width).WithHeight(a.height)
			}
			return a, a.setupForm.Init()
		}

		return a, nil

	case ProgressMsg:
		a.progress = msg.Current
		a.progressMax = msg.Total
		return a, waitForLoadMsg(a.loadSub)

	case SubDataMsg:
		a.subData = msg.Data
		a.subFetching = false

		// Cache org ID if we got one
		if msg.Data != nil && msg.Data.Org.UUID != "" {
			cfg, _ := config.Load()
			if cfg.ClaudeAI.OrgID != msg.Data.Org.UUID {
				cfg.ClaudeAI.OrgID = msg.Data.Org.UUID
				_ = config.Save(cfg)
			}
		}
		return a, nil

	case spinner.TickMsg:
		if !a.loaded {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}
		return a, nil

	case tickMsg:
		a.subTicks++

		cmds := []tea.Cmd{tickCmd()}

		// Refresh subscription data every 5 minutes (1200 ticks at 250ms)
		if a.loaded && !a.subFetching && a.subTicks >= 1200 {
			a.subTicks = 0
			cfg, _ := config.Load()
			if sessionKey := config.GetSessionKey(cfg); sessionKey != "" {
				a.subFetching = true
				cmds = append(cmds, fetchSubDataCmd(sessionKey))
			}
		}

		// Auto-refresh session data
		if a.loaded && a.autoRefresh && !a.refreshing {
			if time.Since(a.lastRefresh) >= a.refreshInterval {
				a.refreshing = true
				cmds = append(cmds, refreshDataCmd(a.claudeDir, a.includeSubagents))
			}
		}

		return a, tea.Batch(cmds...)

	case RefreshDataMsg:
		a.refreshing = false
		a.lastRefresh = time.Now()
		if msg.Sessions != nil {
			a.sessions = msg.Sessions
			a.loadTime = msg.LoadTime
			a.recompute()
		}
		return a, nil
	}

	// Forward unhandled messages to the setup form (cursor blinks, etc.)
	if a.needSetup && a.setupForm != nil {
		return a.updateSetupForm(msg)
	}

	return a, nil
}

func (a App) updateSetupForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := a.setupForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		a.setupForm = f
	}

	if a.setupForm.State == huh.StateCompleted {
		_ = a.saveSetupConfig()
		a.recompute()
		a.needSetup = false
		a.setupForm = nil
		return a, nil
	}

	if a.setupForm.State == huh.StateAborted {
		a.needSetup = false
		a.setupForm = nil
		return a, nil
	}

	return a, cmd
}

func (a App) contentWidth() int {
	cw := a.width
	if cw > maxContentWidth {
		cw = maxContentWidth
	}
	return cw
}

func (a App) isCompactLayout() bool {
	return a.contentWidth() < compactWidth
}

// View implements tea.Model.
func (a App) View() string {
	if a.width == 0 {
		return ""
	}

	if a.width < minTerminalWidth {
		return a.viewTooNarrow()
	}

	if !a.loaded {
		return a.viewLoading()
	}

	// First-run setup wizard
	if a.needSetup && a.setupForm != nil {
		return a.setupForm.View()
	}

	if a.showHelp {
		return a.viewHelp()
	}

	return a.viewMain()
}

func (a App) viewTooNarrow() string {
	h := a.height
	if h < 5 {
		h = 5
	}

	msg := fmt.Sprintf(
		"\n  Terminal too narrow (%d cols)\n\n  cburn needs at least %d columns.\n  Current width: %d\n",
		a.width,
		minTerminalWidth,
		a.width,
	)

	return padHeight(truncateHeight(msg, h), h)
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

	if a.progressMax > 0 {
		barW := w - 20
		if barW < 20 {
			barW = 20
		}
		if barW > 60 {
			barW = 60
		}
		pct := float64(a.progress) / float64(a.progressMax)
		fmt.Fprintf(&b, "  %s Parsing sessions\n", a.spinner.View())
		fmt.Fprintf(&b, "  %s  %s/%s\n",
			components.ProgressBar(pct, barW),
			cli.FormatNumber(int64(a.progress)),
			cli.FormatNumber(int64(a.progressMax)))
	} else {
		fmt.Fprintf(&b, "  %s Scanning sessions\n", a.spinner.View())
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
		{"o/c/s/b", "Overview / Costs / Sessions / Breakdown"},
		{"x", "Settings"},
		{"<- / ->", "Previous / Next tab"},
		{"j / k", "Navigate lists"},
		{"J / K", "Scroll detail pane"},
		{"^d / ^u", "Scroll detail half-page"},
		{"Enter / f", "Expand session full-screen"},
		{"Esc", "Back to split view"},
		{"r / R", "Refresh now / Toggle auto-refresh"},
		{"?", "Toggle this help"},
		{"q", "Quit (or back from full-screen)"},
	}

	for _, bind := range bindings {
		fmt.Fprintf(&b, "  %s  %s\n",
			keyStyle.Render(fmt.Sprintf("%-12s", bind.key)),
			descStyle.Render(bind.desc))
	}

	fmt.Fprintf(&b, "\n  %s\n", descStyle.Render("Press any key to close"))

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
	if a.modelFilter != "" {
		filterStr += " | " + a.modelFilter
	}
	filterStr += "]"
	header := components.RenderTabBar(a.activeTab, w) + "\n" +
		filterStyle.Render(filterStr) + "\n"

	// 2. Render status bar
	dataAge := fmt.Sprintf("%.1fs", a.loadTime.Seconds())
	statusBar := components.RenderStatusBar(w, dataAge, a.subData, a.refreshing, a.autoRefresh)

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
		content = a.renderOverviewTab(cw)
	case 1:
		content = a.renderCostsTab(cw)
	case 2:
		content = a.renderSessionsContent(a.filtered, cw, contentH)
	case 3:
		content = a.renderBreakdownTab(cw)
	case 4:
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

// ─── Helpers ────────────────────────────────────────────────────

// groupSubagents partitions sessions into parent sessions (with combined metrics)
// and a lookup map of parent ID -> original subagent sessions.
// Subagent tokens, costs, and model breakdowns are merged into their parent.
// Orphaned subagents (no matching parent in the list) are kept as standalone entries.
func groupSubagents(sessions []model.SessionStats) ([]model.SessionStats, map[string][]model.SessionStats) {
	subMap := make(map[string][]model.SessionStats)

	// Identify parent IDs
	parentIDs := make(map[string]struct{})
	for _, s := range sessions {
		if !s.IsSubagent {
			parentIDs[s.SessionID] = struct{}{}
		}
	}

	// Partition: collect subagents under their parent, keep orphans standalone
	var parents []model.SessionStats
	for _, s := range sessions {
		if s.IsSubagent {
			if _, ok := parentIDs[s.ParentSession]; ok {
				subMap[s.ParentSession] = append(subMap[s.ParentSession], s)
			} else {
				parents = append(parents, s) // orphan — show standalone
			}
		} else {
			parents = append(parents, s)
		}
	}

	// Merge subagent metrics into each parent (copy to avoid mutating originals)
	for i, p := range parents {
		subs, ok := subMap[p.SessionID]
		if !ok {
			continue
		}

		enriched := p
		enriched.Models = make(map[string]*model.ModelUsage, len(p.Models))
		for k, v := range p.Models {
			cp := *v
			enriched.Models[k] = &cp
		}

		for _, sub := range subs {
			enriched.APICalls += sub.APICalls
			enriched.InputTokens += sub.InputTokens
			enriched.OutputTokens += sub.OutputTokens
			enriched.CacheCreation5mTokens += sub.CacheCreation5mTokens
			enriched.CacheCreation1hTokens += sub.CacheCreation1hTokens
			enriched.CacheReadTokens += sub.CacheReadTokens
			enriched.EstimatedCost += sub.EstimatedCost

			for modelName, mu := range sub.Models {
				existing, exists := enriched.Models[modelName]
				if !exists {
					cp := *mu
					enriched.Models[modelName] = &cp
				} else {
					existing.APICalls += mu.APICalls
					existing.InputTokens += mu.InputTokens
					existing.OutputTokens += mu.OutputTokens
					existing.CacheCreation5mTokens += mu.CacheCreation5mTokens
					existing.CacheCreation1hTokens += mu.CacheCreation1hTokens
					existing.CacheReadTokens += mu.CacheReadTokens
					existing.EstimatedCost += mu.EstimatedCost
				}
			}
		}

		// Recalculate cache hit rate from combined totals
		totalCacheInput := enriched.CacheReadTokens + enriched.CacheCreation5mTokens +
			enriched.CacheCreation1hTokens + enriched.InputTokens
		if totalCacheInput > 0 {
			enriched.CacheHitRate = float64(enriched.CacheReadTokens) / float64(totalCacheInput)
		}

		parents[i] = enriched
	}

	return parents, subMap
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
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
				_ = cache.Close()
				if loadErr == nil {
					sub <- DataLoadedMsg{
						Sessions: cr.Sessions,
						LoadTime: time.Since(start),
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
				Sessions: result.Sessions,
				LoadTime: time.Since(start),
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

// refreshDataCmd refreshes session data in the background (no progress UI).
func refreshDataCmd(claudeDir string, includeSubagents bool) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		cache, err := storeOpen()
		if err == nil {
			cr, loadErr := pipeline.LoadWithCache(claudeDir, includeSubagents, cache, nil)
			_ = cache.Close()
			if loadErr == nil {
				return RefreshDataMsg{
					Sessions: cr.Sessions,
					LoadTime: time.Since(start),
				}
			}
		}

		// Fallback: uncached load
		result, err := pipeline.Load(claudeDir, includeSubagents, nil)
		if err != nil {
			return RefreshDataMsg{LoadTime: time.Since(start)}
		}
		return RefreshDataMsg{
			Sessions: result.Sessions,
			LoadTime: time.Since(start),
		}
	}
}

// chartDateLabels builds compact X-axis labels for a chronological date series.
// First label: month abbreviation (e.g. "Jan"). Month boundaries: "Feb 1".
// Everything else (including last): just the day number.
// days is sorted newest-first; labels are returned oldest-left.
func chartDateLabels(days []model.DailyStats) []string {
	n := len(days)
	labels := make([]string, n)
	// Build chronological date list (oldest first)
	dates := make([]time.Time, n)
	for i, d := range days {
		dates[n-1-i] = d.Date
	}
	prevMonth := time.Month(0)
	for i, dt := range dates {
		m := dt.Month()
		day := dt.Day()
		switch {
		case i == 0:
			labels[i] = dt.Format("Jan")
		case i == n-1:
			labels[i] = strconv.Itoa(day)
		case m != prevMonth:
			labels[i] = dt.Format("Jan")
		default:
			labels[i] = strconv.Itoa(day)
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

func truncStr(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + "…"
}

func truncateHeight(s string, limit int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= limit {
		return s
	}
	return strings.Join(lines[:limit], "\n")
}

func padHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) >= h {
		return s
	}
	padding := strings.Repeat("\n", h-len(lines))
	return s + padding
}

// fetchSubDataCmd fetches subscription data from claude.ai in a background goroutine.
func fetchSubDataCmd(sessionKey string) tea.Cmd {
	return func() tea.Msg {
		client := claudeai.NewClient(sessionKey)
		if client == nil {
			return SubDataMsg{Data: &claudeai.SubscriptionData{
				FetchedAt: time.Now(),
				Error:     errors.New("invalid session key format"),
			}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return SubDataMsg{Data: client.FetchAll(ctx)}
	}
}
