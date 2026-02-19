package cmd

import (
	"fmt"
	"strings"

	"cburn/internal/cli"
	"cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var hourlyCmd = &cobra.Command{
	Use:   "hourly",
	Short: "Activity by hour of day",
	RunE:  runHourly,
}

func init() {
	rootCmd.AddCommand(hourlyCmd)
}

func runHourly(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}
	if len(result.Sessions) == 0 {
		fmt.Println("\n  No sessions found.")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	hours := pipeline.AggregateHourly(filtered, since, until)

	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("ACTIVITY BY HOUR  Last %dd (local time)", flagDays)))
	fmt.Println()

	// Find max for bar scaling
	maxPrompts := 0
	for _, h := range hours {
		if h.Prompts > maxPrompts {
			maxPrompts = h.Prompts
		}
	}

	maxBarWidth := 40
	for _, h := range hours {
		barLen := 0
		if maxPrompts > 0 {
			barLen = h.Prompts * maxBarWidth / maxPrompts
		}
		bar := strings.Repeat("█", barLen)

		promptStr := cli.FormatNumber(int64(h.Prompts))
		fmt.Printf("  %02d:00 │ %6s │ %s\n", h.Hour, promptStr, bar)
	}

	// Find peak hour
	peakHour := 0
	for _, h := range hours {
		if h.Prompts > hours[peakHour].Prompts {
			peakHour = h.Hour
		}
	}
	fmt.Printf("\n  Peak: %02d:00 (%s prompts)\n\n",
		peakHour, cli.FormatNumber(int64(hours[peakHour].Prompts)))

	return nil
}
