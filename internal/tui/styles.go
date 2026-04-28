// Package tui provides the terminal user interface for Chief.
// It includes the main Bubble Tea application, dashboard views,
// log viewer, PRD picker, help overlay, and consistent styling.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - consistent colors used throughout the TUI
var (
	// Primary colors
	PrimaryColor lipgloss.Color
	SuccessColor lipgloss.Color
	WarningColor lipgloss.Color
	ErrorColor   lipgloss.Color
	MutedColor   lipgloss.Color
	BorderColor  lipgloss.Color

	// Text colors
	TextColor       lipgloss.Color
	TextMutedColor  lipgloss.Color
	TextBrightColor lipgloss.Color

	// Background colors
	BgColor          lipgloss.Color
	BgSelectedColor  lipgloss.Color
	BgHighlightColor lipgloss.Color
)

// Aliases for backward compatibility with existing code
var (
	primaryColor lipgloss.Color
	successColor lipgloss.Color
	warningColor lipgloss.Color
	errorColor   lipgloss.Color
	mutedColor   lipgloss.Color
	borderColor  lipgloss.Color
)

// Header styles
var (
	headerStyle       lipgloss.Style
	HeaderBorderStyle lipgloss.Style
)

// Footer styles
var (
	footerStyle       lipgloss.Style
	ShortcutKeyStyle  lipgloss.Style
	ShortcutDescStyle lipgloss.Style
)

// Panel styles
var (
	panelStyle      lipgloss.Style
	PanelActiveStyle lipgloss.Style
	PanelTitleStyle  lipgloss.Style
)

// Selection styles
var (
	selectedStyle   lipgloss.Style
	UnselectedStyle lipgloss.Style
)

// Status badge styles - colored badges for state indicators
var (
	statusPassedStyle     lipgloss.Style
	statusInProgressStyle lipgloss.Style
	statusPendingStyle    lipgloss.Style
	statusFailedStyle     lipgloss.Style
	statusPausedStyle     lipgloss.Style

	StateReadyStyle    lipgloss.Style
	StateRunningStyle  lipgloss.Style
	StatePausedStyle   lipgloss.Style
	StateStoppedStyle  lipgloss.Style
	StateCompleteStyle lipgloss.Style
	StateErrorStyle    lipgloss.Style
)

// Title and label styles
var (
	titleStyle       lipgloss.Style
	labelStyle       lipgloss.Style
	SubtitleStyle    lipgloss.Style
	DescriptionStyle lipgloss.Style
)

// Progress bar styles
var (
	progressBarFillStyle  lipgloss.Style
	progressBarEmptyStyle lipgloss.Style
	ProgressPercentStyle  lipgloss.Style
)

// Activity line styles
var (
	ActivityRunningStyle  lipgloss.Style
	ActivityErrorStyle    lipgloss.Style
	ActivityCompleteStyle lipgloss.Style
	ActivityMutedStyle    lipgloss.Style
)

// Divider styles
var (
	DividerStyle      lipgloss.Style
	ThickDividerStyle lipgloss.Style
)

// Tab bar styles
var (
	TabStyle        lipgloss.Style
	TabActiveStyle  lipgloss.Style
	TabRunningStyle lipgloss.Style
	TabErrorStyle   lipgloss.Style
	TabNewStyle     lipgloss.Style
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

// InitStyles rebuilds all lipgloss styles from ActiveTheme.
// Call this once after setting ActiveTheme, before rendering any TUI component.
func InitStyles() {
	// Update color vars
	PrimaryColor = ActiveTheme.PrimaryColor
	SuccessColor = ActiveTheme.SuccessColor
	WarningColor = ActiveTheme.WarningColor
	ErrorColor = ActiveTheme.ErrorColor
	MutedColor = ActiveTheme.MutedColor
	BorderColor = ActiveTheme.BorderColor
	TextColor = ActiveTheme.TextColor
	TextMutedColor = ActiveTheme.TextMutedColor
	TextBrightColor = ActiveTheme.TextBrightColor
	BgColor = ActiveTheme.BgColor
	BgSelectedColor = ActiveTheme.BgSelectedColor
	BgHighlightColor = ActiveTheme.BgHighlightColor

	primaryColor = PrimaryColor
	successColor = SuccessColor
	warningColor = WarningColor
	errorColor = ErrorColor
	mutedColor = MutedColor
	borderColor = BorderColor

	// Header styles
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ActiveTheme.PrimaryColor).
		Padding(0, 1)

	HeaderBorderStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.BorderColor)

	// Footer styles
	footerStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.MutedColor).
		Padding(0, 1)

	ShortcutKeyStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.PrimaryColor).
		Bold(true)

	ShortcutDescStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.MutedColor)

	// Panel styles
	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.BorderColor).
		Padding(0, 1)

	PanelActiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.PrimaryColor).
		Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ActiveTheme.PrimaryColor)

	// Selection styles
	selectedStyle = lipgloss.NewStyle().
		Background(ActiveTheme.BgSelectedColor).
		Foreground(ActiveTheme.TextColor)

	UnselectedStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.TextColor)

	// Status badge styles
	statusPassedStyle = lipgloss.NewStyle().Foreground(ActiveTheme.SuccessColor)
	statusInProgressStyle = lipgloss.NewStyle().Foreground(ActiveTheme.PrimaryColor)
	statusPendingStyle = lipgloss.NewStyle().Foreground(ActiveTheme.MutedColor)
	statusFailedStyle = lipgloss.NewStyle().Foreground(ActiveTheme.ErrorColor)
	statusPausedStyle = lipgloss.NewStyle().Foreground(ActiveTheme.WarningColor)

	StateReadyStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme.MutedColor)
	StateRunningStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme.PrimaryColor)
	StatePausedStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme.WarningColor)
	StateStoppedStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme.MutedColor)
	StateCompleteStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme.SuccessColor)
	StateErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme.ErrorColor)

	// Title and label styles
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ActiveTheme.TextColor)

	labelStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.PrimaryColor).
		Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.MutedColor)

	DescriptionStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.TextColor)

	// Progress bar styles
	progressBarFillStyle = lipgloss.NewStyle().Foreground(ActiveTheme.SuccessColor)
	progressBarEmptyStyle = lipgloss.NewStyle().Foreground(ActiveTheme.MutedColor)

	ProgressPercentStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.MutedColor)

	// Activity line styles
	ActivityRunningStyle = lipgloss.NewStyle().Foreground(ActiveTheme.PrimaryColor).Padding(0, 1)
	ActivityErrorStyle = lipgloss.NewStyle().Foreground(ActiveTheme.ErrorColor).Padding(0, 1)
	ActivityCompleteStyle = lipgloss.NewStyle().Foreground(ActiveTheme.SuccessColor).Padding(0, 1)
	ActivityMutedStyle = lipgloss.NewStyle().Foreground(ActiveTheme.MutedColor).Padding(0, 1)

	// Divider styles
	DividerStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.BorderColor)

	ThickDividerStyle = lipgloss.NewStyle().
		Foreground(ActiveTheme.BorderColor).
		Bold(true)

	// Tab bar styles
	TabStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.BorderColor).
		Padding(0, 1)

	TabActiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.PrimaryColor).
		Background(ActiveTheme.BgSelectedColor).
		Bold(true).
		Padding(0, 1)

	TabRunningStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.PrimaryColor).
		Padding(0, 1)

	TabErrorStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.ErrorColor).
		Padding(0, 1)

	TabNewStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.MutedColor).
		Foreground(ActiveTheme.MutedColor).
		Padding(0, 1)

	// Confetti uses theme palette colors
	confettiColors = []lipgloss.Color{
		SuccessColor,
		PrimaryColor,
		WarningColor,
		ErrorColor,
		TextBrightColor,
		TextColor,
	}
}

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
