package cmd

import (
	"fmt"
	"sort"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Session list with details",
	RunE:  runSessions,
}

var sessionsLimit int

func init() {
	sessionsCmd.Flags().IntVarP(&sessionsLimit, "limit", "l", 20, "Number of sessions to show")
	rootCmd.AddCommand(sessionsCmd)
}

func runSessions(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}
	if len(result.Sessions) == 0 {
		fmt.Println("\n  No sessions found.")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	sessions := pipeline.FilterByTime(filtered, since, until)

	if len(sessions) == 0 {
		fmt.Println("\n  No sessions in the selected time range.")
		return nil
	}

	// Sort by start time descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	// Limit
	if sessionsLimit > 0 && len(sessions) > sessionsLimit {
		sessions = sessions[:sessionsLimit]
	}

	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("SESSIONS  Last %dd (showing %d)", flagDays, len(sessions))))
	fmt.Println()

	rows := make([][]string, 0, len(sessions))
	for _, s := range sessions {
		startStr := ""
		if !s.StartTime.IsZero() {
			startStr = s.StartTime.Local().Format("Jan 02 15:04")
		}

		totalTokens := s.InputTokens + s.OutputTokens +
			s.CacheCreation5mTokens + s.CacheCreation1hTokens

		project := s.Project
		if s.IsSubagent {
			project += " (sub)"
		}

		rows = append(rows, []string{
			startStr,
			truncate(project, 14),
			cli.FormatDuration(s.DurationSecs),
			cli.FormatTokens(totalTokens),
			cli.FormatCost(s.EstimatedCost),
		})
	}

	fmt.Print(cli.RenderTable(cli.Table{
		Headers: []string{"Start", "Project", "Duration", "Tokens", "Cost"},
		Rows:    rows,
	}))

	return nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
