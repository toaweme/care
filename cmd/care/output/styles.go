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

	// concrete mid-gray (256-color 245) rather than ANSI palette 8: index 8 is
	// remapped per-terminal and iTerm2 renders it near-black, which disappears
	// under window transparency. A fixed gray stays muted but readable anywhere.
	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
)
