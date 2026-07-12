// Package style holds the shared lipgloss theme so every screen renders
// with one consistent look, in both light and dark terminals.
package style

import "github.com/charmbracelet/lipgloss"

var accent = lipgloss.AdaptiveColor{Light: "23", Dark: "86"}

var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		MarginBottom(1)

	Selected = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	Faint = lipgloss.NewStyle().Faint(true)

	ErrorText = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"})
)
