package config

import (
	"strings"
	"time"
)

// ModelPricing holds per-million-token prices for a model.
type ModelPricing struct {
	InputPerMTok        float64
	OutputPerMTok       float64
	CacheWrite5mPerMTok float64
	CacheWrite1hPerMTok float64
	CacheReadPerMTok    float64
	// Long context overrides (>200K input tokens)
	LongInputPerMTok  float64
	LongOutputPerMTok float64
}

type modelPricingVersion struct {
	EffectiveFrom time.Time
	Pricing       ModelPricing
}

// DefaultPricing maps model base names to their pricing.
var DefaultPricing = map[string]ModelPricing{
	"claude-opus-4-6": {
		InputPerMTok: 5.00, OutputPerMTok: 25.00,
		CacheWrite5mPerMTok: 6.25, CacheWrite1hPerMTok: 10.00, CacheReadPerMTok: 0.50,
		LongInputPerMTok: 10.00, LongOutputPerMTok: 37.50,
	},
	"claude-opus-4-5": {
		InputPerMTok: 5.00, OutputPerMTok: 25.00,
		CacheWrite5mPerMTok: 6.25, CacheWrite1hPerMTok: 10.00, CacheReadPerMTok: 0.50,
		LongInputPerMTok: 10.00, LongOutputPerMTok: 37.50,
	},
	"claude-opus-4-1": {
		InputPerMTok: 15.00, OutputPerMTok: 75.00,
		CacheWrite5mPerMTok: 18.75, CacheWrite1hPerMTok: 30.00, CacheReadPerMTok: 1.50,
		LongInputPerMTok: 30.00, LongOutputPerMTok: 112.50,
	},
	"claude-opus-4": {
		InputPerMTok: 15.00, OutputPerMTok: 75.00,
		CacheWrite5mPerMTok: 18.75, CacheWrite1hPerMTok: 30.00, CacheReadPerMTok: 1.50,
		LongInputPerMTok: 30.00, LongOutputPerMTok: 112.50,
	},
	"claude-sonnet-4-6": {
		InputPerMTok: 3.00, OutputPerMTok: 15.00,
		CacheWrite5mPerMTok: 3.75, CacheWrite1hPerMTok: 6.00, CacheReadPerMTok: 0.30,
		LongInputPerMTok: 6.00, LongOutputPerMTok: 22.50,
	},
	"claude-sonnet-4-5": {
		InputPerMTok: 3.00, OutputPerMTok: 15.00,
		CacheWrite5mPerMTok: 3.75, CacheWrite1hPerMTok: 6.00, CacheReadPerMTok: 0.30,
		LongInputPerMTok: 6.00, LongOutputPerMTok: 22.50,
	},
	"claude-sonnet-4": {
		InputPerMTok: 3.00, OutputPerMTok: 15.00,
		CacheWrite5mPerMTok: 3.75, CacheWrite1hPerMTok: 6.00, CacheReadPerMTok: 0.30,
		LongInputPerMTok: 6.00, LongOutputPerMTok: 22.50,
	},
	"claude-haiku-4-5": {
		InputPerMTok: 1.00, OutputPerMTok: 5.00,
		CacheWrite5mPerMTok: 1.25, CacheWrite1hPerMTok: 2.00, CacheReadPerMTok: 0.10,
		LongInputPerMTok: 2.00, LongOutputPerMTok: 7.50,
	},
	"claude-haiku-3-5": {
		InputPerMTok: 0.80, OutputPerMTok: 4.00,
		CacheWrite5mPerMTok: 1.00, CacheWrite1hPerMTok: 1.60, CacheReadPerMTok: 0.08,
		LongInputPerMTok: 1.60, LongOutputPerMTok: 6.00,
	},
}

// defaultPricingHistory stores effective-dated prices for each model.
// Entries must be sorted by EffectiveFrom ascending.
var defaultPricingHistory = makeDefaultPricingHistory(DefaultPricing)

func makeDefaultPricingHistory(base map[string]ModelPricing) map[string][]modelPricingVersion {
	history := make(map[string][]modelPricingVersion, len(base))
	for modelName, pricing := range base {
		history[modelName] = []modelPricingVersion{
			{Pricing: pricing},
		}
	}
	return history
}

func hasPricingModel(model string) bool {
	if _, ok := defaultPricingHistory[model]; ok {
		return true
	}
	_, ok := DefaultPricing[model]
	return ok
}

// NormalizeModelName strips date suffixes from model identifiers.
// e.g., "claude-opus-4-5-20251101" -> "claude-opus-4-5"
func NormalizeModelName(raw string) string {
	// Models can have date suffixes like -20251101 (8 digits)
	// Strategy: try progressively shorter prefixes against the pricing table
	if hasPricingModel(raw) {
		return raw
	}

	// Strip last segment if it looks like a date (all digits)
	parts := strings.Split(raw, "-")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if isAllDigits(last) && len(last) >= 8 {
			candidate := strings.Join(parts[:len(parts)-1], "-")
			if hasPricingModel(candidate) {
				return candidate
			}
		}
	}

	return raw
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// LookupPricing returns the pricing for a model, normalizing the name first.
// Returns zero pricing and false if the model is unknown.
func LookupPricing(model string) (ModelPricing, bool) {
	return LookupPricingAt(model, time.Now())
}

// LookupPricingAt returns the pricing for a model at the given timestamp.
// If at is zero, the latest known pricing entry is used.
func LookupPricingAt(model string, at time.Time) (ModelPricing, bool) {
	normalized := NormalizeModelName(model)
	versions, ok := defaultPricingHistory[normalized]
	if !ok || len(versions) == 0 {
		p, fallback := DefaultPricing[normalized]
		return p, fallback
	}

	if at.IsZero() {
		return versions[len(versions)-1].Pricing, true
	}

	at = at.UTC()
	selected := versions[0].Pricing
	for _, v := range versions {
		if v.EffectiveFrom.IsZero() || !at.Before(v.EffectiveFrom.UTC()) {
			selected = v.Pricing
			continue
		}
		break
	}
	return selected, true
}

// CalculateCost computes the estimated cost in USD for a single API call.
func CalculateCost(model string, inputTokens, outputTokens, cache5m, cache1h, cacheRead int64) float64 {
	return CalculateCostAt(model, time.Now(), inputTokens, outputTokens, cache5m, cache1h, cacheRead)
}

// CalculateCostAt computes the estimated cost in USD for a single API call at a point in time.
func CalculateCostAt(
	model string,
	at time.Time,
	inputTokens,
	outputTokens,
	cache5m,
	cache1h,
	cacheRead int64,
) float64 {
	pricing, ok := LookupPricingAt(model, at)
	if !ok {
		return 0
	}

	// Use standard context pricing (long context detection would need total
	// input context size which we don't have per-call; standard is the default)
	cost := float64(inputTokens) * pricing.InputPerMTok / 1_000_000
	cost += float64(outputTokens) * pricing.OutputPerMTok / 1_000_000
	cost += float64(cache5m) * pricing.CacheWrite5mPerMTok / 1_000_000
	cost += float64(cache1h) * pricing.CacheWrite1hPerMTok / 1_000_000
	cost += float64(cacheRead) * pricing.CacheReadPerMTok / 1_000_000

	return cost
}

// CalculateCacheSavings computes how much the cache reads saved vs full input pricing.
func CalculateCacheSavings(model string, cacheReadTokens int64) float64 {
	return CalculateCacheSavingsAt(model, time.Now(), cacheReadTokens)
}

// CalculateCacheSavingsAt computes how much cache reads saved at a point in time.
func CalculateCacheSavingsAt(model string, at time.Time, cacheReadTokens int64) float64 {
	pricing, ok := LookupPricingAt(model, at)
	if !ok {
		return 0
	}
	// Cache reads at cache rate vs what they would have cost at full input rate
	fullCost := float64(cacheReadTokens) * pricing.InputPerMTok / 1_000_000
	actualCost := float64(cacheReadTokens) * pricing.CacheReadPerMTok / 1_000_000
	return fullCost - actualCost
}
