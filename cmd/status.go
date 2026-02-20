package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cburn/internal/claudeai"
	"cburn/internal/cli"
	"cburn/internal/config"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show claude.ai subscription status and rate limits",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, _ := config.Load()
	sessionKey := config.GetSessionKey(cfg)
	if sessionKey == "" {
		fmt.Println()
		fmt.Println("  No session key configured.")
		fmt.Println()
		fmt.Println("  To get your session key:")
		fmt.Println("    1. Open claude.ai in your browser")
		fmt.Println("    2. DevTools (F12) > Application > Cookies > claude.ai")
		fmt.Println("    3. Copy the 'sessionKey' value (starts with sk-ant-sid...)")
		fmt.Println()
		fmt.Println("  Then configure it:")
		fmt.Println("    cburn setup                                     (interactive)")
		fmt.Println("    CLAUDE_SESSION_KEY=sk-ant-sid... cburn status    (one-shot)")
		fmt.Println()
		return nil
	}

	client := claudeai.NewClient(sessionKey)
	if client == nil {
		return errors.New("invalid session key format (expected sk-ant-sid... prefix)")
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "  Fetching subscription data...\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data := client.FetchAll(ctx)

	if data.Error != nil {
		if errors.Is(data.Error, claudeai.ErrUnauthorized) {
			return errors.New("session key expired or invalid — grab a fresh one from claude.ai cookies")
		}
		if errors.Is(data.Error, claudeai.ErrRateLimited) {
			return errors.New("rate limited by claude.ai — try again in a minute")
		}
		// Partial data may still be available, continue rendering
		if data.Usage == nil && data.Overage == nil {
			return fmt.Errorf("fetch failed: %w", data.Error)
		}
	}

	fmt.Println()
	fmt.Println(cli.RenderTitle("CLAUDE.AI STATUS"))
	fmt.Println()

	// Organization info
	if data.Org.UUID != "" {
		fmt.Printf("  Organization: %s\n", data.Org.Name)
		if len(data.Org.Capabilities) > 0 {
			fmt.Printf("  Capabilities: %s\n", strings.Join(data.Org.Capabilities, ", "))
		}
		fmt.Println()
	}

	// Rate limits
	if data.Usage != nil {
		rows := [][]string{}

		if w := data.Usage.FiveHour; w != nil {
			rows = append(rows, rateLimitRow("5-hour window", w))
		}
		if w := data.Usage.SevenDay; w != nil {
			rows = append(rows, rateLimitRow("7-day (all)", w))
		}
		if w := data.Usage.SevenDayOpus; w != nil {
			rows = append(rows, rateLimitRow("7-day Opus", w))
		}
		if w := data.Usage.SevenDaySonnet; w != nil {
			rows = append(rows, rateLimitRow("7-day Sonnet", w))
		}

		if len(rows) > 0 {
			fmt.Print(cli.RenderTable(cli.Table{
				Title:   "Rate Limits",
				Headers: []string{"Window", "Used", "Bar", "Resets"},
				Rows:    rows,
			}))
		}
	}

	// Overage
	if data.Overage != nil {
		ol := data.Overage
		status := "disabled"
		if ol.IsEnabled {
			status = "enabled"
		}

		rows := [][]string{
			{"Overage", status},
			{"Used Credits", fmt.Sprintf("%.2f %s", ol.UsedCredits, ol.Currency)},
			{"Monthly Limit", fmt.Sprintf("%.2f %s", ol.MonthlyCreditLimit, ol.Currency)},
		}

		if ol.IsEnabled && ol.MonthlyCreditLimit > 0 {
			pct := ol.UsedCredits / ol.MonthlyCreditLimit
			rows = append(rows, []string{"Usage", fmt.Sprintf("%.1f%%", pct*100)})
		}

		fmt.Print(cli.RenderTable(cli.Table{
			Title:   "Overage Spend",
			Headers: []string{"Setting", "Value"},
			Rows:    rows,
		}))
	}

	// Partial error warning
	if data.Error != nil {
		warnStyle := lipgloss.NewStyle().Foreground(cli.ColorOrange)
		fmt.Printf("  %s\n\n", warnStyle.Render(fmt.Sprintf("Partial data — %s", data.Error)))
	}

	fmt.Printf("  Fetched at %s\n\n", data.FetchedAt.Format("3:04:05 PM"))

	return nil
}

func rateLimitRow(label string, w *claudeai.ParsedWindow) []string {
	pctStr := fmt.Sprintf("%.0f%%", w.Pct*100)
	bar := renderMiniBar(w.Pct, 20)
	resets := ""
	if !w.ResetsAt.IsZero() {
		dur := time.Until(w.ResetsAt)
		if dur > 0 {
			resets = formatCountdown(dur)
		} else {
			resets = "now"
		}
	}
	return []string{label, pctStr, bar, resets}
}

func renderMiniBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	empty := width - filled

	// Color based on usage level
	color := cli.ColorGreen
	if pct >= 0.8 {
		color = cli.ColorRed
	} else if pct >= 0.5 {
		color = cli.ColorOrange
	}

	barStyle := lipgloss.NewStyle().Foreground(color)
	dimStyle := lipgloss.NewStyle().Foreground(cli.ColorTextDim)

	return barStyle.Render(strings.Repeat("█", filled)) +
		dimStyle.Render(strings.Repeat("░", empty))
}

func formatCountdown(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
