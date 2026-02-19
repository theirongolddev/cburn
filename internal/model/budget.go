package model

// BudgetStats holds budget tracking and forecast data.
type BudgetStats struct {
	PlanCeiling       float64
	CustomBudget      *float64
	CurrentSpend      float64
	DailyBurnRate     float64
	ProjectedMonthly  float64
	DaysRemaining     int
	BudgetUsedPercent float64
}
