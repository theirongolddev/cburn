package cmd

import (
	"fmt"

	"cburn/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("  Config file: %s\n", config.ConfigPath())
	if config.Exists() {
		fmt.Println("  Status: loaded")
	} else {
		fmt.Println("  Status: using defaults (no config file)")
	}
	fmt.Println()

	fmt.Println("  [General]")
	fmt.Printf("    Default days:      %d\n", cfg.General.DefaultDays)
	fmt.Printf("    Include subagents: %v\n", cfg.General.IncludeSubagents)
	if cfg.General.ClaudeDir != "" {
		fmt.Printf("    Claude directory:  %s\n", cfg.General.ClaudeDir)
	}
	fmt.Println()

	fmt.Println("  [Admin API]")
	apiKey := config.GetAdminAPIKey(cfg)
	if apiKey != "" {
		fmt.Printf("    API key: %s\n", maskAPIKey(apiKey))
	} else {
		fmt.Println("    API key: not configured")
	}
	fmt.Println()

	fmt.Println("  [Appearance]")
	fmt.Printf("    Theme: %s\n", cfg.Appearance.Theme)
	fmt.Println()

	fmt.Println("  [Budget]")
	if cfg.Budget.MonthlyUSD != nil {
		fmt.Printf("    Monthly budget: $%.0f\n", *cfg.Budget.MonthlyUSD)
	} else {
		fmt.Println("    Monthly budget: not set")
	}

	planInfo := config.DetectPlan(flagDataDir)
	fmt.Printf("    Plan ceiling:   $%.0f (auto-detected)\n", planInfo.PlanCeiling)
	fmt.Println()

	fmt.Println("  Run `cburn setup` to reconfigure.")
	return nil
}
