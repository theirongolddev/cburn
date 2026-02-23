package pipeline

import (
	"sort"
	"time"

	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/model"
)

// TokenTypeCosts holds aggregate costs split by token type.
type TokenTypeCosts struct {
	InputCost     float64
	OutputCost    float64
	Cache5mCost   float64
	Cache1hCost   float64
	CacheReadCost float64
	CacheCost     float64
	TotalCost     float64
}

// ModelCostBreakdown holds cost components for one model.
type ModelCostBreakdown struct {
	Model         string
	InputCost     float64
	OutputCost    float64
	Cache5mCost   float64
	Cache1hCost   float64
	CacheReadCost float64
	CacheCost     float64
	TotalCost     float64
}

// AggregateCostBreakdown computes token-type and model cost splits.
// Pricing is resolved at each session timestamp.
func AggregateCostBreakdown(
	sessions []model.SessionStats,
	since time.Time,
	until time.Time,
) (TokenTypeCosts, []ModelCostBreakdown) {
	filtered := FilterByTime(sessions, since, until)

	var totals TokenTypeCosts
	byModel := make(map[string]*ModelCostBreakdown)

	for _, s := range filtered {
		for modelName, usage := range s.Models {
			pricing, ok := config.LookupPricingAt(modelName, s.StartTime)
			if !ok {
				continue
			}

			inputCost := float64(usage.InputTokens) * pricing.InputPerMTok / 1_000_000
			outputCost := float64(usage.OutputTokens) * pricing.OutputPerMTok / 1_000_000
			cache5mCost := float64(usage.CacheCreation5mTokens) * pricing.CacheWrite5mPerMTok / 1_000_000
			cache1hCost := float64(usage.CacheCreation1hTokens) * pricing.CacheWrite1hPerMTok / 1_000_000
			cacheReadCost := float64(usage.CacheReadTokens) * pricing.CacheReadPerMTok / 1_000_000

			totals.InputCost += inputCost
			totals.OutputCost += outputCost
			totals.Cache5mCost += cache5mCost
			totals.Cache1hCost += cache1hCost
			totals.CacheReadCost += cacheReadCost

			row, exists := byModel[modelName]
			if !exists {
				row = &ModelCostBreakdown{Model: modelName}
				byModel[modelName] = row
			}
			row.InputCost += inputCost
			row.OutputCost += outputCost
			row.Cache5mCost += cache5mCost
			row.Cache1hCost += cache1hCost
			row.CacheReadCost += cacheReadCost
		}
	}

	totals.CacheCost = totals.Cache5mCost + totals.Cache1hCost + totals.CacheReadCost
	totals.TotalCost = totals.InputCost + totals.OutputCost + totals.CacheCost

	modelRows := make([]ModelCostBreakdown, 0, len(byModel))
	for _, row := range byModel {
		row.CacheCost = row.Cache5mCost + row.Cache1hCost + row.CacheReadCost
		row.TotalCost = row.InputCost + row.OutputCost + row.CacheCost
		modelRows = append(modelRows, *row)
	}

	sort.Slice(modelRows, func(i, j int) bool {
		return modelRows[i].TotalCost > modelRows[j].TotalCost
	})

	return totals, modelRows
}
