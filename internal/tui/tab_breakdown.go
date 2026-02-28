package tui

import (
	"fmt"
	"strings"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/tui/components"
	"github.com/theirongolddev/cburn/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

func (a App) renderModelsTab(cw int) string {
	t := theme.Active
	models := a.models

	innerW := components.CardInnerWidth(cw)
	fixedCols := 8 + 10 + 10 + 10 + 6 // Calls, Input, Output, Cost, Share
	gaps := 5
	nameW := innerW - fixedCols - gaps
	if nameW < 14 {
		nameW = 14
	}

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	costStyle := lipgloss.NewStyle().Foreground(t.GreenBright).Background(t.Surface)
	shareStyle := lipgloss.NewStyle().Foreground(t.Cyan).Background(t.Surface)

	// Model colors for visual interest - pre-compute styles to avoid allocation in loops
	modelColors := []lipgloss.Color{t.BlueBright, t.Cyan, t.Magenta, t.Yellow, t.Green}
	nameStyles := make([]lipgloss.Style, len(modelColors))
	for i, color := range modelColors {
		nameStyles[i] = lipgloss.NewStyle().Foreground(color).Background(t.Surface)
	}

	var tableBody strings.Builder
	if a.isCompactLayout() {
		shareW := 6
		costW := 10
		callW := 8
		nameW = innerW - shareW - costW - callW - 3
		if nameW < 10 {
			nameW = 10
		}
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %8s %10s %6s", nameW, "Model", "Calls", "Cost", "Share")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+shareW+costW+callW+3)))
		tableBody.WriteString("\n")

		for i, ms := range models {
			tableBody.WriteString(nameStyles[i%len(modelColors)].Render(fmt.Sprintf("%-*s", nameW, truncStr(shortModel(ms.Model), nameW))))
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf(" %8s", cli.FormatNumber(int64(ms.APICalls)))))
			tableBody.WriteString(costStyle.Render(fmt.Sprintf(" %10s", cli.FormatCost(ms.EstimatedCost))))
			tableBody.WriteString(shareStyle.Render(fmt.Sprintf(" %5.1f%%", ms.SharePercent)))
			tableBody.WriteString("\n")
		}
	} else {
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %8s %10s %10s %10s %6s", nameW, "Model", "Calls", "Input", "Output", "Cost", "Share")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
		tableBody.WriteString("\n")

		for i, ms := range models {
			tableBody.WriteString(nameStyles[i%len(modelColors)].Render(fmt.Sprintf("%-*s", nameW, truncStr(shortModel(ms.Model), nameW))))
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf(" %8s %10s %10s",
				cli.FormatNumber(int64(ms.APICalls)),
				cli.FormatTokens(ms.InputTokens),
				cli.FormatTokens(ms.OutputTokens))))
			tableBody.WriteString(costStyle.Render(fmt.Sprintf(" %10s", cli.FormatCost(ms.EstimatedCost))))
			tableBody.WriteString(shareStyle.Render(fmt.Sprintf(" %5.1f%%", ms.SharePercent)))
			tableBody.WriteString("\n")
		}
	}

	return components.ContentCard("Model Usage", tableBody.String(), cw)
}

func (a App) renderProjectsTab(cw int) string {
	t := theme.Active
	projects := a.projects

	innerW := components.CardInnerWidth(cw)
	fixedCols := 6 + 8 + 10 + 10 // Sess, Prompts, Tokens, Cost
	gaps := 4
	nameW := innerW - fixedCols - gaps
	if nameW < 18 {
		nameW = 18
	}

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Background(t.Surface).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary).Background(t.Surface)
	mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Background(t.Surface)
	nameStyle := lipgloss.NewStyle().Foreground(t.Cyan).Background(t.Surface)
	costStyle := lipgloss.NewStyle().Foreground(t.GreenBright).Background(t.Surface)

	var tableBody strings.Builder
	if a.isCompactLayout() {
		costW := 10
		sessW := 6
		nameW = innerW - costW - sessW - 2
		if nameW < 12 {
			nameW = 12
		}
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %6s %10s", nameW, "Project", "Sess.", "Cost")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", nameW+costW+sessW+2)))
		tableBody.WriteString("\n")

		for _, ps := range projects {
			tableBody.WriteString(nameStyle.Render(fmt.Sprintf("%-*s", nameW, truncStr(ps.Project, nameW))))
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf(" %6d", ps.Sessions)))
			tableBody.WriteString(costStyle.Render(fmt.Sprintf(" %10s", cli.FormatCost(ps.EstimatedCost))))
			tableBody.WriteString("\n")
		}
	} else {
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %6s %8s %10s %10s", nameW, "Project", "Sess.", "Prompts", "Tokens", "Cost")))
		tableBody.WriteString("\n")
		tableBody.WriteString(mutedStyle.Render(strings.Repeat("─", innerW)))
		tableBody.WriteString("\n")

		for _, ps := range projects {
			tableBody.WriteString(nameStyle.Render(fmt.Sprintf("%-*s", nameW, truncStr(ps.Project, nameW))))
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf(" %6d %8s %10s",
				ps.Sessions,
				cli.FormatNumber(int64(ps.Prompts)),
				cli.FormatTokens(ps.TotalTokens))))
			tableBody.WriteString(costStyle.Render(fmt.Sprintf(" %10s", cli.FormatCost(ps.EstimatedCost))))
			tableBody.WriteString("\n")
		}
	}

	return components.ContentCard("Projects", tableBody.String(), cw)
}

func (a App) renderBreakdownTab(cw int) string {
	var b strings.Builder
	b.WriteString(a.renderModelsTab(cw))
	b.WriteString("\n")
	b.WriteString(a.renderProjectsTab(cw))
	return b.String()
}
