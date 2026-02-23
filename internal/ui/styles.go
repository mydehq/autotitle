package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

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
		Dark:  lipgloss.CompleteColor{TrueColor: "#bdbdbd", ANSI256: "250", ANSI: "8"},
		Light: lipgloss.CompleteColor{TrueColor: "#626262", ANSI256: "241", ANSI: "0"},
	}
	colorFlag = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#ff5faf", ANSI256: "204", ANSI: "13"},
		Light: lipgloss.CompleteColor{TrueColor: "#af005f", ANSI256: "125", ANSI: "5"},
	}
	colorError = lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: "#ff5faf", ANSI256: "204", ANSI: "13"},
		Light: lipgloss.CompleteColor{TrueColor: "#af005f", ANSI256: "125", ANSI: "5"},
	}

	// Exported Styles for CLI and TUI
	StyleHeader  = lipgloss.NewStyle().Bold(true).Foreground(colorHeader)
	StyleCommand = lipgloss.NewStyle().Bold(true).Foreground(colorCommand)
	StylePath    = lipgloss.NewStyle().Foreground(colorPath)
	StylePattern = lipgloss.NewStyle().Foreground(colorPattern)
	StyleDim     = lipgloss.NewStyle().Foreground(colorDim)
	StyleFlag    = lipgloss.NewStyle().Italic(true).Foreground(colorFlag)
	StyleError   = lipgloss.NewStyle().Foreground(colorError)

	// StyleBanner is the main wizard title banner
	StyleBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCommand).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorHeader).
			Padding(0, 4).
			Align(lipgloss.Center)
)

// AutotitleTheme returns the Catppuccin theme for huh forms.
func AutotitleTheme() *huh.Theme {
	return huh.ThemeCatppuccin()
}

func AutotitleKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()

	// Map both to Quit; we will distinguish them via a bubbletea filter
	km.Quit.SetKeys("esc", "ctrl+c")
	km.Quit.SetHelp("ctrl+c", "quit")

	// Append navigation help to the primary actions
	km.Select.Submit.SetHelp("enter", "choose • esc: back • ctrl+c: quit")
	km.MultiSelect.Submit.SetHelp("enter", "confirm • esc: back • ctrl+c: quit")
	km.Input.Next.SetHelp("enter", "next • esc: back • ctrl+c: quit")
	km.Input.Submit.SetHelp("enter", "submit • esc: back • ctrl+c: quit")
	km.Confirm.Submit.SetHelp("enter", "confirm • esc: back • ctrl+c: quit")
	km.Note.Next.SetHelp("enter", "next • esc: back • ctrl+c: quit")
	km.Note.Submit.SetHelp("enter", "submit • esc: back • ctrl+c: quit")

	return km
}

// ErrUserBack is returned when the user explicitly requests to go to the previous step.
var ErrUserBack = errors.New("user navigated back")

// interceptedKey tracks the last key that triggered an abort (esc vs ctrl+c).
var interceptedKey string

// wizardFilter is a Bubble Tea filter that intercepts esc and ctrl+c to distinguish them.
func wizardFilter(m tea.Model, msg tea.Msg) tea.Msg {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEsc:
			interceptedKey = "esc"
		case tea.KeyCtrlC:
			interceptedKey = "ctrl+c"
		}
	}
	return msg
}

// RunForm is a helper to run a huh form with our custom filter and key interception.
func RunForm(f *huh.Form) error {
	interceptedKey = ""
	return f.WithProgramOptions(tea.WithFilter(wizardFilter)).Run()
}

// ClearAndPrintBanner clears the terminal and prints the AutoTitle header.
func ClearAndPrintBanner(dryRun bool) {
	fmt.Print("\033[H\033[2J")
	fmt.Println()
	fmt.Println(StyleBanner.Render("AutoTitle"))
	fmt.Println()
	if dryRun {
		fmt.Println(StyleFlag.Render("  [DRY RUN]"))
		fmt.Println()
	}
}

// ColorizeEvent adds CLI styling to known event message patterns.
func ColorizeEvent(msg string) string {
	// Messages with "→" (rename/restore): "Renamed: old.mkv → new.mkv"
	if parts := strings.SplitN(msg, " → ", 2); len(parts) == 2 {
		left := parts[0]
		right := parts[1]

		// Split label from filename: "Renamed: old.mkv" → "Renamed:" + "old.mkv"
		var label, oldName string
		if idx := strings.Index(left, ": "); idx >= 0 {
			label = StyleHeader.Render(left[:idx+1]) + " "
			oldName = left[idx+2:]
		} else {
			oldName = left
		}

		return fmt.Sprintf("%s%s %s %s",
			label,
			StyleDim.Render(oldName),
			StyleDim.Render("→"),
			StyleCommand.Render(right),
		)
	}

	// Messages with ": " label: "Tagged: file.mkv", "Backed up: file.mkv"
	if idx := strings.Index(msg, ": "); idx >= 0 {
		label := msg[:idx+1]
		value := msg[idx+2:]
		return fmt.Sprintf("%s %s", StyleHeader.Render(label), StylePath.Render(value))
	}

	return msg
}

// HighlightYAML applies simple syntax highlighting to a YAML string for TUI display.
func HighlightYAML(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 1. Handle Comments
		if idx := strings.Index(line, "#"); idx >= 0 {
			lines[i] = line[:idx] + StyleDim.Render(line[idx:])
			continue
		}

		// 2. Handle Key-Value pairs
		// Find first colon that isn't inside a string (simplistic)
		if idx := strings.Index(line, ":"); idx >= 0 {
			key := line[:idx]
			val := line[idx:] // Includes the colon

			// Style for keys (cyan/bold)
			keyStyle := lipgloss.NewStyle().Foreground(colorCommand).Bold(true)
			// Style for values (pale green)
			valStyle := lipgloss.NewStyle().Foreground(colorPattern)

			// Determine if it's a list item
			prefix := ""
			if strings.HasPrefix(strings.TrimSpace(key), "- ") {
				pIdx := strings.Index(key, "- ")
				prefix = key[:pIdx+2]
				key = key[pIdx+2:]
			}

			// Assemble highlighted line
			if strings.TrimSpace(val) == ":" {
				// Just a key (likely followed by nested object/list)
				lines[i] = prefix + keyStyle.Render(key) + ":"
			} else {
				// Key: Value
				lines[i] = prefix + keyStyle.Render(key) + ":" + valStyle.Render(val[1:])
			}
		}
	}
	return strings.Join(lines, "\n")
}
