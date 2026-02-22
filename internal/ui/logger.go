package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var logger *log.Logger

// SetLogger injects the application logger into the UI package.
func SetLogger(l *log.Logger) {
	logger = l
}

// ConfigureLoggerStyles applies the lipgloss styling to the injected logger.
func ConfigureLoggerStyles() {
	if logger == nil {
		return
	}
	styles := log.DefaultStyles()

	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
		SetString("DEBUG").
		Bold(true).
		Foreground(lipgloss.Color("63"))

	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		SetString("INFO ").
		Bold(true).
		Foreground(lipgloss.Color("86"))

	styles.Levels[log.WarnLevel] = lipgloss.NewStyle().
		SetString("WARN ").
		Bold(true).
		Foreground(lipgloss.Color("192"))

	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERROR").
		Bold(true).
		Foreground(lipgloss.Color("204"))

	logger.SetStyles(styles)
}
