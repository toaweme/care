package output

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	OKStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("2"))

	WarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)
