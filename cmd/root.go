package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cburn/internal/cli"
	"cburn/internal/model"
	"cburn/internal/pipeline"
	"cburn/internal/store"

	"github.com/spf13/cobra"
)

var (
	flagDays        int
	flagProject     string
	flagModel       string
	flagNoCache     bool
	flagDataDir     string
	flagQuiet       bool
	flagNoSubagents bool
)

var rootCmd = &cobra.Command{
	Use:   "cburn",
	Short: "Claude Usage Metrics CLI",
	Long:  "Analyze your Claude Code usage: tokens, costs, sessions, and more.",
	RunE:  runSummary,
}

// Execute is the main entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	homeDir, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(homeDir, ".claude")

	rootCmd.PersistentFlags().IntVarP(&flagDays, "days", "n", 30, "Time window in days")
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Filter to project (substring match)")
	rootCmd.PersistentFlags().StringVarP(&flagModel, "model", "m", "", "Filter to model (substring match)")
	rootCmd.PersistentFlags().BoolVar(&flagNoCache, "no-cache", false, "Skip SQLite cache, reparse everything")
	rootCmd.PersistentFlags().StringVarP(&flagDataDir, "data-dir", "d", defaultDataDir, "Claude data directory")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress progress output")
	rootCmd.PersistentFlags().BoolVar(&flagNoSubagents, "no-subagents", false, "Exclude subagent sessions")
}

// loadData is the shared data loading path used by all commands.
// Uses SQLite cache when available for fast subsequent runs.
func loadData() (*pipeline.LoadResult, error) {
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "  Scanning sessions...\n")
	}

	progressFn := func(current, total int) {
		if flagQuiet {
			return
		}
		if current%100 == 0 || current == total {
			fmt.Fprintf(os.Stderr, "\r  Parsing [%d/%d]", current, total)
		}
	}

	// Try cached load unless --no-cache
	if !flagNoCache {
		cache, err := store.Open(pipeline.CachePath())
		if err != nil {
			// Cache open failed — fall back to uncached
			if !flagQuiet {
				fmt.Fprintf(os.Stderr, "  Cache unavailable, doing full parse\n")
			}
		} else {
			defer cache.Close()

			cr, err := pipeline.LoadWithCache(flagDataDir, !flagNoSubagents, cache, progressFn)
			if err != nil {
				// Cache-assisted load failed — fall back
				if !flagQuiet {
					fmt.Fprintf(os.Stderr, "\n  Cache error, falling back to full parse\n")
				}
			} else {
				if !flagQuiet && cr.TotalFiles > 0 {
					if cr.Reparsed == 0 {
						fmt.Fprintf(os.Stderr, "\r  Loaded %s sessions from cache (%d projects)    \n",
							formatNumber(int64(len(cr.Sessions))),
							cr.ProjectCount,
						)
					} else {
						fmt.Fprintf(os.Stderr, "\r  %s cached + %d reparsed (%d projects)    \n",
							formatNumber(int64(cr.CacheHits)),
							cr.Reparsed,
							cr.ProjectCount,
						)
					}
				}
				return &cr.LoadResult, nil
			}
		}
	}

	// Uncached path
	result, err := pipeline.Load(flagDataDir, !flagNoSubagents, progressFn)
	if err != nil {
		return nil, err
	}

	if !flagQuiet && result.TotalFiles > 0 {
		fmt.Fprintf(os.Stderr, "\r  Parsed %s sessions across %d projects    \n",
			formatNumber(int64(result.ParsedFiles)),
			result.ProjectCount,
		)
	}

	return result, nil
}

// applyFilters returns filtered sessions and the computed time range.
func applyFilters(sessions []model.SessionStats) ([]model.SessionStats, time.Time, time.Time) {
	now := time.Now()
	since := now.AddDate(0, 0, -flagDays)
	until := now

	filtered := sessions
	if flagProject != "" {
		filtered = pipeline.FilterByProject(filtered, flagProject)
	}
	if flagModel != "" {
		filtered = pipeline.FilterByModel(filtered, flagModel)
	}

	return filtered, since, until
}

func formatNumber(n int64) string {
	return cli.FormatNumber(n)
}
