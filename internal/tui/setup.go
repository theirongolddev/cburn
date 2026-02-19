package tui

import (
	"fmt"
	"strings"

	"cburn/internal/config"
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// setupState tracks the first-run setup wizard state.
type setupState struct {
	active    bool
	step      int // 0=welcome, 1=api key, 2=days, 3=theme, 4=done
	apiKeyIn  textinput.Model
	daysChoice int   // index into daysOptions
	themeChoice int  // index into theme.All
	saveErr    error // non-nil if config save failed
}

var daysOptions = []struct {
	label string
	value int
}{
	{"7 days", 7},
	{"30 days", 30},
	{"90 days", 90},
}

func newSetupState() setupState {
	ti := textinput.New()
	ti.Placeholder = "sk-ant-admin-... (or press Enter to skip)"
	ti.CharLimit = 256
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'

	return setupState{
		apiKeyIn:   ti,
		daysChoice: 1, // default 30 days
	}
}

func (a App) renderSetup() string {
	t := theme.Active
	ss := a.setup

	titleStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	accentStyle := lipgloss.NewStyle().Foreground(t.Accent)
	greenStyle := lipgloss.NewStyle().Foreground(t.Green)

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("  Welcome to cburn!"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render(fmt.Sprintf("  Found %s sessions in %s",
		valueStyle.Render(fmt.Sprintf("%d", len(a.sessions))),
		valueStyle.Render(a.claudeDir))))
	b.WriteString("\n\n")

	switch ss.step {
	case 0: // Welcome
		b.WriteString(valueStyle.Render("  Let's set up a few things."))
		b.WriteString("\n\n")
		b.WriteString(accentStyle.Render("  Press Enter to continue"))

	case 1: // API key
		b.WriteString(valueStyle.Render("  1. Anthropic Admin API key"))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("     For real cost data from the billing API."))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("     Get one at console.anthropic.com > Settings > Admin API keys"))
		b.WriteString("\n\n")
		b.WriteString("     ")
		b.WriteString(ss.apiKeyIn.View())
		b.WriteString("\n\n")
		b.WriteString(labelStyle.Render("     Press Enter to continue (leave blank to skip)"))

	case 2: // Default days
		b.WriteString(valueStyle.Render("  2. Default time range"))
		b.WriteString("\n\n")
		for i, opt := range daysOptions {
			if i == ss.daysChoice {
				b.WriteString(accentStyle.Render(fmt.Sprintf("     (o) %s", opt.label)))
			} else {
				b.WriteString(labelStyle.Render(fmt.Sprintf("     ( ) %s", opt.label)))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("     j/k to select, Enter to confirm"))

	case 3: // Theme
		b.WriteString(valueStyle.Render("  3. Color theme"))
		b.WriteString("\n\n")
		for i, th := range theme.All {
			if i == ss.themeChoice {
				b.WriteString(accentStyle.Render(fmt.Sprintf("     (o) %s", th.Name)))
			} else {
				b.WriteString(labelStyle.Render(fmt.Sprintf("     ( ) %s", th.Name)))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("     j/k to select, Enter to confirm"))

	case 4: // Done
		if ss.saveErr != nil {
			warnStyle := lipgloss.NewStyle().Foreground(t.Orange)
			b.WriteString(warnStyle.Render(fmt.Sprintf("  Could not save config: %s", ss.saveErr)))
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("  Settings will apply for this session only."))
		} else {
			b.WriteString(greenStyle.Render("  All set!"))
			b.WriteString("\n\n")
			b.WriteString(labelStyle.Render("  Saved to ~/.config/cburn/config.toml"))
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("  Run `cburn setup` anytime to reconfigure."))
		}
		b.WriteString("\n\n")
		b.WriteString(accentStyle.Render("  Press Enter to launch the dashboard"))
	}

	return b.String()
}

func (a *App) saveSetupConfig() {
	cfg, _ := config.Load()

	apiKey := strings.TrimSpace(a.setup.apiKeyIn.Value())
	if apiKey != "" {
		cfg.AdminAPI.APIKey = apiKey
	}

	if a.setup.daysChoice >= 0 && a.setup.daysChoice < len(daysOptions) {
		cfg.General.DefaultDays = daysOptions[a.setup.daysChoice].value
		a.days = cfg.General.DefaultDays
	}

	if a.setup.themeChoice >= 0 && a.setup.themeChoice < len(theme.All) {
		cfg.Appearance.Theme = theme.All[a.setup.themeChoice].Name
		theme.SetActive(cfg.Appearance.Theme)
	}

	a.setup.saveErr = config.Save(cfg)
}
