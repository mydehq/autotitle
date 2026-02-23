package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// Logger wraps the standard charmbracelet logger to add custom levels
type Logger struct {
	*log.Logger
}

var logger *Logger

// SetLogger injects the application logger into the UI package.
func SetLogger(l *log.Logger) {
	logger = &Logger{Logger: l}
}

// Success prints a success message with a green prefix
func (l *Logger) Success(msg interface{}, keyvals ...interface{}) {
	l.Helper()
	// Create a success label: bold green
	label := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		SetString("SUCCESS").
		String()

	// Use Print instead of Info to avoid the default "INFO" prefix
	l.Print(fmt.Sprintf("%s %v", label, msg), keyvals...)
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
