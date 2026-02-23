package cmd

import (
	"fmt"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Daily usage table",
	RunE:  runDaily,
}

func init() {
	rootCmd.AddCommand(dailyCmd)
}

func runDaily(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}
	if len(result.Sessions) == 0 {
		fmt.Println("\n  No sessions found.")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	days := pipeline.AggregateDays(filtered, since, until)

	if len(days) == 0 {
		fmt.Println("\n  No data for the selected period.")
		return nil
	}

	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("DAILY USAGE  Last %dd", flagDays)))
	fmt.Println()

	rows := make([][]string, 0, len(days))
	for _, d := range days {
		rows = append(rows, []string{
			d.Date.Format("2006-01-02"),
			cli.FormatDayOfWeek(int(d.Date.Weekday())),
			cli.FormatNumber(int64(d.Sessions)),
			cli.FormatNumber(int64(d.Prompts)),
			cli.FormatTokens(d.InputTokens + d.OutputTokens + d.CacheCreation5m + d.CacheCreation1h),
			cli.FormatCost(d.EstimatedCost),
		})
	}

	fmt.Print(cli.RenderTable(cli.Table{
		Headers: []string{"Date", "Day", "Sessions", "Prompts", "Tokens", "Cost"},
		Rows:    rows,
	}))

	return nil
}
