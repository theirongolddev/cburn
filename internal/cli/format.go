// Package cli provides formatting and rendering utilities for terminal output.
package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FormatTokens formats a token count with human-readable suffixes.
// e.g., 1234 -> "1.2K", 1234567 -> "1.2M", 1234567890 -> "1.2B"
func FormatTokens(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}

	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// FormatCost formats a USD cost value.
func FormatCost(cost float64) string {
	if cost >= 1000 {
		return "$" + FormatNumber(int64(math.Round(cost)))
	}
	if cost >= 100 {
		return fmt.Sprintf("$%.0f", cost)
	}
	if cost >= 10 {
		return fmt.Sprintf("$%.1f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// FormatDuration formats seconds into a human-readable duration.
// e.g., 3725 -> "1h 2m", 125 -> "2m", 45 -> "45s"
func FormatDuration(secs int64) string {
	if secs <= 0 {
		return "0s"
	}

	hours := secs / 3600
	mins := (secs % 3600) / 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%ds", secs)
}

// FormatNumber adds comma separators to an integer.
// e.g., 1234567 -> "1,234,567"
func FormatNumber(n int64) string {
	if n < 0 {
		return "-" + FormatNumber(-n)
	}

	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if result.Len() > 0 {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

// FormatPercent formats a 0-1 float as a percentage string.
func FormatPercent(f float64) string {
	return fmt.Sprintf("%.1f%%", f*100)
}

// FormatDelta formats a cost delta with sign and color hint.
// Returns the formatted string and whether it's positive.
func FormatDelta(current, previous float64) string {
	delta := current - previous
	if delta >= 0 {
		return "+" + FormatCost(delta)
	}
	return "-" + FormatCost(-delta)
}

// FormatDayOfWeek returns a 3-letter day abbreviation from a weekday number.
func FormatDayOfWeek(weekday int) string {
	days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	if weekday >= 0 && weekday < 7 {
		return days[weekday]
	}
	return "???"
}
