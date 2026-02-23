package config

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

func TestLookupPricingAt_UsesEffectiveDate(t *testing.T) {
	model := "test-model-windowed"
	orig, had := defaultPricingHistory[model]
	if had {
		defer func() { defaultPricingHistory[model] = orig }()
	} else {
		defer delete(defaultPricingHistory, model)
	}

	defaultPricingHistory[model] = []modelPricingVersion{
		{
			EffectiveFrom: mustDate(t, "2025-01-01"),
			Pricing:       ModelPricing{InputPerMTok: 1.0},
		},
		{
			EffectiveFrom: mustDate(t, "2025-07-01"),
			Pricing:       ModelPricing{InputPerMTok: 2.0},
		},
	}

	aprPrice, ok := LookupPricingAt(model, mustDate(t, "2025-04-15"))
	if !ok {
		t.Fatal("LookupPricingAt returned !ok for historical model")
	}
	if aprPrice.InputPerMTok != 1.0 {
		t.Fatalf("April price InputPerMTok = %.2f, want 1.0", aprPrice.InputPerMTok)
	}

	augPrice, ok := LookupPricingAt(model, mustDate(t, "2025-08-15"))
	if !ok {
		t.Fatal("LookupPricingAt returned !ok for historical model in later window")
	}
	if augPrice.InputPerMTok != 2.0 {
		t.Fatalf("August price InputPerMTok = %.2f, want 2.0", augPrice.InputPerMTok)
	}
}

func TestLookupPricingAt_UsesLatestWhenTimeZero(t *testing.T) {
	model := "test-model-latest"
	orig, had := defaultPricingHistory[model]
	if had {
		defer func() { defaultPricingHistory[model] = orig }()
	} else {
		defer delete(defaultPricingHistory, model)
	}

	defaultPricingHistory[model] = []modelPricingVersion{
		{
			EffectiveFrom: mustDate(t, "2025-01-01"),
			Pricing:       ModelPricing{InputPerMTok: 1.0},
		},
		{
			EffectiveFrom: mustDate(t, "2025-09-01"),
			Pricing:       ModelPricing{InputPerMTok: 3.0},
		},
	}

	price, ok := LookupPricingAt(model, time.Time{})
	if !ok {
		t.Fatal("LookupPricingAt returned !ok for model with pricing history")
	}
	if price.InputPerMTok != 3.0 {
		t.Fatalf("zero-time lookup InputPerMTok = %.2f, want 3.0", price.InputPerMTok)
	}
}
