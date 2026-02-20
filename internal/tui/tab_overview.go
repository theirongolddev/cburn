package tui

import (
	"fmt"
	"strings"
	"time"

	"cburn/internal/cli"
	"cburn/internal/pipeline"
	"cburn/internal/tui/components"
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

func (a App) renderOverviewTab(cw int) string {
	t := theme.Active
	stats := a.stats
	prev := a.prevStats
	days := a.dailyStats
	models := a.models
	var b strings.Builder

	// Row 1: Metric cards
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

	// Row 2: Daily token usage chart
	if len(days) > 0 {
		chartVals := make([]float64, len(days))
		chartLabels := chartDateLabels(days)
		for i, d := range days {
			chartVals[len(days)-1-i] = float64(d.InputTokens + d.OutputTokens + d.CacheCreation5m + d.CacheCreation1h)
		}
		chartInnerW := components.CardInnerWidth(cw)
		b.WriteString(components.ContentCard(
			fmt.Sprintf("Daily Token Usage (%dd)", a.days),
			components.BarChart(chartVals, chartLabels, t.Blue, chartInnerW, 10),
			cw,
		))
		b.WriteString("\n")
	}

	// Row 3: Model Split + Activity Patterns
	halves := components.LayoutRow(cw, 2)
	innerW := components.CardInnerWidth(halves[0])

	nameStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	barStyle := lipgloss.NewStyle().Foreground(t.Accent)
	pctStyle := lipgloss.NewStyle().Foreground(t.TextDim)

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
	for _, ms := range models[:limit] {
		barLen := 0
		if maxShare > 0 {
			barLen = int(ms.SharePercent / maxShare * float64(barMaxLen))
		}
		fmt.Fprintf(&modelBody, "%s %s %s\n",
			nameStyle.Render(fmt.Sprintf("%-*s", nameW, shortModel(ms.Model))),
			barStyle.Render(strings.Repeat("█", barLen)),
			pctStyle.Render(fmt.Sprintf("%.0f%%", ms.SharePercent)))
	}

	// Compact activity: aggregate prompts into 4-hour buckets
	now := time.Now()
	since := now.AddDate(0, 0, -a.days)
	hours := pipeline.AggregateHourly(a.filtered, since, now)

	type actBucket struct {
		label string
		total int
		color lipgloss.Color
	}
	buckets := []actBucket{
		{"Night   00-03", 0, t.Red},
		{"Early   04-07", 0, t.Yellow},
		{"Morning 08-11", 0, t.Green},
		{"Midday  12-15", 0, t.Green},
		{"Evening 16-19", 0, t.Green},
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

	// Compute number column width from actual data so bars never overflow.
	maxNumW := 5
	for _, bk := range buckets {
		if nw := len(cli.FormatNumber(int64(bk.total))); nw > maxNumW {
			maxNumW = nw
		}
	}
	// prefix = 13 (label) + 1 (space) + maxNumW (number) + 1 (space)
	actBarMax := actInnerW - 15 - maxNumW
	if actBarMax < 1 {
		actBarMax = 1
	}

	numStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	var actBody strings.Builder
	for _, bk := range buckets {
		bl := 0
		if maxBucket > 0 {
			bl = bk.total * actBarMax / maxBucket
		}
		bar := lipgloss.NewStyle().Foreground(bk.color).Render(strings.Repeat("█", bl))
		fmt.Fprintf(&actBody, "%s %s %s\n",
			numStyle.Render(bk.label),
			numStyle.Render(fmt.Sprintf("%*s", maxNumW, cli.FormatNumber(int64(bk.total)))),
			bar)
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
