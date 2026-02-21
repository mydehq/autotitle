package cli

import "github.com/charmbracelet/lipgloss"

var (
	// Adaptive Color definitions
	colorHeader = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#00af00", ANSI256: "34", ANSI: "2"},
		Light: lipgloss.CompleteColor{TrueColor: "#008700", ANSI256: "28", ANSI: "2"},
	}
	colorCommand = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#5fffff", ANSI256: "86", ANSI: "6"},
		Light: lipgloss.CompleteColor{TrueColor: "#008787", ANSI256: "30", ANSI: "6"},
	}
	colorPath = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#5f5fff", ANSI256: "63", ANSI: "4"},
		Light: lipgloss.CompleteColor{TrueColor: "#0000af", ANSI256: "19", ANSI: "4"},
	}
	colorPattern = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#d7ff87", ANSI256: "192", ANSI: "11"},
		Light: lipgloss.CompleteColor{TrueColor: "#5f8700", ANSI256: "64", ANSI: "10"},
	}
	colorDim = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#9e9e9e", ANSI256: "247", ANSI: "8"},
		Light: lipgloss.CompleteColor{TrueColor: "#444444", ANSI256: "238", ANSI: "0"},
	}
	colorFlag = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#ff5faf", ANSI256: "204", ANSI: "13"},
		Light: lipgloss.CompleteColor{TrueColor: "#af005f", ANSI256: "125", ANSI: "5"},
	}

	// Exported Styles for CLI and TUI
	StyleHeader  = lipgloss.NewStyle().Bold(true).Foreground(colorHeader)
	StyleCommand = lipgloss.NewStyle().Bold(true).Foreground(colorCommand)
	StylePath    = lipgloss.NewStyle().Foreground(colorPath)
	StylePattern = lipgloss.NewStyle().Foreground(colorPattern)
	StyleDim     = lipgloss.NewStyle().Foreground(colorDim)
	styleFlag    = lipgloss.NewStyle().Italic(true).Foreground(colorFlag)
)
