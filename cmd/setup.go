package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"cburn/internal/config"
	"cburn/internal/source"

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
	reader := bufio.NewReader(os.Stdin)

	// Load existing config or defaults
	cfg, _ := config.Load()

	// Count sessions
	files, _ := source.ScanDir(flagDataDir)
	projectCount := source.CountProjects(files)

	fmt.Println()
	fmt.Println("  Welcome to cburn!")
	fmt.Println()
	if len(files) > 0 {
		fmt.Printf("  Found %s sessions in %s (%d projects)\n\n",
			formatNumber(int64(len(files))), flagDataDir, projectCount)
	}

	// 1. API key
	fmt.Println("  1. Anthropic Admin API key")
	fmt.Println("     For real cost data from the billing API.")
	existing := config.GetAdminAPIKey(cfg)
	if existing != "" {
		fmt.Printf("     Current: %s\n", maskAPIKey(existing))
	}
	fmt.Print("     > ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" {
		cfg.AdminAPI.APIKey = apiKey
	}
	fmt.Println()

	// 2. Default time range
	fmt.Println("  2. Default time range")
	fmt.Println("     (1) 7 days")
	fmt.Println("     (2) 30 days [default]")
	fmt.Println("     (3) 90 days")
	fmt.Print("     > ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)
	switch choice {
	case "1":
		cfg.General.DefaultDays = 7
	case "3":
		cfg.General.DefaultDays = 90
	default:
		cfg.General.DefaultDays = 30
	}
	fmt.Println()

	// 3. Theme
	fmt.Println("  3. Color theme")
	fmt.Println("     (1) Flexoki Dark [default]")
	fmt.Println("     (2) Catppuccin Mocha")
	fmt.Println("     (3) Tokyo Night")
	fmt.Println("     (4) Terminal (ANSI 16)")
	fmt.Print("     > ")
	themeChoice, _ := reader.ReadString('\n')
	themeChoice = strings.TrimSpace(themeChoice)
	switch themeChoice {
	case "2":
		cfg.Appearance.Theme = "catppuccin-mocha"
	case "3":
		cfg.Appearance.Theme = "tokyo-night"
	case "4":
		cfg.Appearance.Theme = "terminal"
	default:
		cfg.Appearance.Theme = "flexoki-dark"
	}

	// Save
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println()
	fmt.Printf("  Saved to %s\n", config.ConfigPath())
	fmt.Println("  Run `cburn setup` anytime to reconfigure.")
	fmt.Println()

	return nil
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
