package main

import "github.com/charmbracelet/lipgloss"

// Palette. Foregrounds that render directly against the terminal background
// use ANSI palette colors (or the default foreground) instead of fixed hex
// values, so text stays readable on both light and dark themes. titleStyle
// keeps a hex pair because it sets foreground and background together.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111B")).
			Background(lipgloss.Color("#89B4FA")).
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	countStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	nameStyle    = lipgloss.NewStyle()
	nameSelStyle = lipgloss.NewStyle().Bold(true)
	descStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	matchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	barStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	labelSel     = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)
