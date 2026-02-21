package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/cli"
)

type state int

const (
	stateInitial state = iota
	stateScanning
	stateInitInput
	stateInitializing
	stateConfirmation
	stateRenaming
	stateFinished
)

var (
	titleStyle = cli.StyleCommand

	subTitleStyle = cli.StyleDim

	infoStyle    = cli.StyleCommand
	successStyle = cli.StyleHeader
	warningStyle = cli.StylePattern
	errorStyle   = cli.StylePath

	actionBarMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "229", Dark: "229"}).
				Background(lipgloss.AdaptiveColor{Light: "57", Dark: "57"}).
				Padding(0, 1)

	actionBarKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "57", Dark: "57"}).
				Background(lipgloss.AdaptiveColor{Light: "229", Dark: "229"}).
				Padding(0, 1).
				Bold(true)
)

type scanDoneMsg struct {
	ops []autotitle.RenameOperation
	err error
}

type eventMsg autotitle.Event

type renameDoneMsg struct {
	ops []autotitle.RenameOperation
	err error
}

type initDoneMsg struct {
	err error
}

type Model struct {
	state    state
	path     string
	err      error
	quitting bool

	// Content
	table    table.Model
	input    textinput.Model
	ops      []autotitle.RenameOperation
	progress string

	// Logs
	events []string

	width     int
	height    int
	eventChan chan autotitle.Event
}

func NewModel(path string) Model {
	absPath, _ := filepath.Abs(path)

	columns := []table.Column{
		{Title: "Source File", Width: 40},
		{Title: "Target Name", Width: 40},
		{Title: "Status", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10), // dynamically updated
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("86"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "https://myanimelist.net/anime/XXXXX"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		state:     stateInitial,
		path:      absPath,
		eventChan: make(chan autotitle.Event),
		table:     t,
		input:     ti,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.state == stateInitial || m.state == stateConfirmation || m.state == stateFinished {
				m.quitting = true
				return m, tea.Quit
			}

		case "esc":
			if m.state == stateInitInput {
				m.state = stateInitial
				return m, nil
			}

		case "enter":
			if m.state == stateInitInput {
				m.state = stateInitializing
				return m, m.initDir(m.input.Value())
			} else if m.state == stateInitial || m.state == stateFinished {
				m.state = stateScanning
				m.err = nil
				m.ops = nil
				m.updateTable()
				cmds = append(cmds, m.scanDir())
			} else if m.state == stateConfirmation && len(m.ops) > 0 {
				m.state = stateRenaming
				m.err = nil
				m.events = nil
				cmds = append(cmds, m.runRename(), m.listenForEvents())
			}

		case "backspace":
			if m.state == stateConfirmation {
				m.state = stateInitial
				return m, nil
			}
		}

	case scanDoneMsg:
		if msg.err != nil {
			errStr := msg.err.Error()
			if strings.Contains(errStr, "no such file or directory") && strings.Contains(errStr, "_autotitle.yml") {
				m.err = nil
				m.state = stateInitInput
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}

			m.err = msg.err
			m.state = stateInitial
			return m, nil
		}
		m.ops = msg.ops
		m.state = stateConfirmation
		m.updateTable()

	case eventMsg:
		m.progress = msg.Message
		var styledMsg string
		switch msg.Type {
		case autotitle.EventSuccess:
			styledMsg = successStyle.Render(msg.Message)
		case autotitle.EventWarning:
			styledMsg = warningStyle.Render(msg.Message)
		case autotitle.EventError:
			styledMsg = errorStyle.Render(msg.Message)
		default:
			styledMsg = infoStyle.Render(msg.Message)
		}

		m.events = append(m.events, fmt.Sprintf("[%s] %s", msg.Type, styledMsg))
		if len(m.events) > 100 {
			m.events = m.events[len(m.events)-100:]
		}

		// If it's a success, a file rename likely just finished. Re-sync the table status.
		// A full re-sync is slightly inefficient but safe for typical dir sizes.
		if m.state == stateRenaming {
			// In a real execution, we would update the specific row.
			// For now, we trust runRename updates ops[] behind the scenes or we just stream logs.
		}

		return m, m.listenForEvents()

	case renameDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateInitial
			return m, nil
		}
		m.ops = msg.ops
		m.state = stateFinished
		m.updateTable()
		return m, nil

	case initDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateInitial
			return m, nil
		}
		// If init succeeded, automatically start scanning
		m.state = stateScanning
		m.err = nil
		m.ops = nil
		m.updateTable()
		cmds = append(cmds, m.scanDir())
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTable()

	case error:
		m.err = msg
		return m, nil
	}

	switch m.state {
	case stateConfirmation, stateFinished, stateRenaming:
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	case stateInitInput:
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTable() {
	var rows []table.Row
	for _, op := range m.ops {
		status := "Pending"
		switch op.Status {
		case autotitle.StatusSuccess:
			status = successStyle.Render("Success")
		case autotitle.StatusFailed:
			status = errorStyle.Render("Failed")
		case autotitle.StatusSkipped:
			status = warningStyle.Render("Skipped")
		}
		rows = append(rows, table.Row{filepath.Base(op.SourcePath), filepath.Base(op.TargetPath), status})
	}
	m.table.SetRows(rows)
}

func (m *Model) resizeTable() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	// Calculate widths dynamically for full-width layout
	totalW := m.width - 4 // Padding compensation
	statusW := 10
	flexW := (totalW - statusW) / 2

	if flexW < 10 {
		flexW = 10
	}

	cols := []table.Column{
		{Title: "Source File", Width: flexW},
		{Title: "Target File", Width: flexW},
		{Title: "Status", Width: statusW},
	}
	m.table.SetColumns(cols)

	// Calculate height
	headerH := 4 // Title + Path + padding
	footerH := 2 // Action bar
	contentH := m.height - headerH - footerH

	if m.state == stateRenaming {
		// Split space between table and logs
		contentH = contentH / 2
	}

	if contentH < 5 {
		contentH = 5
	}

	m.table.SetHeight(contentH - 2) // -2 for table borders
}

func (m Model) scanDir() tea.Cmd {
	return func() tea.Msg {
		ops, err := autotitle.Rename(context.Background(), m.path, autotitle.WithDryRun())
		return scanDoneMsg{ops: ops, err: err}
	}
}

func (m Model) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		return eventMsg(<-m.eventChan)
	}
}

func (m Model) runRename() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		handler := func(e autotitle.Event) {
			m.eventChan <- e
		}

		ops, err := autotitle.Rename(ctx, m.path, autotitle.WithEvents(handler))
		return renameDoneMsg{ops: ops, err: err}
	}
}

func (m Model) initDir(url string) tea.Cmd {
	return func() tea.Msg {
		if url == "" {
			url = "https://myanimelist.net/anime/XXXXX/Series_Name"
		}
		err := autotitle.Init(context.Background(), m.path, autotitle.WithURL(url), autotitle.WithForce())
		return initDoneMsg{err: err}
	}
}

func (m Model) renderActionBar(actions []string) string {
	var rendered []string
	for _, a := range actions {
		parts := strings.SplitN(a, " ", 2)
		if len(parts) == 2 {
			rendered = append(rendered, actionBarKeyStyle.Render(parts[0])+actionBarMsgStyle.Render(parts[1]))
		}
	}
	bar := strings.Join(rendered, lipgloss.NewStyle().Background(lipgloss.Color("57")).Render("  "))

	// Pad the rest of the bar to full width
	padW := m.width - lipgloss.Width(bar)
	if padW < 0 {
		padW = 0
	}
	padding := lipgloss.NewStyle().Background(lipgloss.Color("57")).Render(strings.Repeat(" ", padW))

	return bar + padding
}

func (m Model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	if m.width <= 0 || m.height <= 0 {
		return "Starting..."
	}

	var s strings.Builder

	// 1. Top Bar (Header)
	header := fmt.Sprintf("%s  %s", titleStyle.Render("AUTOTITLE"), subTitleStyle.Render("DIR: "+m.path))
	s.WriteString(lipgloss.NewStyle().Padding(1, 2).Render(header))
	s.WriteString("\n")

	// 2. Main Content
	var contentView string
	var actionBarView string

	switch m.state {
	case stateInitial:
		if m.err != nil {
			contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress Enter to try again.", m.err)))
		} else {
			contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, "Press Enter to Scan Directory")
		}
		actionBarView = m.renderActionBar([]string{"Enter Scan", "q Quit"})

	case stateInitInput:
		prompt := lipgloss.NewStyle().Bold(true).Render("Enter Provider URL (MAL/TMDB):")
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Render(prompt + "\n\n" + m.input.View())
		contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box)
		actionBarView = m.renderActionBar([]string{"Enter Submit", "Esc Cancel"})

	case stateInitializing:
		contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, infoStyle.Render("Initializing directory mappings..."))
		actionBarView = m.renderActionBar([]string{"ctrl+c Abort"})

	case stateScanning:
		contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, infoStyle.Render("Scanning directory and matching episodes..."))
		actionBarView = m.renderActionBar([]string{"ctrl+c Abort"})

	case stateConfirmation:
		if len(m.ops) == 0 {
			contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, "No files found to rename.")
			actionBarView = m.renderActionBar([]string{"Enter Rescan", "q Quit"})
		} else {
			statStr := subTitleStyle.Render(fmt.Sprintf("%d files mapped.", len(m.ops)))
			contentView = lipgloss.NewStyle().Padding(0, 2).Render(statStr + "\n\n" + m.table.View())
			actionBarView = m.renderActionBar([]string{"Enter Execute Rename", "Backspace Rescan", "↑/↓ Scroll", "q Quit"})
		}

	case stateRenaming:
		// Top Half: Table
		statStr := infoStyle.Render("Renaming in progress...")
		tableView := lipgloss.NewStyle().Padding(0, 2).Render(statStr + "\n\n" + m.table.View())

		// Bottom Half: Logs
		logBuilder := strings.Builder{}
		logH := (m.height - 6) / 2
		if logH < 5 {
			logH = 5
		}

		maxLogs := logH - 2
		if maxLogs < 0 {
			maxLogs = 0
		}

		startIdx := 0
		if len(m.events) > maxLogs {
			startIdx = len(m.events) - maxLogs
		}
		logLines := m.events[startIdx:]
		if len(logLines) == 0 {
			logBuilder.WriteString(subTitleStyle.Render("Waiting for events..."))
		} else {
			logBuilder.WriteString(strings.Join(logLines, "\n"))
		}

		logBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Width(m.width - 6).
			Height(maxLogs + 1).
			Render(titleStyle.Render("Event Logs") + "\n" + logBuilder.String())

		logView := lipgloss.NewStyle().Padding(1, 2).Render(logBox)

		contentView = lipgloss.JoinVertical(lipgloss.Left, tableView, logView)
		actionBarView = m.renderActionBar([]string{"ctrl+c Abort Operation"})

	case stateFinished:
		success := 0
		for _, op := range m.ops {
			if op.Status == autotitle.StatusSuccess {
				success++
			}
		}

		summary := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("34")).Render(
			fmt.Sprintf("%s\nSuccessfully processed %d files.", successStyle.Bold(true).Render("COMPLETED"), success),
		)

		contentView = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, summary)
		actionBarView = m.renderActionBar([]string{"Enter Rescan", "q Quit"})
	}

	s.WriteString(contentView)

	// Force the action bar to the absolute bottom via newlines
	currentLines := strings.Count(s.String(), "\n")
	neededNewLines := (m.height - 2) - currentLines
	if neededNewLines > 0 {
		s.WriteString(strings.Repeat("\n", neededNewLines))
	} else {
		s.WriteString("\n")
	}
	s.WriteString(actionBarView)

	return s.String()
}
