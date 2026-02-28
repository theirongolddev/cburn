package tui

import "testing"

func TestTabAtXMatchesTabWidths(t *testing.T) {
	for active := 0; active < 5; active++ {
		a := App{activeTab: active}
		pos := 0

		for i := 0; i < 5; i++ {
			w := tabWidthForTest(i, active)
			x := pos + w/2 // midpoint inside this tab
			if got := a.tabAtX(x); got != i {
				t.Fatalf("active=%d x=%d -> tab=%d, want %d", active, x, got, i)
			}
			pos += w
			if i < 4 {
				pos++ // separator
			}
		}
	}
}

func tabWidthForTest(tabIdx, activeIdx int) int {
	nameWidths := []int{
		len("Overview"),
		len("Costs"),
		len("Sessions"),
		len("Breakdown"),
		len("Settings"),
	}

	w := nameWidths[tabIdx] + 2 // horizontal padding in tab renderer
	if tabIdx != activeIdx && tabIdx == 4 {
		w += 3 // inactive Settings adds "[x]"
	}
	return w
}
