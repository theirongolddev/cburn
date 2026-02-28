package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/theirongolddev/cburn/internal/claudeai"
	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/model"
	"github.com/theirongolddev/cburn/internal/tui/components"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

func (a App) renderCostsTab(cw int) string {
	t := theme.Active
	stats := a.stats
	days := a.dailyStats
	modelCosts := a.modelCosts

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

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	valueStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)
	costValueStyle := lipgloss.NewStyle().Foreground(t.GreenBright).Background(t.Surface)
	modelNameStyle := lipgloss.NewStyle().Foreground(t.BlueBright).Background(t.Surface)
	tokenCostStyle := lipgloss.NewStyle().Foreground(t.Cyan).Background(t.Surface)
	spaceStyle := lipgloss.NewStyle().Background(t.Surface)

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

		for _, mc := range modelCosts {
			tableBody.WriteString(modelNameStyle.Render(fmt.Sprintf("%-*s", nameW, truncStr(shortModel(mc.Model), nameW))))
			tableBody.WriteString(costValueStyle.Render(fmt.Sprintf(" %10s", cli.FormatCost(mc.TotalCost))))
			tableBody.WriteString("\n")
		}
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+totalW+1)))
	} else {
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %10s %10s %10s %10s", nameW, "Model", "Input", "Output", "Cache", "Total")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
		tableBody.WriteString("\n")

		for _, mc := range modelCosts {
			tableBody.WriteString(modelNameStyle.Render(fmt.Sprintf("%-*s", nameW, truncStr(shortModel(mc.Model), nameW))))
			tableBody.WriteString(tokenCostStyle.Render(fmt.Sprintf(" %10s %10s %10s",
				cli.FormatCost(mc.InputCost),
				cli.FormatCost(mc.OutputCost),
				cli.FormatCost(mc.CacheCost))))
			tableBody.WriteString(costValueStyle.Render(fmt.Sprintf(" %10s", cli.FormatCost(mc.TotalCost))))
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
		body.WriteString(spaceStyle.Render(" "))
		body.WriteString(valueStyle.Render(fmt.Sprintf("%.0f%%", pct*100)))
		body.WriteString("\n")
		body.WriteString(labelStyle.Render("Used"))
		body.WriteString(spaceStyle.Render("  "))
		body.WriteString(valueStyle.Render(fmt.Sprintf("$%.2f", ol.UsedCredits)))
		body.WriteString(spaceStyle.Render(" / "))
		body.WriteString(valueStyle.Render(fmt.Sprintf("$%.2f", ol.MonthlyCreditLimit)))
		body.WriteString(spaceStyle.Render(" "))
		body.WriteString(labelStyle.Render(ol.Currency))

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
			spendBody.WriteString(valueStyle.Render(d.Date.Format("Jan 02")))
			spendBody.WriteString(spaceStyle.Render("  "))
			spendBody.WriteString(lipgloss.NewStyle().Foreground(t.Green).Background(t.Surface).Render(cli.FormatCost(d.EstimatedCost)))
			spendBody.WriteString("\n")
		}
	} else {
		spendBody.WriteString(labelStyle.Render("No data"))
		spendBody.WriteString("\n")
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

	effMetrics := []struct {
		name  string
		value string
		color lipgloss.Color
	}{
		{"Tokens/Prompt", cli.FormatTokens(tokPerPrompt), t.Cyan},
		{"Output/Prompt", cli.FormatTokens(outPerPrompt), t.Cyan},
		{"Prompts/Session", fmt.Sprintf("%.1f", promptsPerSess), t.Magenta},
		{"Minutes/Day", fmt.Sprintf("%.0f", stats.MinutesPerDay), t.Yellow},
	}

	var effBody strings.Builder
	for _, m := range effMetrics {
		effBody.WriteString(labelStyle.Render(fmt.Sprintf("%-20s", m.name)))
		effBody.WriteString(lipgloss.NewStyle().Foreground(m.color).Background(t.Surface).Render(fmt.Sprintf(" %10s", m.value)))
		effBody.WriteString("\n")
	}

	b.WriteString(components.ContentCard("Efficiency", effBody.String(), cw))

	return b.String()
}

// renderSubscriptionCard renders the rate limit + overage card at the top of the costs tab.
func (a App) renderSubscriptionCard(cw int) string {
	t := theme.Active
	hintStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)

	// No session key configured
	if a.subData == nil && !a.subFetching {
		cfg := loadConfigOrDefault()
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
		warnStyle := lipgloss.NewStyle().Foreground(t.Orange).Background(t.Surface)
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
		body.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface).Render(strings.Repeat("─", innerW)))
		body.WriteString("\n")
		body.WriteString(components.RateLimitBar("Overage",
			pct, time.Time{}, labelW, barW))

		spendStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
		body.WriteString(spendStyle.Render(
			fmt.Sprintf("  $%.2f / $%.2f", ol.UsedCredits, ol.MonthlyCreditLimit)))
	}

	// Fetch timestamp
	if !a.subData.FetchedAt.IsZero() {
		body.WriteString("\n")
		tsStyle := lipgloss.NewStyle().Foreground(t.TextDim).Background(t.Surface)
		body.WriteString(tsStyle.Render("Updated " + a.subData.FetchedAt.Format("3:04 PM")))
	}

	title := "Subscription"
	if a.subData.Org.Name != "" {
		title = "Subscription — " + a.subData.Org.Name
	}

	return components.ContentCard(title, body.String(), cw) + "\n"
}
