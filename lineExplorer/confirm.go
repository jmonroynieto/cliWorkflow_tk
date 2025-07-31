package main

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Custom styles

var myTheme = huh.Theme{
	Form:           lipgloss.NewStyle().Foreground(lipgloss.Color("#2f63a2ff")).Width(80),
	Group:          lipgloss.NewStyle().Foreground(lipgloss.Color("#5BC0BE")).Width(80),
	FieldSeparator: lipgloss.NewStyle().Foreground(lipgloss.Color("#6FFFE9")).Bold(true),
	Blurred: huh.FieldStyles{
		Title:            lipgloss.NewStyle().Foreground(lipgloss.Color("#A9A9A9")).Italic(true),
		Option:           lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5c3dff")).Padding(0, 2),
		BlurredButton:    lipgloss.NewStyle().Foreground(lipgloss.Color("#EFEFEF")).Background(lipgloss.Color("#393E46")).Padding(0, 2),
		UnselectedOption: lipgloss.NewStyle().Foreground(lipgloss.Color("#535C68")).Padding(0, 2),
	},
	Focused: huh.FieldStyles{
		Title:          lipgloss.NewStyle().Foreground(lipgloss.Color("#273a74")).Bold(true),
		Option:         lipgloss.NewStyle().Foreground(lipgloss.Color("#273a74")).Bold(true).Background(lipgloss.Color("#EFEFEF")).Padding(0, 2),
		FocusedButton:  lipgloss.NewStyle().Foreground(lipgloss.Color("#D6BA7C")).Background(lipgloss.Color("#393E46")).Bold(true).Padding(0, 2),
		SelectedOption: lipgloss.NewStyle().Foreground(lipgloss.Color("#D6BA7C")).Bold(true).Padding(0, 2),
	},
}

var validateOverwrite huh.Form = *huh.NewForm(
	huh.NewGroup(
		huh.NewConfirm().Title(lipgloss.NewStyle().Width(60).AlignHorizontal(lipgloss.Center).Render("Detected changes to the file. Replace currently saved version with the version you sampled minus the lines you marked for deletion? Recent changes will be lost")).Value(&userOverwrite).
			WithTheme(&myTheme),
	),
)
