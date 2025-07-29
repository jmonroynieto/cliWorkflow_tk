package main

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Custom styles

var myTheme = huh.Theme{
	Form:           lipgloss.NewStyle().Foreground(lipgloss.Color("#3A506B")),
	Group:          lipgloss.NewStyle().Foreground(lipgloss.Color("#5BC0BE")),
	FieldSeparator: lipgloss.NewStyle().Foreground(lipgloss.Color("#6FFFE9")).Bold(true),
	Blurred: huh.FieldStyles{
		Title:            lipgloss.NewStyle().Foreground(lipgloss.Color("#A9A9A9")).Italic(true),
		Option:           lipgloss.NewStyle().Foreground(lipgloss.Color("#CED4DA")),
		BlurredButton:    lipgloss.NewStyle().Foreground(lipgloss.Color("#EFEFEF")).Background(lipgloss.Color("#393E46")),
		UnselectedOption: lipgloss.NewStyle().Foreground(lipgloss.Color("#535C68")),
	},
	Focused: huh.FieldStyles{
		Title:          lipgloss.NewStyle().Foreground(lipgloss.Color("#0B132B")).Bold(true),
		Option:         lipgloss.NewStyle().Foreground(lipgloss.Color("#5BC0BE")).Bold(true),
		FocusedButton:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6FFFE9")).Background(lipgloss.Color("#393E46")).Bold(true),
		SelectedOption: lipgloss.NewStyle().Foreground(lipgloss.Color("#D6BA7C")).Bold(true),
	},
}

var validateOverwrite huh.Form = *huh.NewForm(
	huh.NewGroup(
		huh.NewConfirm().Title("changes to file detected overwrite changes with deletion over the file as it was originally?").Value(&userOverwrite).
			WithTheme(&myTheme),
	),
)
