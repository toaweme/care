package output

import "github.com/charmbracelet/lipgloss"

// HeaderStyle is the bold, accent-colored style for section headers; OKStyle,
// WarnStyle, ErrorStyle, and DimStyle color check outcomes and secondary text.
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
