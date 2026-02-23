package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cburn/internal/claudeai"
	"cburn/internal/cli"
	"cburn/internal/config"
	"cburn/internal/model"
	"cburn/internal/tui/components"
	"cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

func (a App) renderCostsTab(cw int) string {
	t := theme.Active
	stats := a.stats
	models := a.models
	days := a.dailyStats

	var b strings.Builder

	// Row 0: Subscription rate limits (live data from claude.ai)
	b.WriteString(a.renderSubscriptionCard(cw))

	// Row 1: Cost metric cards
	savingsMultiplier := 0.0
	if stats.EstimatedCost > 0 {
		savingsMultiplier = stats.CacheSavings / stats.EstimatedCost
	}
	costCards := []struct{ Label, Value, Delta string }{
		{"Total Cost", cli.FormatCost(stats.EstimatedCost), cli.FormatCost(stats.CostPerDay) + "/day"},
		{"Cache Savings", cli.FormatCost(stats.CacheSavings), fmt.Sprintf("%.1fx cost", savingsMultiplier)},
		{"Projected", cli.FormatCost(stats.CostPerDay*30) + "/mo", cli.FormatCost(stats.CostPerDay) + "/day"},
		{"Cache Rate", cli.FormatPercent(stats.CacheHitRate), ""},
	}
	b.WriteString(components.MetricCardRow(costCards, cw))
	b.WriteString("\n")

	// Row 2: Cost breakdown table
	innerW := components.CardInnerWidth(cw)
	fixedCols := 10 + 10 + 10 + 10
	gaps := 4
	nameW := innerW - fixedCols - gaps
	if nameW < 14 {
		nameW = 14
	}

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

	var tableBody strings.Builder
	if a.isCompactLayout() {
		totalW := 10
		nameW = innerW - totalW - 1
		if nameW < 10 {
			nameW = 10
		}
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %10s", nameW, "Model", "Total")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+totalW+1)))
		tableBody.WriteString("\n")

		for _, ms := range models {
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %10s",
				nameW,
				truncStr(shortModel(ms.Model), nameW),
				cli.FormatCost(ms.EstimatedCost))))
			tableBody.WriteString("\n")
		}
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+totalW+1)))
	} else {
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %10s %10s %10s %10s", nameW, "Model", "Input", "Output", "Cache", "Total")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
		tableBody.WriteString("\n")

		for _, ms := range models {
			inputCost := 0.0
			outputCost := 0.0
			if p, ok := config.LookupPricing(ms.Model); ok {
				inputCost = float64(ms.InputTokens) * p.InputPerMTok / 1e6
				outputCost = float64(ms.OutputTokens) * p.OutputPerMTok / 1e6
			}
			cacheCost := ms.EstimatedCost - inputCost - outputCost

			tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %10s %10s %10s %10s",
				nameW,
				truncStr(shortModel(ms.Model), nameW),
				cli.FormatCost(inputCost),
				cli.FormatCost(outputCost),
				cli.FormatCost(cacheCost),
				cli.FormatCost(ms.EstimatedCost))))
			tableBody.WriteString("\n")
		}

		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
	}

	title := fmt.Sprintf("Cost Breakdown  %s (%dd)", cli.FormatCost(stats.EstimatedCost), a.days)
	b.WriteString(components.ContentCard(title, tableBody.String(), cw))
	b.WriteString("\n")

	// Row 3: Budget progress + Top Spend Days
	halves := components.LayoutRow(cw, 2)

	// Use real overage data if available, otherwise show placeholder
	var progressCard string
	if a.subData != nil && a.subData.Overage != nil && a.subData.Overage.IsEnabled {
		ol := a.subData.Overage
		pct := 0.0
		if ol.MonthlyCreditLimit > 0 {
			pct = ol.UsedCredits / ol.MonthlyCreditLimit
		}

		barW := components.CardInnerWidth(halves[0]) - 10
		if barW < 10 {
			barW = 10
		}
		bar := progress.New(
			progress.WithSolidFill(components.ColorForPct(pct)),
			progress.WithWidth(barW),
			progress.WithoutPercentage(),
		)
		bar.EmptyColor = string(t.TextDim)

		var body strings.Builder
		body.WriteString(bar.ViewAs(pct))
		fmt.Fprintf(&body, " %.0f%%\n", pct*100)
		fmt.Fprintf(&body, "%s  %s / %s %s",
			labelStyle.Render("Used"),
			valueStyle.Render(fmt.Sprintf("$%.2f", ol.UsedCredits)),
			valueStyle.Render(fmt.Sprintf("$%.2f", ol.MonthlyCreditLimit)),
			labelStyle.Render(ol.Currency))

		progressCard = components.ContentCard("Overage Spend", body.String(), halves[0])
	} else {
		ceiling := 200.0
		pct := stats.EstimatedCost / ceiling
		progressInnerW := components.CardInnerWidth(halves[0])
		progressBody := components.ProgressBar(pct, progressInnerW-10) + "\n" +
			labelStyle.Render("flat-rate plan ceiling")
		progressCard = components.ContentCard("Budget Progress", progressBody, halves[0])
	}

	var spendBody strings.Builder
	if len(days) > 0 {
		spendLimit := 5
		if len(days) < spendLimit {
			spendLimit = len(days)
		}
		sorted := make([]model.DailyStats, len(days))
		copy(sorted, days)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].EstimatedCost > sorted[j].EstimatedCost
		})
		topDays := sorted[:spendLimit]
		sort.Slice(topDays, func(i, j int) bool {
			return topDays[i].Date.After(topDays[j].Date)
		})
		for _, d := range topDays {
			fmt.Fprintf(&spendBody, "%s  %s\n",
				valueStyle.Render(d.Date.Format("Jan 02")),
				lipgloss.NewStyle().Foreground(t.Green).Render(cli.FormatCost(d.EstimatedCost)))
		}
	} else {
		spendBody.WriteString("No data\n")
	}
	spendCard := components.ContentCard("Top Spend Days", spendBody.String(), halves[1])

	if a.isCompactLayout() {
		b.WriteString(progressCard)
		b.WriteString("\n")
		b.WriteString(components.ContentCard("Top Spend Days", spendBody.String(), cw))
	} else {
		b.WriteString(components.CardRow([]string{progressCard, spendCard}))
	}
	b.WriteString("\n")

	// Row 4: Efficiency metrics
	tokPerPrompt := int64(0)
	outPerPrompt := int64(0)
	if stats.TotalPrompts > 0 {
		tokPerPrompt = (stats.InputTokens + stats.OutputTokens) / int64(stats.TotalPrompts)
		outPerPrompt = stats.OutputTokens / int64(stats.TotalPrompts)
	}
	promptsPerSess := 0.0
	if stats.TotalSessions > 0 {
		promptsPerSess = float64(stats.TotalPrompts) / float64(stats.TotalSessions)
	}

	effMetrics := []struct{ name, value string }{
		{"Tokens/Prompt", cli.FormatTokens(tokPerPrompt)},
		{"Output/Prompt", cli.FormatTokens(outPerPrompt)},
		{"Prompts/Session", fmt.Sprintf("%.1f", promptsPerSess)},
		{"Minutes/Day", fmt.Sprintf("%.0f", stats.MinutesPerDay)},
	}

	var effBody strings.Builder
	for _, m := range effMetrics {
		effBody.WriteString(rowStyle.Render(fmt.Sprintf("%-20s %10s", m.name, m.value)))
		effBody.WriteString("\n")
	}

	b.WriteString(components.ContentCard("Efficiency", effBody.String(), cw))

	return b.String()
}

// renderSubscriptionCard renders the rate limit + overage card at the top of the costs tab.
func (a App) renderSubscriptionCard(cw int) string {
	t := theme.Active
	hintStyle := lipgloss.NewStyle().Foreground(t.TextDim)

	// No session key configured
	if a.subData == nil && !a.subFetching {
		cfg, _ := config.Load()
		if config.GetSessionKey(cfg) == "" {
			return components.ContentCard("Subscription",
				hintStyle.Render("Configure session key in Settings to see rate limits"),
				cw) + "\n"
		}
		// Key configured but no data yet (initial fetch in progress)
		return components.ContentCard("Subscription",
			hintStyle.Render("Fetching rate limits..."),
			cw) + "\n"
	}

	// Still fetching
	if a.subData == nil {
		return components.ContentCard("Subscription",
			hintStyle.Render("Fetching rate limits..."),
			cw) + "\n"
	}

	// Error with no usable data
	if a.subData.Usage == nil && a.subData.Error != nil {
		warnStyle := lipgloss.NewStyle().Foreground(t.Orange)
		return components.ContentCard("Subscription",
			warnStyle.Render(fmt.Sprintf("Error: %s", a.subData.Error)),
			cw) + "\n"
	}

	// No usage data at all
	if a.subData.Usage == nil {
		return ""
	}

	innerW := components.CardInnerWidth(cw)
	labelW := 13                 // enough for "Weekly Sonnet"
	barW := innerW - labelW - 16 // label + bar + pct(5) + countdown(~10) + gaps
	if barW < 10 {
		barW = 10
	}

	var body strings.Builder

	type windowRow struct {
		label  string
		window *claudeai.ParsedWindow
	}

	rows := []windowRow{}
	if w := a.subData.Usage.FiveHour; w != nil {
		rows = append(rows, windowRow{"5-hour", w})
	}
	if w := a.subData.Usage.SevenDay; w != nil {
		rows = append(rows, windowRow{"Weekly", w})
	}
	if w := a.subData.Usage.SevenDayOpus; w != nil {
		rows = append(rows, windowRow{"Weekly Opus", w})
	}
	if w := a.subData.Usage.SevenDaySonnet; w != nil {
		rows = append(rows, windowRow{"Weekly Sonnet", w})
	}

	for i, r := range rows {
		body.WriteString(components.RateLimitBar(r.label, r.window.Pct, r.window.ResetsAt, labelW, barW))
		if i < len(rows)-1 {
			body.WriteString("\n")
		}
	}

	// Overage line if enabled
	if ol := a.subData.Overage; ol != nil && ol.IsEnabled && ol.MonthlyCreditLimit > 0 {
		pct := ol.UsedCredits / ol.MonthlyCreditLimit
		body.WriteString("\n")
		body.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(strings.Repeat("─", innerW)))
		body.WriteString("\n")
		body.WriteString(components.RateLimitBar("Overage",
			pct, time.Time{}, labelW, barW))

		spendStyle := lipgloss.NewStyle().Foreground(t.TextDim)
		body.WriteString(spendStyle.Render(
			fmt.Sprintf("  $%.2f / $%.2f", ol.UsedCredits, ol.MonthlyCreditLimit)))
	}

	// Fetch timestamp
	if !a.subData.FetchedAt.IsZero() {
		body.WriteString("\n")
		tsStyle := lipgloss.NewStyle().Foreground(t.TextDim)
		body.WriteString(tsStyle.Render("Updated " + a.subData.FetchedAt.Format("3:04 PM")))
	}

	title := "Subscription"
	if a.subData.Org.Name != "" {
		title = "Subscription — " + a.subData.Org.Name
	}

	return components.ContentCard(title, body.String(), cw) + "\n"
}
