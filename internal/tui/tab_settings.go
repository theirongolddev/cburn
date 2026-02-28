package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/tui/components"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	settingsFieldAPIKey = iota
	settingsFieldSessionKey
	settingsFieldTheme
	settingsFieldDays
	settingsFieldBudget
	settingsFieldAutoRefresh
	settingsFieldRefreshInterval
	settingsFieldCount // sentinel
)

// settingsFieldCount is used by app.go for cursor bounds checking

// settingsState tracks the settings tab state.
type settingsState struct {
	cursor  int
	editing bool
	input   textinput.Model
	saved   bool  // flash "saved" message briefly
	saveErr error // non-nil if last save failed
}

func newSettingsInput() textinput.Model {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 50
	return ti
}

func (a App) settingsStartEdit() (tea.Model, tea.Cmd) {
	cfg := loadConfigOrDefault()
	a.settings.editing = true
	a.settings.saved = false

	ti := newSettingsInput()

	switch a.settings.cursor {
	case settingsFieldAPIKey:
		ti.Placeholder = "sk-ant-admin-..."
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
		existing := config.GetAdminAPIKey(cfg)
		if existing != "" {
			ti.SetValue(existing)
		}
	case settingsFieldSessionKey:
		ti.Placeholder = "sk-ant-sid..."
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
		existing := config.GetSessionKey(cfg)
		if existing != "" {
			ti.SetValue(existing)
		}
	case settingsFieldTheme:
		ti.Placeholder = "flexoki-dark, catppuccin-mocha, tokyo-night, terminal"
		ti.SetValue(cfg.Appearance.Theme)
		ti.EchoMode = textinput.EchoNormal
	case settingsFieldDays:
		ti.Placeholder = "30"
		ti.SetValue(strconv.Itoa(cfg.General.DefaultDays))
		ti.EchoMode = textinput.EchoNormal
	case settingsFieldBudget:
		ti.Placeholder = "500 (monthly USD, leave empty to clear)"
		if cfg.Budget.MonthlyUSD != nil {
			ti.SetValue(fmt.Sprintf("%.0f", *cfg.Budget.MonthlyUSD))
		}
		ti.EchoMode = textinput.EchoNormal
	case settingsFieldAutoRefresh:
		ti.Placeholder = "true or false"
		ti.SetValue(strconv.FormatBool(a.autoRefresh))
		ti.EchoMode = textinput.EchoNormal
	case settingsFieldRefreshInterval:
		ti.Placeholder = "30 (seconds, minimum 10)"
		// Use effective value from App state to match display
		intervalSec := int(a.refreshInterval.Seconds())
		if intervalSec < 10 {
			intervalSec = 30
		}
		ti.SetValue(strconv.Itoa(intervalSec))
		ti.EchoMode = textinput.EchoNormal
	}

	ti.Focus()
	a.settings.input = ti
	return a, ti.Cursor.BlinkCmd()
}

func (a App) updateSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		a.settingsSave()
		a.settings.editing = false
		a.settings.saved = a.settings.saveErr == nil
		return a, nil
	case "esc":
		a.settings.editing = false
		return a, nil
	}

	var cmd tea.Cmd
	a.settings.input, cmd = a.settings.input.Update(msg)
	return a, cmd
}

func (a *App) settingsSave() {
	cfg := loadConfigOrDefault()
	val := strings.TrimSpace(a.settings.input.Value())

	switch a.settings.cursor {
	case settingsFieldAPIKey:
		cfg.AdminAPI.APIKey = val
	case settingsFieldSessionKey:
		cfg.ClaudeAI.SessionKey = val
	case settingsFieldTheme:
		// Validate theme name
		found := false
		for _, t := range theme.All {
			if t.Name == val {
				found = true
				break
			}
		}
		if found {
			cfg.Appearance.Theme = val
			theme.SetActive(val)
		}
	case settingsFieldDays:
		var d int
		if _, err := fmt.Sscanf(val, "%d", &d); err == nil && d > 0 {
			cfg.General.DefaultDays = d
			a.days = d
			a.recompute()
		}
	case settingsFieldBudget:
		if val == "" {
			cfg.Budget.MonthlyUSD = nil
		} else {
			var b float64
			if _, err := fmt.Sscanf(val, "%f", &b); err == nil && b > 0 {
				cfg.Budget.MonthlyUSD = &b
			}
		}
	case settingsFieldAutoRefresh:
		cfg.TUI.AutoRefresh = val == "true" || val == "1" || val == "yes"
		a.autoRefresh = cfg.TUI.AutoRefresh
	case settingsFieldRefreshInterval:
		var interval int
		if _, err := fmt.Sscanf(val, "%d", &interval); err == nil && interval >= 10 {
			cfg.TUI.RefreshIntervalSec = interval
			a.refreshInterval = time.Duration(interval) * time.Second
		}
	}

	a.settings.saveErr = config.Save(cfg)
}

func (a App) renderSettingsTab(cw int) string {
	t := theme.Active
	cfg := loadConfigOrDefault()

	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)
	selectedStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.SurfaceBright).Bold(true)
	selectedLabelStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.SurfaceBright).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(t.AccentBright).Background(t.Surface)
	greenStyle := lipgloss.NewStyle().Foreground(t.GreenBright).Background(t.Surface)
	markerStyle := lipgloss.NewStyle().Foreground(t.AccentBright).Background(t.SurfaceBright)

	type field struct {
		label string
		value string
	}

	apiKeyDisplay := "(not set)"
	existingKey := config.GetAdminAPIKey(cfg)
	if existingKey != "" {
		if len(existingKey) > 12 {
			apiKeyDisplay = existingKey[:8] + "..." + existingKey[len(existingKey)-4:]
		} else {
			apiKeyDisplay = "****"
		}
	}

	sessionKeyDisplay := "(not set)"
	existingSession := config.GetSessionKey(cfg)
	if existingSession != "" {
		if len(existingSession) > 16 {
			sessionKeyDisplay = existingSession[:12] + "..." + existingSession[len(existingSession)-4:]
		} else {
			sessionKeyDisplay = "****"
		}
	}

	// Use live App state for TUI-specific settings (auto-refresh, interval)
	// to ensure display matches actual behavior after R toggle
	refreshIntervalSec := int(a.refreshInterval.Seconds())
	if refreshIntervalSec < 10 {
		refreshIntervalSec = 30 // match the effective default
	}

	fields := []field{
		{"Admin API Key", apiKeyDisplay},
		{"Session Key", sessionKeyDisplay},
		{"Theme", cfg.Appearance.Theme},
		{"Default Days", strconv.Itoa(cfg.General.DefaultDays)},
		{"Monthly Budget", func() string {
			if cfg.Budget.MonthlyUSD != nil {
				return fmt.Sprintf("$%.0f", *cfg.Budget.MonthlyUSD)
			}
			return "(not set)"
		}()},
		{"Auto Refresh", strconv.FormatBool(a.autoRefresh)},
		{"Refresh Interval", fmt.Sprintf("%ds", refreshIntervalSec)},
	}

	var formBody strings.Builder
	for i, f := range fields {
		// Show text input if currently editing this field
		if a.settings.editing && i == a.settings.cursor {
			formBody.WriteString(markerStyle.Render("▸ "))
			formBody.WriteString(accentStyle.Render(fmt.Sprintf("%-18s ", f.label)))
			formBody.WriteString(a.settings.input.View())
			formBody.WriteString("\n")
			continue
		}

		if i == a.settings.cursor {
			// Selected row with marker and highlight
			marker := markerStyle.Render("▸ ")
			label := selectedLabelStyle.Render(fmt.Sprintf("%-18s ", f.label+":"))
			value := selectedStyle.Render(f.value)
			formBody.WriteString(marker)
			formBody.WriteString(label)
			formBody.WriteString(value)
			// Use lipgloss.Width() for correct visual width calculation
			usedWidth := lipgloss.Width(marker) + lipgloss.Width(label) + lipgloss.Width(value)
			innerW := components.CardInnerWidth(cw)
			padLen := innerW - usedWidth
			if padLen > 0 {
				formBody.WriteString(lipgloss.NewStyle().Background(t.SurfaceBright).Render(strings.Repeat(" ", padLen)))
			}
		} else {
			// Normal row
			formBody.WriteString(lipgloss.NewStyle().Background(t.Surface).Render("  "))
			formBody.WriteString(labelStyle.Render(fmt.Sprintf("%-18s ", f.label+":")))
			formBody.WriteString(valueStyle.Render(f.value))
		}
		formBody.WriteString("\n")
	}

	if a.settings.saveErr != nil {
		warnStyle := lipgloss.NewStyle().Foreground(t.Orange).Background(t.Surface)
		formBody.WriteString("\n")
		formBody.WriteString(warnStyle.Render(fmt.Sprintf("Save failed: %s", a.settings.saveErr)))
	} else if a.settings.saved {
		formBody.WriteString("\n")
		formBody.WriteString(greenStyle.Render("Saved!"))
	}

	formBody.WriteString("\n")
	formBody.WriteString(labelStyle.Render("[j/k] navigate  [Enter] edit  [Esc] cancel"))

	// General info card
	var infoBody strings.Builder
	infoBody.WriteString(labelStyle.Render("Data directory:  ") + valueStyle.Render(a.claudeDir) + "\n")
	infoBody.WriteString(labelStyle.Render("Sessions loaded: ") + valueStyle.Render(cli.FormatNumber(int64(len(a.sessions)))) + "\n")
	infoBody.WriteString(labelStyle.Render("Load time:       ") + valueStyle.Render(fmt.Sprintf("%.1fs", a.loadTime.Seconds())) + "\n")
	infoBody.WriteString(labelStyle.Render("Config file:     ") + valueStyle.Render(config.Path()))

	var b strings.Builder
	b.WriteString(components.ContentCard("Settings", formBody.String(), cw))
	b.WriteString("\n")
	b.WriteString(components.ContentCard("General", infoBody.String(), cw))

	return b.String()
}
