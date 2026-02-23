package tui

import (
	"fmt"

	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/huh"
)

// setupValues holds the form-bound variables for the setup wizard.
type setupValues struct {
	sessionKey string
	adminKey   string
	days       int
	theme      string
}

// newSetupForm builds the huh form for first-run configuration.
func newSetupForm(numSessions int, claudeDir string, vals *setupValues) *huh.Form {
	cfg, _ := config.Load()

	// Pre-populate defaults
	vals.days = cfg.General.DefaultDays
	if vals.days == 0 {
		vals.days = 30
	}
	vals.theme = cfg.Appearance.Theme
	if vals.theme == "" {
		vals.theme = "flexoki-dark"
	}

	// Build welcome text
	welcomeDesc := "Let's configure your dashboard."
	if numSessions > 0 {
		welcomeDesc = fmt.Sprintf("Found %d sessions in %s.", numSessions, claudeDir)
	}

	// Placeholder text for key fields
	sessionPlaceholder := "sk-ant-sid... (Enter to skip)"
	if key := config.GetSessionKey(cfg); key != "" {
		sessionPlaceholder = maskKey(key) + " (Enter to keep)"
	}
	adminPlaceholder := "sk-ant-admin-... (Enter to skip)"
	if key := config.GetAdminAPIKey(cfg); key != "" {
		adminPlaceholder = maskKey(key) + " (Enter to keep)"
	}

	// Build theme options from the registered theme list
	themeOpts := make([]huh.Option[string], len(theme.All))
	for i, t := range theme.All {
		themeOpts[i] = huh.NewOption(t.Name, t.Name)
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Welcome to cburn").
				Description(welcomeDesc).
				Next(true).
				NextLabel("Start"),
		),

		huh.NewGroup(
			huh.NewInput().
				Title("Claude.ai session key").
				Description("For rate-limit and subscription data.\nclaude.ai > DevTools > Application > Cookies > sessionKey").
				Placeholder(sessionPlaceholder).
				EchoMode(huh.EchoModePassword).
				Value(&vals.sessionKey),

			huh.NewInput().
				Title("Anthropic Admin API key").
				Description("For real cost data from the billing API.").
				Placeholder(adminPlaceholder).
				EchoMode(huh.EchoModePassword).
				Value(&vals.adminKey),
		),

		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Default time range").
				Options(
					huh.NewOption("7 days", 7),
					huh.NewOption("30 days", 30),
					huh.NewOption("90 days", 90),
				).
				Value(&vals.days),

			huh.NewSelect[string]().
				Title("Color theme").
				Options(themeOpts...).
				Value(&vals.theme),
		),
	).WithTheme(huh.ThemeDracula()).WithShowHelp(false)
}

// saveSetupConfig persists the setup wizard values to the config file.
func (a *App) saveSetupConfig() error {
	cfg, _ := config.Load()

	if a.setupVals.sessionKey != "" {
		cfg.ClaudeAI.SessionKey = a.setupVals.sessionKey
	}
	if a.setupVals.adminKey != "" {
		cfg.AdminAPI.APIKey = a.setupVals.adminKey
	}
	cfg.General.DefaultDays = a.setupVals.days
	a.days = a.setupVals.days

	cfg.Appearance.Theme = a.setupVals.theme
	theme.SetActive(a.setupVals.theme)

	return config.Save(cfg)
}

func maskKey(key string) string {
	if len(key) > 16 {
		return key[:8] + "..." + key[len(key)-4:]
	}
	if len(key) > 4 {
		return key[:4] + "..."
	}
	return "****"
}
