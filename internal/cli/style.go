package cli

import "github.com/charmbracelet/lipgloss"

var headerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#7D56F4")).
	Padding(1, 5).
	MarginBottom(1).
	Align(lipgloss.Center).
	Border(lipgloss.RoundedBorder())
