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

	"github.com/theirongolddev/cburn/internal/claudeai"
	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/model"
	"github.com/theirongolddev/cburn/internal/pipeline"
	"github.com/theirongolddev/cburn/internal/store"
	"github.com/theirongolddev/cburn/internal/tui/components"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/spinner"
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
	costByType pipeline.TokenTypeCosts
	modelCosts []pipeline.ModelCostBreakdown

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

	// Scroll navigation
	scrollOverhead    = 10 // approximate header + status bar height for half-page calc
	minHalfPageScroll = 1  // minimum lines for half-page scroll
	minContentHeight  = 5  // minimum content area height
)

// loadConfigOrDefault loads config, returning defaults on error.
// This ensures the TUI can always start even if config is corrupted.
func loadConfigOrDefault() config.Config {
	cfg, err := config.Load()
	if err != nil {
		// Return zero-value config with sensible defaults applied
		return config.Config{
			TUI: config.TUIConfig{
				RefreshIntervalSec: 30,
			},
		}
	}
	return cfg
}

// NewApp creates a new TUI app model.
func NewApp(claudeDir string, days int, project, modelFilter string, includeSubagents bool) App {
	needSetup := !config.Exists()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3AA99F")).Background(theme.Active.Surface)

	// Load refresh settings from config
	cfg := loadConfigOrDefault()
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
	cfg := loadConfigOrDefault()
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
	a.costByType, a.modelCosts = pipeline.AggregateCostBreakdown(filtered, since, now)

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

	case tea.MouseMsg:
		if !a.loaded || a.showHelp || (a.needSetup && a.setupForm != nil) {
			return a, nil
		}

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			// Scroll up in sessions tab
			if a.activeTab == 2 && !a.sessState.searching {
				if a.sessState.cursor > 0 {
					a.sessState.cursor--
					a.sessState.detailScroll = 0
				}
			}
			return a, nil

		case tea.MouseButtonWheelDown:
			// Scroll down in sessions tab
			if a.activeTab == 2 && !a.sessState.searching {
				searchFiltered := a.getSearchFilteredSessions()
				if a.sessState.cursor < len(searchFiltered)-1 {
					a.sessState.cursor++
					a.sessState.detailScroll = 0
				}
			}
			return a, nil

		case tea.MouseButtonLeft:
			// Check if click is in tab bar area (first 2 lines)
			if msg.Y <= 1 {
				if tab := a.tabAtX(msg.X); tab >= 0 && tab < len(components.Tabs) {
					a.activeTab = tab
				}
			}
			return a, nil
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

		// Sessions search mode intercepts all keys when active
		if a.activeTab == 2 && a.sessState.searching {
			return a.updateSessionsSearch(msg)
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
			searchFiltered := a.getSearchFilteredSessions()

			switch key {
			case "/":
				// Start search mode
				a.sessState.searching = true
				a.sessState.searchInput = newSearchInput()
				a.sessState.searchInput.Focus()
				return a, a.sessState.searchInput.Cursor.BlinkCmd()
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
				// Clear search if active, otherwise exit detail view
				if a.sessState.searchQuery != "" {
					a.sessState.searchQuery = ""
					a.sessState.cursor = 0
					a.sessState.offset = 0
					return a, nil
				}
				if compactSessions {
					return a, nil
				}
				if a.sessState.viewMode == sessViewDetail {
					a.sessState.viewMode = sessViewSplit
				}
				return a, nil
			case "j", "down":
				if a.sessState.cursor < len(searchFiltered)-1 {
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
				a.sessState.cursor = len(searchFiltered) - 1
				if a.sessState.cursor < 0 {
					a.sessState.cursor = 0
				}
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
				halfPage := (a.height - scrollOverhead) / 2
				if halfPage < minHalfPageScroll {
					halfPage = minHalfPageScroll
				}
				a.sessState.detailScroll += halfPage
				return a, nil
			case "ctrl+u":
				halfPage := (a.height - scrollOverhead) / 2
				if halfPage < minHalfPageScroll {
					halfPage = minHalfPageScroll
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
			// Persist to config (best-effort, ignore errors)
			cfg := loadConfigOrDefault()
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

		// Cache org ID if we got one (best-effort, ignore errors)
		if msg.Data != nil && msg.Data.Org.UUID != "" {
			cfg := loadConfigOrDefault()
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
			cfg := loadConfigOrDefault()
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

	// Polished loading card with accent border
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderAccent).
		Background(t.Surface).
		Padding(2, 4)

	// ASCII art logo effect
	logoStyle := lipgloss.NewStyle().
		Foreground(t.AccentBright).
		Background(t.Surface).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface)

	spinnerStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Surface)

	countStyle := lipgloss.NewStyle().
		Foreground(t.TextPrimary).
		Background(t.Surface)

	var b strings.Builder
	b.WriteString(logoStyle.Render("◈ cburn"))
	b.WriteString(subtitleStyle.Render(" · Claude Usage Metrics"))
	b.WriteString("\n\n")

	if a.progressMax > 0 {
		barW := 40
		if barW > w-30 {
			barW = w - 30
		}
		if barW < 20 {
			barW = 20
		}
		pct := float64(a.progress) / float64(a.progressMax)
		b.WriteString(spinnerStyle.Render(a.spinner.View()))
		b.WriteString(subtitleStyle.Render(" Parsing sessions\n\n"))
		b.WriteString(components.ProgressBar(pct, barW))
		b.WriteString("\n")
		b.WriteString(countStyle.Render(cli.FormatNumber(int64(a.progress))))
		b.WriteString(subtitleStyle.Render(" / "))
		b.WriteString(countStyle.Render(cli.FormatNumber(int64(a.progressMax))))
	} else {
		b.WriteString(spinnerStyle.Render(a.spinner.View()))
		b.WriteString(subtitleStyle.Render(" Discovering sessions..."))
	}

	card := cardStyle.Render(b.String())

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, card,
		lipgloss.WithWhitespaceBackground(t.Background))
}

func (a App) viewHelp() string {
	t := theme.Active
	h := a.height
	w := a.width

	// Polished help overlay with accent border
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderAccent).
		Background(t.Surface).
		Padding(1, 3)

	titleStyle := lipgloss.NewStyle().
		Foreground(t.AccentBright).
		Background(t.Surface).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Surface).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Cyan).
		Background(t.Surface).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Surface)

	dimStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.Surface)

	var b strings.Builder
	b.WriteString(titleStyle.Render("◈ Keyboard Shortcuts"))
	b.WriteString("\n\n")

	// Navigation section
	b.WriteString(sectionStyle.Render("Navigation"))
	b.WriteString("\n")
	navBindings := []struct{ key, desc string }{
		{"o c s b x", "Jump to tab"},
		{"← →", "Previous / Next tab"},
		{"j k", "Navigate lists"},
		{"J K", "Scroll detail pane"},
		{"^d ^u", "Half-page scroll"},
	}
	for _, bind := range navBindings {
		fmt.Fprintf(&b, "  %s  %s\n",
			keyStyle.Render(fmt.Sprintf("%-10s", bind.key)),
			descStyle.Render(bind.desc))
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions"))
	b.WriteString("\n")
	actionBindings := []struct{ key, desc string }{
		{"/", "Search sessions"},
		{"Enter", "Expand / Confirm"},
		{"Esc", "Back / Cancel"},
		{"r", "Refresh data"},
		{"R", "Toggle auto-refresh"},
		{"?", "Toggle help"},
		{"q", "Quit"},
	}
	for _, bind := range actionBindings {
		fmt.Fprintf(&b, "  %s  %s\n",
			keyStyle.Render(fmt.Sprintf("%-10s", bind.key)),
			descStyle.Render(bind.desc))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press any key to close"))

	card := cardStyle.Render(b.String())

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, card,
		lipgloss.WithWhitespaceBackground(t.Background))
}

func (a App) viewMain() string {
	t := theme.Active
	w := a.width
	cw := a.contentWidth()
	h := a.height

	// 1. Render header (tab bar + filter pill)
	filterPillStyle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Background(t.Surface)

	filterAccentStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Surface).
		Bold(true)

	filterStr := filterPillStyle.Render(" ") +
		filterAccentStyle.Render(fmt.Sprintf("%dd", a.days))
	if a.project != "" {
		filterStr += filterPillStyle.Render(" │ ") + filterAccentStyle.Render(a.project)
	}
	if a.modelFilter != "" {
		filterStr += filterPillStyle.Render(" │ ") + filterAccentStyle.Render(a.modelFilter)
	}
	filterStr += filterPillStyle.Render(" ")

	// Pad filter line to full width
	filterRowStyle := lipgloss.NewStyle().
		Background(t.Surface).
		Width(w)

	header := components.RenderTabBar(a.activeTab, w) +
		filterRowStyle.Render(filterStr)

	// 2. Render status bar
	dataAge := fmt.Sprintf("%.1fs", a.loadTime.Seconds())
	statusBar := components.RenderStatusBar(w, dataAge, a.subData, a.refreshing, a.autoRefresh)

	// 3. Calculate content zone height
	headerH := lipgloss.Height(header)
	statusH := lipgloss.Height(statusBar)
	contentH := h - headerH - statusH
	if contentH < minContentHeight {
		contentH = minContentHeight
	}

	// 4. Render tab content (pass contentH to sessions)
	var content string
	switch a.activeTab {
	case 0:
		content = a.renderOverviewTab(cw)
	case 1:
		content = a.renderCostsTab(cw)
	case 2:
		searchFiltered := a.getSearchFilteredSessions()
		content = a.renderSessionsContent(searchFiltered, cw, contentH)
	case 3:
		content = a.renderBreakdownTab(cw)
	case 4:
		content = a.renderSettingsTab(cw)
	}

	// 5. Truncate + pad to exactly contentH lines
	content = padHeight(truncateHeight(content, contentH), contentH)

	// 6. Fill each line to full width with background (fixes gaps between cards)
	content = fillLinesWithBackground(content, cw, t.Background)

	// 7. Place content with background fill (handles centering when w > cw)
	content = lipgloss.Place(w, contentH, lipgloss.Center, lipgloss.Top, content,
		lipgloss.WithWhitespaceBackground(t.Background))

	// 8. Stack vertically
	output := lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar)

	// 9. Ensure entire terminal is filled with background
	// This handles any edge cases where the calculated heights don't perfectly match
	return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, output,
		lipgloss.WithWhitespaceBackground(t.Background))
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

// fillLinesWithBackground pads each line to width w with background color.
// This ensures gaps between cards and empty lines have proper background fill.
func fillLinesWithBackground(s string, w int, bg lipgloss.Color) string {
	lines := strings.Split(s, "\n")

	var result strings.Builder
	for i, line := range lines {
		// Use PlaceHorizontal to ensure proper width and background fill
		// This is more reliable than just Background().Render(spaces)
		placed := lipgloss.PlaceHorizontal(w, lipgloss.Left, line,
			lipgloss.WithWhitespaceBackground(bg))
		result.WriteString(placed)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
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

// ─── Mouse Support ──────────────────────────────────────────────

// tabAtX returns the tab index at the given X coordinate, or -1 if none.
// Hitboxes are derived from the same width rules used by RenderTabBar.
func (a App) tabAtX(x int) int {
	pos := 0
	for i, tab := range components.Tabs {
		// Must match RenderTabBar's visual width calculation exactly.
		// Use lipgloss.Width() to handle unicode and styled text correctly.
		tabW := components.TabVisualWidth(tab, i == a.activeTab)

		if x >= pos && x < pos+tabW {
			return i
		}
		pos += tabW

		// Separator is one column between tabs.
		if i < len(components.Tabs)-1 {
			pos++
		}
	}
	return -1
}

// ─── Session Search ─────────────────────────────────────────────

// updateSessionsSearch handles key events while in search mode.
func (a App) updateSessionsSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		// Apply search and exit search mode
		a.sessState.searchQuery = strings.TrimSpace(a.sessState.searchInput.Value())
		a.sessState.searching = false
		a.sessState.cursor = 0
		a.sessState.offset = 0
		a.sessState.detailScroll = 0
		return a, nil

	case "esc":
		// Cancel search mode without applying
		a.sessState.searching = false
		return a, nil
	}

	// Forward other keys to the text input
	var cmd tea.Cmd
	a.sessState.searchInput, cmd = a.sessState.searchInput.Update(msg)
	return a, cmd
}

// getSearchFilteredSessions returns sessions filtered by the current search query.
func (a App) getSearchFilteredSessions() []model.SessionStats {
	if a.sessState.searchQuery == "" {
		return a.filtered
	}
	return filterSessionsBySearch(a.filtered, a.sessState.searchQuery)
}
