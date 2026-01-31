// Package tui provides Bubble Tea TUI components for the quarry CLI.
//
// TUI rules per CONTRACT_CLI.md:
//   - TUI is opt-in only (--tui flag)
//   - TUI is read-only only (inspect, stats commands)
//   - TUI uses same data payloads as non-TUI rendering
//   - No TUI-exclusive data allowed
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	successColor   = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	highlightColor = lipgloss.Color("#3B82F6") // Blue
)

// Styles for TUI components.
var (
	// TitleStyle for headers and titles.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// LabelStyle for field labels.
	LabelStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(16)

	// ValueStyle for field values.
	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// SuccessStyle for success states.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	// WarningStyle for warning states.
	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// ErrorStyle for error states.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	// BoxStyle for bordered containers.
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(1, 2)

	// HelpStyle for help text.
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// StatBoxStyle for stat display boxes.
	StatBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlightColor).
			Padding(0, 2).
			Width(20).
			Align(lipgloss.Center)

	// StatLabelStyle for stat labels.
	StatLabelStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Align(lipgloss.Center)

	// StatValueStyle for stat values.
	StatValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Align(lipgloss.Center)
)

// StateStyle returns a style based on the state string.
func StateStyle(state string) lipgloss.Style {
	switch state {
	case "succeeded", "completed", "idle":
		return SuccessStyle
	case "running", "in_progress":
		return WarningStyle
	case "failed", "error":
		return ErrorStyle
	default:
		return ValueStyle
	}
}
