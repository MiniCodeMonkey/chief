package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/minicodemonkey/chief/internal/config"
)

// SettingsItemType represents the type of a settings item.
type SettingsItemType int

const (
	SettingsItemBool   SettingsItemType = iota
	SettingsItemString
)

// SettingsItem represents a single editable setting.
type SettingsItem struct {
	Section  string
	Label    string
	Key      string // config key for identification
	Type     SettingsItemType
	BoolVal  bool
	StringVal string
}

// SettingsOverlay manages the settings modal overlay state.
type SettingsOverlay struct {
	width  int
	height int

	items         []SettingsItem
	selectedIndex int

	// Inline text editing
	editing    bool
	editBuffer string

	// GH CLI validation error
	ghError    string
	showGHError bool
}

// NewSettingsOverlay creates a new settings overlay.
func NewSettingsOverlay() *SettingsOverlay {
	return &SettingsOverlay{}
}

// SetSize sets the overlay dimensions.
func (s *SettingsOverlay) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// LoadFromConfig populates settings items from a config.
func (s *SettingsOverlay) LoadFromConfig(cfg *config.Config) {
	s.items = []SettingsItem{
		{Section: "Worktree", Label: "Setup command", Key: "worktree.setup", Type: SettingsItemString, StringVal: cfg.Worktree.Setup},
		{Section: "On Complete", Label: "Push to remote", Key: "onComplete.push", Type: SettingsItemBool, BoolVal: cfg.OnComplete.Push},
		{Section: "On Complete", Label: "Create pull request", Key: "onComplete.createPR", Type: SettingsItemBool, BoolVal: cfg.OnComplete.CreatePR},
	}
	s.selectedIndex = 0
	s.editing = false
	s.editBuffer = ""
	s.ghError = ""
	s.showGHError = false
}

// ApplyToConfig writes the current settings values back to a config.
func (s *SettingsOverlay) ApplyToConfig(cfg *config.Config) {
	for _, item := range s.items {
		switch item.Key {
		case "worktree.setup":
			cfg.Worktree.Setup = item.StringVal
		case "onComplete.push":
			cfg.OnComplete.Push = item.BoolVal
		case "onComplete.createPR":
			cfg.OnComplete.CreatePR = item.BoolVal
		}
	}
}

// MoveUp moves the selection up.
func (s *SettingsOverlay) MoveUp() {
	if s.selectedIndex > 0 {
		s.selectedIndex--
	}
}

// MoveDown moves the selection down.
func (s *SettingsOverlay) MoveDown() {
	if s.selectedIndex < len(s.items)-1 {
		s.selectedIndex++
	}
}

// IsEditing returns true if a string value is being edited.
func (s *SettingsOverlay) IsEditing() bool {
	return s.editing
}

// StartEditing begins inline editing of the selected string value.
func (s *SettingsOverlay) StartEditing() {
	if s.selectedIndex < len(s.items) && s.items[s.selectedIndex].Type == SettingsItemString {
		s.editing = true
		s.editBuffer = s.items[s.selectedIndex].StringVal
	}
}

// ConfirmEdit saves the edit buffer to the selected item.
func (s *SettingsOverlay) ConfirmEdit() {
	if s.editing && s.selectedIndex < len(s.items) {
		s.items[s.selectedIndex].StringVal = s.editBuffer
		s.editing = false
		s.editBuffer = ""
	}
}

// CancelEdit discards the edit buffer.
func (s *SettingsOverlay) CancelEdit() {
	s.editing = false
	s.editBuffer = ""
}

// AddEditChar adds a character to the edit buffer.
func (s *SettingsOverlay) AddEditChar(ch rune) {
	s.editBuffer += string(ch)
}

// DeleteEditChar removes the last character from the edit buffer.
func (s *SettingsOverlay) DeleteEditChar() {
	if len(s.editBuffer) > 0 {
		runes := []rune(s.editBuffer)
		s.editBuffer = string(runes[:len(runes)-1])
	}
}

// ToggleBool toggles the selected boolean value.
// Returns the key and new value for the caller to act on.
func (s *SettingsOverlay) ToggleBool() (key string, newVal bool) {
	if s.selectedIndex < len(s.items) && s.items[s.selectedIndex].Type == SettingsItemBool {
		s.items[s.selectedIndex].BoolVal = !s.items[s.selectedIndex].BoolVal
		return s.items[s.selectedIndex].Key, s.items[s.selectedIndex].BoolVal
	}
	return "", false
}

// RevertToggle reverts the last toggle (used when validation fails).
func (s *SettingsOverlay) RevertToggle() {
	if s.selectedIndex < len(s.items) && s.items[s.selectedIndex].Type == SettingsItemBool {
		s.items[s.selectedIndex].BoolVal = !s.items[s.selectedIndex].BoolVal
	}
}

// SetGHError sets the GH CLI error message.
func (s *SettingsOverlay) SetGHError(msg string) {
	s.ghError = msg
	s.showGHError = true
}

// HasGHError returns true if a GH CLI error is being displayed.
func (s *SettingsOverlay) HasGHError() bool {
	return s.showGHError
}

// DismissGHError clears the GH CLI error.
func (s *SettingsOverlay) DismissGHError() {
	s.showGHError = false
	s.ghError = ""
}

// GetSelectedItem returns the currently selected settings item.
func (s *SettingsOverlay) GetSelectedItem() *SettingsItem {
	if s.selectedIndex >= 0 && s.selectedIndex < len(s.items) {
		return &s.items[s.selectedIndex]
	}
	return nil
}

// Render renders the settings overlay.
func (s *SettingsOverlay) Render() string {
	modalWidth := min(60, s.width-10)
	modalHeight := min(18, s.height-6)

	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalHeight < 12 {
		modalHeight = 12
	}

	var content strings.Builder

	// Header: "Settings" left-aligned, ".chief/config.yaml" right-aligned
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)
	pathStyle := lipgloss.NewStyle().
		Foreground(MutedColor)

	title := titleStyle.Render("Settings")
	path := pathStyle.Render(".chief/config.yaml")
	titleWidth := lipgloss.Width(title)
	pathWidth := lipgloss.Width(path)
	titlePadding := modalWidth - 4 - titleWidth - pathWidth
	if titlePadding < 1 {
		titlePadding = 1
	}
	content.WriteString(" ")
	content.WriteString(title)
	content.WriteString(strings.Repeat(" ", titlePadding))
	content.WriteString(path)
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n\n")

	// GH error dialog overlay
	if s.showGHError {
		content.WriteString(s.renderGHError(modalWidth))
	} else {
		// Render settings items grouped by section
		content.WriteString(s.renderItems(modalWidth))
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(DividerStyle.Render(strings.Repeat("─", modalWidth-4)))
	content.WriteString("\n")

	footerStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)

	if s.showGHError {
		content.WriteString(footerStyle.Render("Press any key to dismiss"))
	} else if s.editing {
		content.WriteString(footerStyle.Render("Enter: save  │  Esc: cancel"))
	} else {
		content.WriteString(footerStyle.Render("Enter: toggle/edit  │  j/k: navigate  │  Esc: close"))
	}

	// Modal box style
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modal := modalStyle.Render(content.String())

	return centerModal(modal, s.width, s.height)
}

// renderItems renders the settings items grouped by section.
func (s *SettingsOverlay) renderItems(modalWidth int) string {
	var result strings.Builder

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Padding(0, 1)
	labelStyle := lipgloss.NewStyle().
		Foreground(TextColor)
	selectedLabelStyle := lipgloss.NewStyle().
		Foreground(TextBrightColor).
		Bold(true)
	valueStyle := lipgloss.NewStyle().
		Foreground(SuccessColor)
	valueOffStyle := lipgloss.NewStyle().
		Foreground(MutedColor)
	cursorStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	currentSection := ""
	for i, item := range s.items {
		// Section header
		if item.Section != currentSection {
			if currentSection != "" {
				result.WriteString("\n")
			}
			result.WriteString(sectionStyle.Render(item.Section))
			result.WriteString("\n")
			currentSection = item.Section
		}

		isSelected := i == s.selectedIndex

		// Cursor
		if isSelected {
			result.WriteString(cursorStyle.Render("  > "))
		} else {
			result.WriteString("    ")
		}

		// Label
		label := item.Label
		if isSelected {
			result.WriteString(selectedLabelStyle.Render(label))
		} else {
			result.WriteString(labelStyle.Render(label))
		}

		// Value (right-aligned)
		var valueStr string
		switch item.Type {
		case SettingsItemBool:
			if item.BoolVal {
				valueStr = valueStyle.Render("Yes")
			} else {
				valueStr = valueOffStyle.Render("No")
			}
		case SettingsItemString:
			if isSelected && s.editing {
				// Show edit buffer with cursor
				editStyle := lipgloss.NewStyle().Foreground(TextBrightColor)
				cursorChar := lipgloss.NewStyle().Foreground(PrimaryColor).Render("█")
				if s.editBuffer == "" {
					valueStr = editStyle.Render("(empty)") + cursorChar
				} else {
					valueStr = editStyle.Render(s.editBuffer) + cursorChar
				}
			} else if item.StringVal == "" {
				valueStr = valueOffStyle.Render("(not set)")
			} else {
				// Truncate long values
				val := item.StringVal
				maxValWidth := modalWidth - 4 - 4 - len(label) - 4
				if maxValWidth < 10 {
					maxValWidth = 10
				}
				if len(val) > maxValWidth {
					val = val[:maxValWidth-1] + "…"
				}
				valueStr = valueStyle.Render(val)
			}
		}

		// Calculate padding between label and value
		labelWidth := lipgloss.Width(label) + 4 // cursor prefix
		valWidth := lipgloss.Width(valueStr)
		padding := modalWidth - 4 - labelWidth - valWidth - 2
		if padding < 2 {
			padding = 2
		}
		result.WriteString(strings.Repeat(" ", padding))
		result.WriteString(valueStr)
		result.WriteString("\n")
	}

	return result.String()
}

// renderGHError renders the GH CLI error dialog.
func (s *SettingsOverlay) renderGHError(modalWidth int) string {
	var result strings.Builder

	errorHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ErrorColor).
		Padding(0, 1)
	errorMsgStyle := lipgloss.NewStyle().
		Foreground(TextColor).
		Padding(0, 1)

	result.WriteString(errorHeaderStyle.Render("GitHub CLI Error"))
	result.WriteString("\n\n")
	result.WriteString(errorMsgStyle.Render(s.ghError))
	result.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)
	result.WriteString(hintStyle.Render(fmt.Sprintf("Install: https://cli.github.com")))
	result.WriteString("\n")
	result.WriteString(hintStyle.Render("PR creation has been disabled."))

	_ = modalWidth
	return result.String()
}
