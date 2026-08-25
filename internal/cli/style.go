package cli

import "github.com/charmbracelet/lipgloss"

var headerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(lipgloss.AdaptiveColor{Light: "#4338CA", Dark: "#4F46E5"}).
	Padding(1, 4).
	MarginTop(1).
	MarginBottom(1).
	Align(lipgloss.Center).
	BorderStyle(lipgloss.ThickBorder()).
	BorderForeground(lipgloss.AdaptiveColor{Light: "#4338CA", Dark: "#4F46E5"})

var bannerBodyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}).
	Align(lipgloss.Left).
	MarginBottom(1)
