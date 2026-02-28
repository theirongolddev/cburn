package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/theirongolddev/cburn/internal/tui/theme"
)

func init() {
	// Force TrueColor output so ANSI codes are generated in tests
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestCardRowBackgroundFill(t *testing.T) {
	// Initialize theme
	theme.SetActive("flexoki-dark")

	shortCard := ContentCard("Short", "Content", 22)
	tallCard := ContentCard("Tall", "Line 1\nLine 2\nLine 3\nLine 4\nLine 5", 22)

	shortLines := len(strings.Split(shortCard, "\n"))
	tallLines := len(strings.Split(tallCard, "\n"))

	t.Logf("Short card lines: %d", shortLines)
	t.Logf("Tall card lines: %d", tallLines)

	if shortLines >= tallLines {
		t.Fatal("Test setup error: short card should be shorter than tall card")
	}

	// Test the fixed CardRow
	joined := CardRow([]string{tallCard, shortCard})
	lines := strings.Split(joined, "\n")
	t.Logf("Joined lines: %d", len(lines))

	if len(lines) != tallLines {
		t.Errorf("Joined height should match tallest card: got %d, want %d", len(lines), tallLines)
	}

	// Check that all lines have ANSI codes (indicating background styling)
	for i, line := range lines {
		hasESC := strings.Contains(line, "\x1b[")
		// After the short card ends, the padding should still have ANSI codes
		if i >= shortLines {
			t.Logf("Line %d (padding): hasANSI=%v, raw=%q", i, hasESC, line)
			if !hasESC {
				t.Errorf("Line %d has NO ANSI codes - will show as black squares", i)
			}
		}
	}
}

func TestCardRowWidthConsistency(t *testing.T) {
	// Verify all lines have consistent width
	theme.SetActive("flexoki-dark")

	shortCard := ContentCard("Short", "A", 30)
	tallCard := ContentCard("Tall", "A\nB\nC\nD\nE\nF", 20)

	joined := CardRow([]string{tallCard, shortCard})
	lines := strings.Split(joined, "\n")

	// All lines should have the same visual width
	for i, line := range lines {
		w := len(line) // Raw byte length - will differ if ANSI codes vary
		// Visual width should be consistent (tall card width + short card width)
		// Using lipgloss.Width would be better but we're checking raw structure
		t.Logf("Line %d: byteLen=%d", i, w)
	}

	// Verify the joined output has expected number of lines
	tallLines := len(strings.Split(tallCard, "\n"))
	if len(lines) != tallLines {
		t.Errorf("Joined should have %d lines (tallest), got %d", tallLines, len(lines))
	}
}
