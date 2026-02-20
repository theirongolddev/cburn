package tui

import (
	"fmt"
	"strings"

	"cburn/internal/cli"
	"cburn/internal/tui/components"
	"cburn/internal/tui/theme"

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

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

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

		for _, ms := range models {
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %8s %10s %5.1f%%",
				nameW,
				truncStr(shortModel(ms.Model), nameW),
				cli.FormatNumber(int64(ms.APICalls)),
				cli.FormatCost(ms.EstimatedCost),
				ms.SharePercent)))
			tableBody.WriteString("\n")
		}
	} else {
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %8s %10s %10s %10s %6s", nameW, "Model", "Calls", "Input", "Output", "Cost", "Share")))
		tableBody.WriteString("\n")

		for _, ms := range models {
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %8s %10s %10s %10s %5.1f%%",
				nameW,
				truncStr(shortModel(ms.Model), nameW),
				cli.FormatNumber(int64(ms.APICalls)),
				cli.FormatTokens(ms.InputTokens),
				cli.FormatTokens(ms.OutputTokens),
				cli.FormatCost(ms.EstimatedCost),
				ms.SharePercent)))
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

	headerStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)

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

		for _, ps := range projects {
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %6d %10s",
				nameW,
				truncStr(ps.Project, nameW),
				ps.Sessions,
				cli.FormatCost(ps.EstimatedCost))))
			tableBody.WriteString("\n")
		}
	} else {
		tableBody.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %6s %8s %10s %10s", nameW, "Project", "Sess.", "Prompts", "Tokens", "Cost")))
		tableBody.WriteString("\n")

		for _, ps := range projects {
			tableBody.WriteString(rowStyle.Render(fmt.Sprintf("%-*s %6d %8s %10s %10s",
				nameW,
				truncStr(ps.Project, nameW),
				ps.Sessions,
				cli.FormatNumber(int64(ps.Prompts)),
				cli.FormatTokens(ps.TotalTokens),
				cli.FormatCost(ps.EstimatedCost))))
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
