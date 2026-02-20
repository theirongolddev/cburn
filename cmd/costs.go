package cmd

import (
	"fmt"

	"cburn/internal/cli"
	"cburn/internal/config"
	"cburn/internal/pipeline"

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
	models := pipeline.AggregateModels(filtered, since, until)

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

	// Calculate costs per token type from raw token counts using canonical pricing
	var inputCost, outputCost, cache5mCost, cache1hCost, cacheReadCost float64
	for _, s := range pipeline.FilterByTime(filtered, since, until) {
		for modelName, mu := range s.Models {
			p, ok := config.LookupPricing(modelName)
			if !ok {
				continue
			}
			inputCost += float64(mu.InputTokens) * p.InputPerMTok / 1_000_000
			outputCost += float64(mu.OutputTokens) * p.OutputPerMTok / 1_000_000
			cache5mCost += float64(mu.CacheCreation5mTokens) * p.CacheWrite5mPerMTok / 1_000_000
			cache1hCost += float64(mu.CacheCreation1hTokens) * p.CacheWrite1hPerMTok / 1_000_000
			cacheReadCost += float64(mu.CacheReadTokens) * p.CacheReadPerMTok / 1_000_000
		}
	}

	totalCost := inputCost + outputCost + cache5mCost + cache1hCost + cacheReadCost

	costs := []tokenCost{
		{"Output", outputCost},
		{"Cache Write (1h)", cache1hCost},
		{"Input", inputCost},
		{"Cache Write (5m)", cache5mCost},
		{"Cache Read", cacheReadCost},
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
	modelRows := make([][]string, 0, len(models)+2)
	for _, ms := range models {
		p, _ := config.LookupPricing(ms.Model)
		mInput := float64(ms.InputTokens) * p.InputPerMTok / 1_000_000
		mOutput := float64(ms.OutputTokens) * p.OutputPerMTok / 1_000_000
		mCache := float64(ms.CacheCreation5m)*p.CacheWrite5mPerMTok/1_000_000 +
			float64(ms.CacheCreation1h)*p.CacheWrite1hPerMTok/1_000_000 +
			float64(ms.CacheReadTokens)*p.CacheReadPerMTok/1_000_000

		modelRows = append(modelRows, []string{
			shortModel(ms.Model),
			cli.FormatCost(mInput),
			cli.FormatCost(mOutput),
			cli.FormatCost(mCache),
			cli.FormatCost(ms.EstimatedCost),
		})
	}
	modelRows = append(modelRows, []string{"---"})
	modelRows = append(modelRows, []string{
		"TOTAL",
		cli.FormatCost(inputCost),
		cli.FormatCost(outputCost),
		cli.FormatCost(cache5mCost + cache1hCost + cacheReadCost),
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
