package cmd

import (
	"fmt"
	"os"

	"cburn/internal/cli"
	"cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Detailed usage summary with costs",
	RunE:  runSummary,
}

func init() {
	rootCmd.AddCommand(summaryCmd)
}

func runSummary(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}

	if len(result.Sessions) == 0 {
		fmt.Println("\n  No Claude Code sessions found.")
		fmt.Println("  Use Claude Code first, then come back!")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	stats := pipeline.Aggregate(filtered, since, until)

	if stats.TotalSessions == 0 {
		fmt.Println("\n  No sessions found in the selected time range.")
		return nil
	}

	// Compute previous period for comparison
	prevDuration := until.Sub(since)
	prevSince := since.Add(-prevDuration)
	prevStats := pipeline.Aggregate(filtered, prevSince, since)

	// Render output
	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("CLAUDE USAGE  Last %dd", flagDays)))
	fmt.Println()

	// Build the summary table
	rows := [][]string{
		{"Sessions", cli.FormatNumber(int64(stats.TotalSessions))},
		{"Prompts", cli.FormatNumber(int64(stats.TotalPrompts))},
		{"Total Time", cli.FormatDuration(stats.TotalDurationSecs)},
		{"---"},
		{"Input Tokens", cli.FormatTokens(stats.InputTokens)},
		{"Output Tokens", cli.FormatTokens(stats.OutputTokens)},
		{"Cache Write (5m)", cli.FormatTokens(stats.CacheCreation5mTokens)},
		{"Cache Write (1h)", cli.FormatTokens(stats.CacheCreation1hTokens)},
		{"Cache Read", cli.FormatTokens(stats.CacheReadTokens)},
		{"Total Billed", cli.FormatTokens(stats.TotalBilledTokens)},
		{"---"},
		{"Cost (est)", cli.FormatCost(stats.EstimatedCost)},
		{"Cache Savings", cli.FormatCost(stats.CacheSavings)},
		{"Cache Hit Rate", cli.FormatPercent(stats.CacheHitRate)},
		{"---"},
	}

	// Cost per day with delta
	costDayStr := fmt.Sprintf("%s/day", cli.FormatCost(stats.CostPerDay))
	if prevStats.CostPerDay > 0 {
		costDayStr += fmt.Sprintf("  (%s vs prev %dd)",
			cli.FormatDelta(stats.CostPerDay, prevStats.CostPerDay), flagDays)
	}
	rows = append(rows, []string{"Cost/day", costDayStr})
	rows = append(rows, []string{"Tokens/day", cli.FormatTokens(stats.TokensPerDay)})
	rows = append(rows, []string{"Sessions/day", fmt.Sprintf("%.1f", stats.SessionsPerDay)})

	table := cli.Table{
		Headers: []string{"Metric", "Value"},
		Rows:    rows,
	}

	fmt.Print(cli.RenderTable(table))

	// Print warnings
	if result.FileErrors > 0 {
		fmt.Fprintf(os.Stderr, "\n  %d files could not be parsed\n", result.FileErrors)
	}

	return nil
}
