package cmd

import (
	"fmt"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Model usage breakdown",
	RunE:  runModels,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}

func runModels(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}
	if len(result.Sessions) == 0 {
		fmt.Println("\n  No sessions found.")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	models := pipeline.AggregateModels(filtered, since, until)

	if len(models) == 0 {
		fmt.Println("\n  No model data in the selected time range.")
		return nil
	}

	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("MODEL USAGE  Last %dd", flagDays)))
	fmt.Println()

	rows := make([][]string, 0, len(models))
	for _, ms := range models {
		rows = append(rows, []string{
			shortModel(ms.Model),
			cli.FormatNumber(int64(ms.APICalls)),
			cli.FormatTokens(ms.InputTokens),
			cli.FormatTokens(ms.OutputTokens),
			cli.FormatCost(ms.EstimatedCost),
			fmt.Sprintf("%.1f%%", ms.SharePercent),
		})
	}

	fmt.Print(cli.RenderTable(cli.Table{
		Headers: []string{"Model", "Calls", "Input", "Output", "Cost", "Share"},
		Rows:    rows,
	}))

	return nil
}
