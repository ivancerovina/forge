package ui

import "github.com/charmbracelet/lipgloss"

var (
	Purple    = lipgloss.Color("#9D4EDD")
	DimPurple = lipgloss.Color("#7B2CBF")
	White     = lipgloss.Color("#E0E0E0")
	Dim       = lipgloss.Color("#6C6C6C")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Purple)

	HeadingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(DimPurple)

	CmdStyle = lipgloss.NewStyle().
			Foreground(White)

	DescStyle = lipgloss.NewStyle().
			Foreground(Dim)

	ErrStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66BB6A"))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA726"))
)
