package cmd

import (
	"fmt"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var costsCmd = &cobra.Command{
	Use:   "costs",
	Short: "Cost breakdown by token type and model",
	RunE:  runCosts,
}

func init() {
	rootCmd.AddCommand(costsCmd)
}

func runCosts(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}
	if len(result.Sessions) == 0 {
		fmt.Println("\n  No sessions found.")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	stats := pipeline.Aggregate(filtered, since, until)
	tokenCosts, modelCosts := pipeline.AggregateCostBreakdown(filtered, since, until)

	if stats.TotalSessions == 0 {
		fmt.Println("\n  No sessions in the selected time range.")
		return nil
	}

	// Previous period for comparison
	prevDuration := until.Sub(since)
	prevSince := since.Add(-prevDuration)
	prevStats := pipeline.Aggregate(filtered, prevSince, since)

	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("COST BREAKDOWN  Last %dd", flagDays)))
	fmt.Println()

	// Cost by token type
	type tokenCost struct {
		name string
		cost float64
	}

	totalCost := tokenCosts.TotalCost

	costs := []tokenCost{
		{"Output", tokenCosts.OutputCost},
		{"Cache Write (1h)", tokenCosts.Cache1hCost},
		{"Input", tokenCosts.InputCost},
		{"Cache Write (5m)", tokenCosts.Cache5mCost},
		{"Cache Read", tokenCosts.CacheReadCost},
	}

	// Sort by cost descending (already in expected order, but ensure)
	typeRows := make([][]string, 0, len(costs)+2)
	for _, tc := range costs {
		pct := ""
		if totalCost > 0 {
			pct = fmt.Sprintf("%.1f%%", tc.cost/totalCost*100)
		}
		typeRows = append(typeRows, []string{tc.name, cli.FormatCost(tc.cost), pct})
	}
	typeRows = append(typeRows, []string{"---"})
	typeRows = append(typeRows, []string{"TOTAL", cli.FormatCost(totalCost), ""})

	fmt.Print(cli.RenderTable(cli.Table{
		Title:   "By Token Type",
		Headers: []string{"Type", "Cost", "Share"},
		Rows:    typeRows,
	}))

	// Period comparison
	if prevStats.EstimatedCost > 0 {
		fmt.Printf("  Period Comparison\n")
		maxCost := stats.EstimatedCost
		if prevStats.EstimatedCost > maxCost {
			maxCost = prevStats.EstimatedCost
		}
		fmt.Printf("  This %dd  %s  %s\n",
			flagDays,
			cli.RenderHorizontalBar("", stats.EstimatedCost, maxCost, 30),
			cli.FormatCost(stats.EstimatedCost))
		fmt.Printf("  Prev %dd  %s  %s\n\n",
			flagDays,
			cli.RenderHorizontalBar("", prevStats.EstimatedCost, maxCost, 30),
			cli.FormatCost(prevStats.EstimatedCost))
	}

	// Cost by model
	modelRows := make([][]string, 0, len(modelCosts)+2)
	for _, mc := range modelCosts {
		modelRows = append(modelRows, []string{
			shortModel(mc.Model),
			cli.FormatCost(mc.InputCost),
			cli.FormatCost(mc.OutputCost),
			cli.FormatCost(mc.CacheCost),
			cli.FormatCost(mc.TotalCost),
		})
	}
	modelRows = append(modelRows, []string{"---"})
	modelRows = append(modelRows, []string{
		"TOTAL",
		cli.FormatCost(tokenCosts.InputCost),
		cli.FormatCost(tokenCosts.OutputCost),
		cli.FormatCost(tokenCosts.CacheCost),
		cli.FormatCost(totalCost),
	})

	fmt.Print(cli.RenderTable(cli.Table{
		Title:   "By Model",
		Headers: []string{"Model", "Input", "Output", "Cache", "Total"},
		Rows:    modelRows,
	}))

	fmt.Printf("  Cache Savings: %s saved this period\n\n",
		cli.FormatCost(stats.CacheSavings))

	return nil
}

func shortModel(name string) string {
	// "claude-opus-4-6" -> "opus-4-6"
	if len(name) > 7 && name[:7] == "claude-" {
		return name[7:]
	}
	return name
}
