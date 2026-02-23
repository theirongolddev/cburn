package cmd

import (
	"fmt"

	"github.com/theirongolddev/cburn/internal/cli"
	"github.com/theirongolddev/cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Project usage ranking",
	RunE:  runProjects,
}

func init() {
	rootCmd.AddCommand(projectsCmd)
}

func runProjects(_ *cobra.Command, _ []string) error {
	result, err := loadData()
	if err != nil {
		return err
	}
	if len(result.Sessions) == 0 {
		fmt.Println("\n  No sessions found.")
		return nil
	}

	filtered, since, until := applyFilters(result.Sessions)
	projects := pipeline.AggregateProjects(filtered, since, until)

	if len(projects) == 0 {
		fmt.Println("\n  No project data in the selected time range.")
		return nil
	}

	fmt.Println()
	fmt.Println(cli.RenderTitle(fmt.Sprintf("PROJECTS  Last %dd", flagDays)))
	fmt.Println()

	rows := make([][]string, 0, len(projects))
	for _, ps := range projects {
		rows = append(rows, []string{
			truncate(ps.Project, 18),
			cli.FormatNumber(int64(ps.Sessions)),
			cli.FormatNumber(int64(ps.Prompts)),
			cli.FormatTokens(ps.TotalTokens),
			cli.FormatCost(ps.EstimatedCost),
		})
	}

	fmt.Print(cli.RenderTable(cli.Table{
		Headers: []string{"Project", "Sessions", "Prompts", "Tokens", "Cost"},
		Rows:    rows,
	}))

	return nil
}
