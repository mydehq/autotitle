package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mydehq/autotitle/internal/tui"
)

func main() {
	path := "."
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	p := tea.NewProgram(tui.NewModel(path), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
