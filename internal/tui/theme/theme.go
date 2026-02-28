// Package theme defines color themes for the cburn TUI dashboard.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color roles used throughout the TUI.
type Theme struct {
	Name          string
	Background    lipgloss.Color // Main app background
	Surface       lipgloss.Color // Card/panel backgrounds
	SurfaceHover  lipgloss.Color // Highlighted surface (active tab, selected row)
	SurfaceBright lipgloss.Color // Extra bright surface for emphasis
	Border        lipgloss.Color // Subtle borders
	BorderBright  lipgloss.Color // Prominent borders (cards, focus)
	BorderAccent  lipgloss.Color // Accent-colored borders for focus states
	TextDim       lipgloss.Color // Lowest contrast text (hints, disabled)
	TextMuted     lipgloss.Color // Secondary text (labels, metadata)
	TextPrimary   lipgloss.Color // Primary content text
	Accent        lipgloss.Color // Primary accent (links, active states)
	AccentBright  lipgloss.Color // Brighter accent for emphasis
	AccentDim     lipgloss.Color // Dimmed accent for backgrounds
	Green         lipgloss.Color
	GreenBright   lipgloss.Color
	Orange        lipgloss.Color
	Red           lipgloss.Color
	Blue          lipgloss.Color
	BlueBright    lipgloss.Color
	Yellow        lipgloss.Color
	Magenta       lipgloss.Color
	Cyan          lipgloss.Color
}

// Active is the currently selected theme.
var Active = FlexokiDark

// FlexokiDark is the default theme - warm, paper-inspired dark theme.
var FlexokiDark = Theme{
	Name:          "flexoki-dark",
	Background:    lipgloss.Color("#100F0F"),
	Surface:       lipgloss.Color("#1C1B1A"),
	SurfaceHover:  lipgloss.Color("#282726"),
	SurfaceBright: lipgloss.Color("#343331"),
	Border:        lipgloss.Color("#403E3C"),
	BorderBright:  lipgloss.Color("#575653"),
	BorderAccent:  lipgloss.Color("#3AA99F"),
	TextDim:       lipgloss.Color("#575653"),
	TextMuted:     lipgloss.Color("#878580"),
	TextPrimary:   lipgloss.Color("#FFFCF0"),
	Accent:        lipgloss.Color("#3AA99F"),
	AccentBright:  lipgloss.Color("#5BC8BE"),
	AccentDim:     lipgloss.Color("#1A3533"),
	Green:         lipgloss.Color("#879A39"),
	GreenBright:   lipgloss.Color("#A3B859"),
	Orange:        lipgloss.Color("#DA702C"),
	Red:           lipgloss.Color("#D14D41"),
	Blue:          lipgloss.Color("#4385BE"),
	BlueBright:    lipgloss.Color("#6BA3D6"),
	Yellow:        lipgloss.Color("#D0A215"),
	Magenta:       lipgloss.Color("#CE5D97"),
	Cyan:          lipgloss.Color("#24837B"),
}

// CatppuccinMocha is a warm pastel theme with soft, soothing colors.
var CatppuccinMocha = Theme{
	Name:          "catppuccin-mocha",
	Background:    lipgloss.Color("#1E1E2E"),
	Surface:       lipgloss.Color("#313244"),
	SurfaceHover:  lipgloss.Color("#45475A"),
	SurfaceBright: lipgloss.Color("#585B70"),
	Border:        lipgloss.Color("#585B70"),
	BorderBright:  lipgloss.Color("#7F849C"),
	BorderAccent:  lipgloss.Color("#89B4FA"),
	TextDim:       lipgloss.Color("#6C7086"),
	TextMuted:     lipgloss.Color("#A6ADC8"),
	TextPrimary:   lipgloss.Color("#CDD6F4"),
	Accent:        lipgloss.Color("#89B4FA"),
	AccentBright:  lipgloss.Color("#B4D0FB"),
	AccentDim:     lipgloss.Color("#293147"),
	Green:         lipgloss.Color("#A6E3A1"),
	GreenBright:   lipgloss.Color("#C6F6C1"),
	Orange:        lipgloss.Color("#FAB387"),
	Red:           lipgloss.Color("#F38BA8"),
	Blue:          lipgloss.Color("#89B4FA"),
	BlueBright:    lipgloss.Color("#B4D0FB"),
	Yellow:        lipgloss.Color("#F9E2AF"),
	Magenta:       lipgloss.Color("#F5C2E7"),
	Cyan:          lipgloss.Color("#94E2D5"),
}

// TokyoNight is a cool blue/purple theme inspired by Tokyo city lights.
var TokyoNight = Theme{
	Name:          "tokyo-night",
	Background:    lipgloss.Color("#1A1B26"),
	Surface:       lipgloss.Color("#24283B"),
	SurfaceHover:  lipgloss.Color("#343A52"),
	SurfaceBright: lipgloss.Color("#414868"),
	Border:        lipgloss.Color("#565F89"),
	BorderBright:  lipgloss.Color("#7982A9"),
	BorderAccent:  lipgloss.Color("#7AA2F7"),
	TextDim:       lipgloss.Color("#565F89"),
	TextMuted:     lipgloss.Color("#A9B1D6"),
	TextPrimary:   lipgloss.Color("#C0CAF5"),
	Accent:        lipgloss.Color("#7AA2F7"),
	AccentBright:  lipgloss.Color("#A9C1FF"),
	AccentDim:     lipgloss.Color("#252B3F"),
	Green:         lipgloss.Color("#9ECE6A"),
	GreenBright:   lipgloss.Color("#B9E87A"),
	Orange:        lipgloss.Color("#FF9E64"),
	Red:           lipgloss.Color("#F7768E"),
	Blue:          lipgloss.Color("#7AA2F7"),
	BlueBright:    lipgloss.Color("#A9C1FF"),
	Yellow:        lipgloss.Color("#E0AF68"),
	Magenta:       lipgloss.Color("#BB9AF7"),
	Cyan:          lipgloss.Color("#7DCFFF"),
}

// Terminal uses ANSI 16 colors only - maximum compatibility.
var Terminal = Theme{
	Name:          "terminal",
	Background:    lipgloss.Color("0"),
	Surface:       lipgloss.Color("0"),
	SurfaceHover:  lipgloss.Color("8"),
	SurfaceBright: lipgloss.Color("8"),
	Border:        lipgloss.Color("8"),
	BorderBright:  lipgloss.Color("7"),
	BorderAccent:  lipgloss.Color("6"),
	TextDim:       lipgloss.Color("8"),
	TextMuted:     lipgloss.Color("7"),
	TextPrimary:   lipgloss.Color("15"),
	Accent:        lipgloss.Color("6"),
	AccentBright:  lipgloss.Color("14"),
	AccentDim:     lipgloss.Color("0"),
	Green:         lipgloss.Color("2"),
	GreenBright:   lipgloss.Color("10"),
	Orange:        lipgloss.Color("3"),
	Red:           lipgloss.Color("1"),
	Blue:          lipgloss.Color("4"),
	BlueBright:    lipgloss.Color("12"),
	Yellow:        lipgloss.Color("3"),
	Magenta:       lipgloss.Color("5"),
	Cyan:          lipgloss.Color("6"),
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
