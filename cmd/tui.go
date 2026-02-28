package cmd

import (
	"fmt"

	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/tui"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI dashboard",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(_ *cobra.Command, _ []string) error {
	// Load config for theme
	cfg, _ := config.Load()
	theme.SetActive(cfg.Appearance.Theme)

	// Force TrueColor profile so all background styling produces ANSI codes
	// Without this, lipgloss may default to Ascii profile (no colors)
	lipgloss.SetColorProfile(termenv.TrueColor)

	app := tui.NewApp(flagDataDir, flagDays, flagProject, flagModel, !flagNoSubagents)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
