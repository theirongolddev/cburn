package cmd

import (
	"errors"
	"fmt"
	"strings"

	"cburn/internal/config"
	"cburn/internal/source"
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "First-time setup wizard",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(_ *cobra.Command, _ []string) error {
	cfg, _ := config.Load()
	files, _ := source.ScanDir(flagDataDir)
	projectCount := source.CountProjects(files)

	// Pre-populate from existing config
	var sessionKey, adminKey string
	days := cfg.General.DefaultDays
	if days == 0 {
		days = 30
	}
	themeName := cfg.Appearance.Theme
	if themeName == "" {
		themeName = "flexoki-dark"
	}

	// Build welcome description
	welcomeDesc := "Let's configure your dashboard."
	if len(files) > 0 {
		welcomeDesc = fmt.Sprintf("Found %d sessions across %d projects in %s.",
			len(files), projectCount, flagDataDir)
	}

	// Build placeholder text showing masked existing values
	sessionPlaceholder := "sk-ant-sid... (Enter to skip)"
	if key := config.GetSessionKey(cfg); key != "" {
		sessionPlaceholder = maskAPIKey(key) + " (Enter to keep)"
	}
	adminPlaceholder := "sk-ant-admin-... (Enter to skip)"
	if key := config.GetAdminAPIKey(cfg); key != "" {
		adminPlaceholder = maskAPIKey(key) + " (Enter to keep)"
	}

	form := huh.NewForm(
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
				Value(&sessionKey),

			huh.NewInput().
				Title("Anthropic Admin API key").
				Description("For real cost data from the billing API.").
				Placeholder(adminPlaceholder).
				EchoMode(huh.EchoModePassword).
				Value(&adminKey),
		),

		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Default time range").
				Options(
					huh.NewOption("7 days", 7),
					huh.NewOption("30 days", 30),
					huh.NewOption("90 days", 90),
				).
				Value(&days),

			huh.NewSelect[string]().
				Title("Color theme").
				Options(themeOpts()...).
				Value(&themeName),
		),
	).WithTheme(huh.ThemeDracula())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println("\n  Setup cancelled.")
			return nil
		}
		return fmt.Errorf("setup form: %w", err)
	}

	// Only overwrite keys if the user typed new ones
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey != "" {
		cfg.ClaudeAI.SessionKey = sessionKey
	}
	adminKey = strings.TrimSpace(adminKey)
	if adminKey != "" {
		cfg.AdminAPI.APIKey = adminKey
	}
	cfg.General.DefaultDays = days
	cfg.Appearance.Theme = themeName

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\n  Saved to %s\n", config.Path())
	fmt.Println("  Run `cburn setup` anytime to reconfigure.")
	fmt.Println()

	return nil
}

func themeOpts() []huh.Option[string] {
	opts := make([]huh.Option[string], len(theme.All))
	for i, t := range theme.All {
		opts[i] = huh.NewOption(t.Name, t.Name)
	}
	return opts
}

func maskAPIKey(key string) string {
	if len(key) > 16 {
		return key[:8] + "..." + key[len(key)-4:]
	}
	if len(key) > 4 {
		return key[:4] + "..."
	}
	return "****"
}
