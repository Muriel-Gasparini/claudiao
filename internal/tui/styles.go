package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent   = lipgloss.Color("#c77dff")
	colorMuted    = lipgloss.Color("#9b9b9b")
	colorOK       = lipgloss.Color("#7bed9f")
	colorWarn     = lipgloss.Color("#ffb86c")
	colorErr      = lipgloss.Color("#ff6b6b")
	colorSelected = lipgloss.Color("#ffffff")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)

	okStyle   = lipgloss.NewStyle().Foreground(colorOK)
	warnStyle = lipgloss.NewStyle().Foreground(colorWarn)
	errStyle  = lipgloss.NewStyle().Foreground(colorErr)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(colorSelected).
				Bold(true)

	itemStyle = lipgloss.NewStyle().Foreground(colorMuted)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)
)
