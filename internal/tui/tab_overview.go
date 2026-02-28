package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/pipeline"
	"github.com/theirongolddev/cburn/internal/tui/components"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

func (a App) renderOverviewTab(cw int) string {
	t := theme.Active
	stats := a.stats
	prev := a.prevStats
	days := a.dailyStats
	models := a.models
	var b strings.Builder

	// Row 1: Metric cards with colored values
	costDelta := ""
	if prev.CostPerDay > 0 {
		costDelta = fmt.Sprintf("%s/day (%s)", cli.FormatCost(stats.CostPerDay), cli.FormatDelta(stats.CostPerDay, prev.CostPerDay))
	} else {
		costDelta = cli.FormatCost(stats.CostPerDay) + "/day"
	}

	sessDelta := ""
	if prev.SessionsPerDay > 0 {
		pctChange := (stats.SessionsPerDay - prev.SessionsPerDay) / prev.SessionsPerDay * 100
		sessDelta = fmt.Sprintf("%.1f/day (%+.0f%%)", stats.SessionsPerDay, pctChange)
	} else {
		sessDelta = fmt.Sprintf("%.1f/day", stats.SessionsPerDay)
	}

	cacheDelta := ""
	if prev.CacheHitRate > 0 {
		ppDelta := (stats.CacheHitRate - prev.CacheHitRate) * 100
		cacheDelta = fmt.Sprintf("saved %s (%+.1fpp)", cli.FormatCost(stats.CacheSavings), ppDelta)
	} else {
		cacheDelta = "saved " + cli.FormatCost(stats.CacheSavings)
	}

	cards := []struct{ Label, Value, Delta string }{
		{"Tokens", cli.FormatTokens(stats.TotalBilledTokens), cli.FormatTokens(stats.TokensPerDay) + "/day"},
		{"Sessions", cli.FormatNumber(int64(stats.TotalSessions)), sessDelta},
		{"Cost", cli.FormatCost(stats.EstimatedCost), costDelta},
		{"Cache", cli.FormatPercent(stats.CacheHitRate), cacheDelta},
	}
	b.WriteString(components.MetricCardRow(cards, cw))
	b.WriteString("\n")

	// Row 2: Daily token usage chart - use PanelCard for emphasis
	if len(days) > 0 {
		chartVals := make([]float64, len(days))
		chartLabels := chartDateLabels(days)
		for i, d := range days {
			chartVals[len(days)-1-i] = float64(d.InputTokens + d.OutputTokens + d.CacheCreation5m + d.CacheCreation1h)
		}
		chartInnerW := components.CardInnerWidth(cw)
		b.WriteString(components.PanelCard(
			fmt.Sprintf("Daily Token Usage (%dd)", a.days),
			components.BarChart(chartVals, chartLabels, t.BlueBright, chartInnerW, 10),
			cw,
		))
		b.WriteString("\n")
	}

	// Row 2.5: Live Activity (Today + Last Hour)
	liveHalves := components.LayoutRow(cw, 2)
	liveChartH := 8
	if a.isCompactLayout() {
		liveChartH = 6
	}

	// Left: Today's hourly activity
	var todayCard string
	if len(a.todayHourly) > 0 {
		hourVals := make([]float64, 24)
		var todayTotal int64
		for i, h := range a.todayHourly {
			hourVals[i] = float64(h.Tokens)
			todayTotal += h.Tokens
		}
		todayCard = components.ContentCard(
			fmt.Sprintf("Today (%s)", cli.FormatTokens(todayTotal)),
			components.BarChart(hourVals, hourLabels24(), t.Cyan, components.CardInnerWidth(liveHalves[0]), liveChartH),
			liveHalves[0],
		)
	}

	// Right: Last hour's 5-minute activity
	var lastHourCard string
	if len(a.lastHour) > 0 {
		minVals := make([]float64, 12)
		var hourTotal int64
		for i, m := range a.lastHour {
			minVals[i] = float64(m.Tokens)
			hourTotal += m.Tokens
		}
		lastHourCard = components.ContentCard(
			fmt.Sprintf("Last Hour (%s)", cli.FormatTokens(hourTotal)),
			components.BarChart(minVals, minuteLabels(), t.Magenta, components.CardInnerWidth(liveHalves[1]), liveChartH),
			liveHalves[1],
		)
	}

	if a.isCompactLayout() {
		if todayCard != "" {
			b.WriteString(todayCard)
			b.WriteString("\n")
		}
		if lastHourCard != "" {
			b.WriteString(lastHourCard)
			b.WriteString("\n")
		}
	} else {
		b.WriteString(components.CardRow([]string{todayCard, lastHourCard}))
		b.WriteString("\n")
	}

	// Row 3: Model Split + Activity Patterns
	halves := components.LayoutRow(cw, 2)
	innerW := components.CardInnerWidth(halves[0])

	// Model split with colored bars per model
	var modelBody strings.Builder
	limit := 5
	if len(models) < limit {
		limit = len(models)
	}
	maxShare := 0.0
	for _, ms := range models[:limit] {
		if ms.SharePercent > maxShare {
			maxShare = ms.SharePercent
		}
	}
	nameW := innerW / 3
	if nameW < 10 {
		nameW = 10
	}
	barMaxLen := innerW - nameW - 8
	if barMaxLen < 1 {
		barMaxLen = 1
	}

	// Color palette for models - pre-compute styles to avoid allocation in loop
	modelColors := []lipgloss.Color{t.BlueBright, t.Cyan, t.Magenta, t.Yellow, t.Green}
	sepStyle := lipgloss.NewStyle().Background(t.Surface)
	nameStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)

	// Pre-compute bar and percent styles for each color
	barStyles := make([]lipgloss.Style, len(modelColors))
	pctStyles := make([]lipgloss.Style, len(modelColors))
	for i, color := range modelColors {
		barStyles[i] = lipgloss.NewStyle().Foreground(color).Background(t.Surface)
		pctStyles[i] = lipgloss.NewStyle().Foreground(color).Background(t.Surface).Bold(true)
	}

	for i, ms := range models[:limit] {
		barLen := 0
		if maxShare > 0 {
			barLen = int(ms.SharePercent / maxShare * float64(barMaxLen))
		}

		colorIdx := i % len(modelColors)
		modelBody.WriteString(nameStyle.Render(fmt.Sprintf("%-*s", nameW, shortModel(ms.Model))))
		modelBody.WriteString(sepStyle.Render(" "))
		modelBody.WriteString(barStyles[colorIdx].Render(strings.Repeat("█", barLen)))
		modelBody.WriteString(sepStyle.Render(" "))
		modelBody.WriteString(pctStyles[colorIdx].Render(fmt.Sprintf("%3.0f%%", ms.SharePercent)))
		modelBody.WriteString("\n")
	}

	// Activity patterns with time-of-day coloring
	now := time.Now()
	since := now.AddDate(0, 0, -a.days)
	hours := pipeline.AggregateHourly(a.filtered, since, now)

	type actBucket struct {
		label string
		total int
		color lipgloss.Color
	}
	buckets := []actBucket{
		{"Night   00-03", 0, t.Magenta},
		{"Early   04-07", 0, t.Orange},
		{"Morning 08-11", 0, t.GreenBright},
		{"Midday  12-15", 0, t.Green},
		{"Evening 16-19", 0, t.Cyan},
		{"Late    20-23", 0, t.Yellow},
	}
	for _, h := range hours {
		idx := h.Hour / 4
		if idx >= 6 {
			idx = 5
		}
		buckets[idx].total += h.Prompts
	}

	maxBucket := 0
	for _, bk := range buckets {
		if bk.total > maxBucket {
			maxBucket = bk.total
		}
	}

	actInnerW := components.CardInnerWidth(halves[1])

	// Compute number column width
	maxNumW := 5
	for _, bk := range buckets {
		if nw := len(cli.FormatNumber(int64(bk.total))); nw > maxNumW {
			maxNumW = nw
		}
	}
	actBarMax := actInnerW - 15 - maxNumW
	if actBarMax < 1 {
		actBarMax = 1
	}

	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	numStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)

	var actBody strings.Builder
	for _, bk := range buckets {
		bl := 0
		if maxBucket > 0 {
			bl = bk.total * actBarMax / maxBucket
		}
		barStyle := lipgloss.NewStyle().Foreground(bk.color).Background(t.Surface)
		actBody.WriteString(labelStyle.Render(bk.label))
		actBody.WriteString(sepStyle.Render(" "))
		actBody.WriteString(numStyle.Render(fmt.Sprintf("%*s", maxNumW, cli.FormatNumber(int64(bk.total)))))
		actBody.WriteString(sepStyle.Render(" "))
		actBody.WriteString(barStyle.Render(strings.Repeat("█", bl)))
		actBody.WriteString("\n")
	}

	modelCard := components.ContentCard("Model Split", modelBody.String(), halves[0])
	actCard := components.ContentCard("Activity", actBody.String(), halves[1])
	if a.isCompactLayout() {
		b.WriteString(components.ContentCard("Model Split", modelBody.String(), cw))
		b.WriteString("\n")
		b.WriteString(components.ContentCard("Activity", actBody.String(), cw))
	} else {
		b.WriteString(components.CardRow([]string{modelCard, actCard}))
	}

	return b.String()
}

// hourLabels24 returns X-axis labels for 24 hourly buckets.
func hourLabels24() []string {
	labels := make([]string, 24)
	for i := 0; i < 24; i++ {
		h := i % 12
		if h == 0 {
			h = 12
		}
		suffix := "a"
		if i >= 12 {
			suffix = "p"
		}
		labels[i] = fmt.Sprintf("%d%s", h, suffix)
	}
	return labels
}

// minuteLabels returns X-axis labels for 12 five-minute buckets.
func minuteLabels() []string {
	return []string{"-55", "-50", "-45", "-40", "-35", "-30", "-25", "-20", "-15", "-10", "-5", "now"}
}
