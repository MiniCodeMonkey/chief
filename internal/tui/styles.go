package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - consistent colors used throughout the TUI
var (
	// Primary colors
	PrimaryColor = lipgloss.Color("#00D7FF") // Cyan - primary brand, in-progress states
	SuccessColor = lipgloss.Color("#5AF78E") // Green - passed, complete states
	WarningColor = lipgloss.Color("#F3F99D") // Yellow - paused, warning states
	ErrorColor   = lipgloss.Color("#FF5C57") // Red - failed, error states
	MutedColor   = lipgloss.Color("#6C7086") // Gray - pending, muted text
	BorderColor  = lipgloss.Color("#45475A") // Dark gray - borders, dividers

	// Text colors
	TextColor       = lipgloss.Color("#CDD6F4") // Light gray - primary text
	TextMutedColor  = lipgloss.Color("#6C7086") // Muted text
	TextBrightColor = lipgloss.Color("#FFFFFF") // Bright white - emphasis

	// Background colors
	BgColor         = lipgloss.Color("#1E1E2E") // Dark background
	BgSelectedColor = lipgloss.Color("#313244") // Selected item background
	BgHighlightColor = lipgloss.Color("#45475A") // Highlight background
)

// Aliases for backward compatibility with existing code
var (
	primaryColor = PrimaryColor
	successColor = SuccessColor
	warningColor = WarningColor
	errorColor   = ErrorColor
	mutedColor   = MutedColor
	borderColor  = BorderColor
)

// Header styles
var (
	// Main header style with branding
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			Padding(0, 1)

	// Header border/divider
	HeaderBorderStyle = lipgloss.NewStyle().
				Foreground(BorderColor)
)

// Footer styles
var (
	footerStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(0, 1)

	// Shortcut key style
	ShortcutKeyStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	// Shortcut description style
	ShortcutDescStyle = lipgloss.NewStyle().
				Foreground(MutedColor)
)

// Panel styles
var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	// Panel with focus/active state
	PanelActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				Padding(0, 1)

	// Panel title style
	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor)
)

// Selection styles
var (
	selectedStyle = lipgloss.NewStyle().
			Background(BgSelectedColor).
			Foreground(TextColor)

	// Unselected/normal item style
	UnselectedStyle = lipgloss.NewStyle().
			Foreground(TextColor)
)

// Status badge styles - colored badges for state indicators
var (
	// Story status styles
	statusPassedStyle     = lipgloss.NewStyle().Foreground(SuccessColor)
	statusInProgressStyle = lipgloss.NewStyle().Foreground(PrimaryColor)
	statusPendingStyle    = lipgloss.NewStyle().Foreground(MutedColor)
	statusFailedStyle     = lipgloss.NewStyle().Foreground(ErrorColor)
	statusPausedStyle     = lipgloss.NewStyle().Foreground(WarningColor)

	// State badge styles (with bold for headers)
	StateReadyStyle    = lipgloss.NewStyle().Bold(true).Foreground(MutedColor)
	StateRunningStyle  = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor)
	StatePausedStyle   = lipgloss.NewStyle().Bold(true).Foreground(WarningColor)
	StateStoppedStyle  = lipgloss.NewStyle().Bold(true).Foreground(MutedColor)
	StateCompleteStyle = lipgloss.NewStyle().Bold(true).Foreground(SuccessColor)
	StateErrorStyle    = lipgloss.NewStyle().Bold(true).Foreground(ErrorColor)
)

// Title and label styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextColor)

	labelStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	// Subtitle style
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	// Description text style
	DescriptionStyle = lipgloss.NewStyle().
				Foreground(TextColor)
)

// Progress bar styles
var (
	progressBarFillStyle  = lipgloss.NewStyle().Foreground(SuccessColor)
	progressBarEmptyStyle = lipgloss.NewStyle().Foreground(MutedColor)

	// Progress percentage style
	ProgressPercentStyle = lipgloss.NewStyle().
				Foreground(MutedColor)
)

// Activity line styles
var (
	ActivityRunningStyle  = lipgloss.NewStyle().Foreground(PrimaryColor).Padding(0, 1)
	ActivityErrorStyle    = lipgloss.NewStyle().Foreground(ErrorColor).Padding(0, 1)
	ActivityCompleteStyle = lipgloss.NewStyle().Foreground(SuccessColor).Padding(0, 1)
	ActivityMutedStyle    = lipgloss.NewStyle().Foreground(MutedColor).Padding(0, 1)
)

// Divider styles
var (
	DividerStyle = lipgloss.NewStyle().
			Foreground(BorderColor)

	// Thick divider (for section separators)
	ThickDividerStyle = lipgloss.NewStyle().
				Foreground(BorderColor).
				Bold(true)
)

// Status icons
const (
	IconPassed     = "✓"
	IconInProgress = "●"
	IconPending    = "○"
	IconFailed     = "✗"
	IconPaused     = "◐"
)

// Backward compatibility aliases
const (
	iconPassed     = IconPassed
	iconInProgress = IconInProgress
	iconPending    = IconPending
	iconFailed     = IconFailed
)

// GetStatusIcon returns the appropriate icon for a story's status.
func GetStatusIcon(passed, inProgress bool) string {
	if passed {
		return statusPassedStyle.Render(IconPassed)
	}
	if inProgress {
		return statusInProgressStyle.Render(IconInProgress)
	}
	return statusPendingStyle.Render(IconPending)
}

// GetStateStyle returns the appropriate style for an app state.
func GetStateStyle(state AppState) lipgloss.Style {
	switch state {
	case StateRunning:
		return StateRunningStyle
	case StatePaused:
		return StatePausedStyle
	case StateComplete:
		return StateCompleteStyle
	case StateError:
		return StateErrorStyle
	case StateStopped:
		return StateStoppedStyle
	default:
		return StateReadyStyle
	}
}

// GetActivityStyle returns the appropriate style for activity line based on state.
func GetActivityStyle(state AppState) lipgloss.Style {
	switch state {
	case StateRunning:
		return ActivityRunningStyle
	case StateError:
		return ActivityErrorStyle
	case StateComplete:
		return ActivityCompleteStyle
	default:
		return ActivityMutedStyle
	}
}
