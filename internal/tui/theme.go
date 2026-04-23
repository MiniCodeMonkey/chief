package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all color values used throughout the TUI.
type Theme struct {
	PrimaryColor    lipgloss.Color
	SuccessColor    lipgloss.Color
	WarningColor    lipgloss.Color
	ErrorColor      lipgloss.Color
	MutedColor      lipgloss.Color
	BorderColor     lipgloss.Color
	TextColor       lipgloss.Color
	TextMutedColor  lipgloss.Color
	TextBrightColor lipgloss.Color
	BgColor         lipgloss.Color
	BgSelectedColor lipgloss.Color
	BgHighlightColor lipgloss.Color
}

// CatppuccinMochaTheme is the default theme based on the Catppuccin Mocha palette.
var CatppuccinMochaTheme = Theme{
	PrimaryColor:    lipgloss.Color("#00D7FF"),
	SuccessColor:    lipgloss.Color("#5AF78E"),
	WarningColor:    lipgloss.Color("#F3F99D"),
	ErrorColor:      lipgloss.Color("#FF5C57"),
	MutedColor:      lipgloss.Color("#6C7086"),
	BorderColor:     lipgloss.Color("#45475A"),
	TextColor:       lipgloss.Color("#CDD6F4"),
	TextMutedColor:  lipgloss.Color("#6C7086"),
	TextBrightColor: lipgloss.Color("#FFFFFF"),
	BgColor:         lipgloss.Color("#1E1E2E"),
	BgSelectedColor:  lipgloss.Color("#313244"),
	BgHighlightColor: lipgloss.Color("#45475A"),
}

// GruvboxDarkTheme is a theme based on the Gruvbox Dark palette.
var GruvboxDarkTheme = Theme{
	PrimaryColor:     lipgloss.Color("#83A598"),
	SuccessColor:     lipgloss.Color("#B8BB26"),
	WarningColor:     lipgloss.Color("#FABD2F"),
	ErrorColor:       lipgloss.Color("#FB4934"),
	MutedColor:       lipgloss.Color("#928374"),
	BorderColor:      lipgloss.Color("#504945"),
	TextColor:        lipgloss.Color("#EBDBB2"),
	TextMutedColor:   lipgloss.Color("#928374"),
	TextBrightColor:  lipgloss.Color("#FBF1C7"),
	BgColor:          lipgloss.Color("#282828"),
	BgSelectedColor:  lipgloss.Color("#3C3836"),
	BgHighlightColor: lipgloss.Color("#504945"),
}

// ActiveTheme is the currently active theme, defaulting to CatppuccinMochaTheme.
var ActiveTheme = CatppuccinMochaTheme

// ThemeByName returns the Theme for the given name and whether the name was valid.
// Falls back to CatppuccinMochaTheme for unknown names.
func ThemeByName(name string) (Theme, bool) {
	switch name {
	case "catppuccin-mocha":
		return CatppuccinMochaTheme, true
	case "gruvbox-dark":
		return GruvboxDarkTheme, true
	default:
		return CatppuccinMochaTheme, false
	}
}
