package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// PlanInfo holds detected Claude subscription plan info.
type PlanInfo struct {
	BillingType string
	PlanCeiling float64
}

// DetectPlan reads ~/.claude/.claude.json to determine the billing plan.
func DetectPlan(claudeDir string) PlanInfo {
	path := filepath.Join(claudeDir, ".claude.json")
	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from known claudeDir
	if err != nil {
		return PlanInfo{PlanCeiling: 200} // default to Max plan
	}

	var raw struct {
		BillingType string `json:"billingType"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return PlanInfo{PlanCeiling: 200}
	}

	info := PlanInfo{BillingType: raw.BillingType}

	switch raw.BillingType {
	case "stripe_subscription":
		info.PlanCeiling = 200 // Max plan default
	default:
		info.PlanCeiling = 100 // Pro plan default
	}

	return info
}
