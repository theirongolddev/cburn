// Package theme defines color themes for the cburn TUI dashboard.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color roles used throughout the TUI.
type Theme struct {
	Name        string
	Background  lipgloss.Color
	Surface     lipgloss.Color
	Border      lipgloss.Color
	TextDim     lipgloss.Color
	TextMuted   lipgloss.Color
	TextPrimary lipgloss.Color
	Accent      lipgloss.Color
	Green       lipgloss.Color
	Orange      lipgloss.Color
	Red         lipgloss.Color
	Blue        lipgloss.Color
	Yellow      lipgloss.Color
}

// Active is the currently selected theme.
var Active = FlexokiDark

// FlexokiDark is the default theme.
var FlexokiDark = Theme{
	Name:        "flexoki-dark",
	Background:  lipgloss.Color("#100F0F"),
	Surface:     lipgloss.Color("#1C1B1A"),
	Border:      lipgloss.Color("#282726"),
	TextDim:     lipgloss.Color("#575653"),
	TextMuted:   lipgloss.Color("#6F6E69"),
	TextPrimary: lipgloss.Color("#FFFCF0"),
	Accent:      lipgloss.Color("#3AA99F"),
	Green:       lipgloss.Color("#879A39"),
	Orange:      lipgloss.Color("#DA702C"),
	Red:         lipgloss.Color("#D14D41"),
	Blue:        lipgloss.Color("#4385BE"),
	Yellow:      lipgloss.Color("#D0A215"),
}

// CatppuccinMocha is a warm pastel theme.
var CatppuccinMocha = Theme{
	Name:        "catppuccin-mocha",
	Background:  lipgloss.Color("#1E1E2E"),
	Surface:     lipgloss.Color("#313244"),
	Border:      lipgloss.Color("#45475A"),
	TextDim:     lipgloss.Color("#6C7086"),
	TextMuted:   lipgloss.Color("#A6ADC8"),
	TextPrimary: lipgloss.Color("#CDD6F4"),
	Accent:      lipgloss.Color("#89B4FA"),
	Green:       lipgloss.Color("#A6E3A1"),
	Orange:      lipgloss.Color("#FAB387"),
	Red:         lipgloss.Color("#F38BA8"),
	Blue:        lipgloss.Color("#89B4FA"),
	Yellow:      lipgloss.Color("#F9E2AF"),
}

// TokyoNight is a cool blue/purple theme.
var TokyoNight = Theme{
	Name:        "tokyo-night",
	Background:  lipgloss.Color("#1A1B26"),
	Surface:     lipgloss.Color("#24283B"),
	Border:      lipgloss.Color("#414868"),
	TextDim:     lipgloss.Color("#565F89"),
	TextMuted:   lipgloss.Color("#A9B1D6"),
	TextPrimary: lipgloss.Color("#C0CAF5"),
	Accent:      lipgloss.Color("#7AA2F7"),
	Green:       lipgloss.Color("#9ECE6A"),
	Orange:      lipgloss.Color("#FF9E64"),
	Red:         lipgloss.Color("#F7768E"),
	Blue:        lipgloss.Color("#7AA2F7"),
	Yellow:      lipgloss.Color("#E0AF68"),
}

// Terminal uses ANSI 16 colors only.
var Terminal = Theme{
	Name:        "terminal",
	Background:  lipgloss.Color("0"),
	Surface:     lipgloss.Color("0"),
	Border:      lipgloss.Color("8"),
	TextDim:     lipgloss.Color("8"),
	TextMuted:   lipgloss.Color("7"),
	TextPrimary: lipgloss.Color("15"),
	Accent:      lipgloss.Color("6"),
	Green:       lipgloss.Color("2"),
	Orange:      lipgloss.Color("3"),
	Red:         lipgloss.Color("1"),
	Blue:        lipgloss.Color("4"),
	Yellow:      lipgloss.Color("3"),
}

// All available themes.
var All = []Theme{FlexokiDark, CatppuccinMocha, TokyoNight, Terminal}

// ByName returns a theme by its name, defaulting to FlexokiDark.
func ByName(name string) Theme {
	for _, t := range All {
		if t.Name == name {
			return t
		}
	}
	return FlexokiDark
}

// SetActive sets the active theme by name.
func SetActive(name string) {
	Active = ByName(name)
}
