package cli

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mydehq/autotitle"
	"github.com/mydehq/autotitle/internal/types"
)

// streamResultMsg delivers a single search result to the Bubble Tea model.
type streamResultMsg struct {
	result types.SearchResult
}

// streamDoneMsg signals that all providers have finished.
type streamDoneMsg struct{}

// searchPicker is a Bubble Tea model that displays search results
// as they stream in from a channel.
type searchPicker struct {
	ch       <-chan types.SearchResult
	results  []types.SearchResult
	cursor   int
	selected string
	done     bool // all providers finished
	aborted  bool
	chosen   bool
	filter   string

	// Visible window for scrolling
	windowSize int
}

func newSearchPicker(ch <-chan types.SearchResult) searchPicker {
	return searchPicker{
		ch:         ch,
		windowSize: 12,
	}
}

// waitForResult returns a Cmd that reads the next result from the channel.
func waitForResult(ch <-chan types.SearchResult) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return streamResultMsg{result: r}
	}
}

func (m searchPicker) Init() tea.Cmd {
	return waitForResult(m.ch)
}

func (m searchPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case streamResultMsg:
		m.results = append(m.results, msg.result)
		return m, waitForResult(m.ch)

	case streamDoneMsg:
		m.done = true
		return m, nil

	case tea.KeyMsg:
		filtered := m.filteredResults()

		switch msg.Type {
		case tea.KeyCtrlC:
			m.aborted = true
			interceptedKey = "ctrl+c"
			return m, tea.Quit

		case tea.KeyEsc:
			m.aborted = true
			interceptedKey = "esc"
			return m, tea.Quit

		case tea.KeyEnter:
			if len(filtered) > 0 && m.cursor < len(filtered) {
				m.chosen = true
				m.selected = filtered[m.cursor].URL
				return m, tea.Quit
			}

		case tea.KeyUp, tea.KeyShiftTab:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyDown, tea.KeyTab:
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}

		case tea.KeyRunes:
			m.filter += string(msg.Runes)
			m.cursor = 0

		case tea.KeyBackspace:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.cursor = 0
			}
		}
	}

	return m, nil
}

func (m searchPicker) View() string {
	var b strings.Builder

	// Title
	title := StyleHeader.Render("Select your series")

	// Status indicator
	var status string
	filtered := m.filteredResults()
	if m.done {
		status = StyleDim.Render(fmt.Sprintf("  %d results", len(m.results)))
	} else {
		status = StyleCommand.Render(fmt.Sprintf("  ⟳ searching… %d so far", len(m.results)))
	}
	b.WriteString(title + status + "\n")

	// Filter bar
	if m.filter != "" {
		b.WriteString(StyleDim.Render("  filter: ") + StyleCommand.Render(m.filter) + "\n")
	}
	b.WriteString("\n")

	if len(filtered) == 0 {
		if m.done {
			b.WriteString(StyleDim.Render("  No results found.\n"))
		} else {
			b.WriteString(StyleDim.Render("  Waiting for results…\n"))
		}
	} else {
		// Calculate visible window
		start := 0
		end := len(filtered)
		if end > m.windowSize {
			half := m.windowSize / 2
			start = m.cursor - half
			if start < 0 {
				start = 0
			}
			end = start + m.windowSize
			if end > len(filtered) {
				end = len(filtered)
				start = end - m.windowSize
			}
		}

		if start > 0 {
			b.WriteString(StyleDim.Render(fmt.Sprintf("  ↑ %d more\n", start)))
		}

		selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(colorCommand)
		providerStyle := StyleDim

		for i := start; i < end; i++ {
			r := filtered[i]

			label := r.Title
			if r.Year > 0 {
				label += fmt.Sprintf(" (%d)", r.Year)
			}
			provTag := providerStyle.Render(" [" + strings.ToUpper(r.Provider) + "]")

			if i == m.cursor {
				b.WriteString("  " + selectedStyle.Render("> "+label) + provTag + "\n")
			} else {
				b.WriteString("    " + label + provTag + "\n")
			}
		}

		if end < len(filtered) {
			b.WriteString(StyleDim.Render(fmt.Sprintf("  ↓ %d more\n", len(filtered)-end)))
		}
	}

	b.WriteString("\n")
	helpText := StyleDim.Render("  ↑/↓ navigate • enter select • esc back • ctrl+c quit")
	if m.filter == "" {
		helpText = StyleDim.Render("  ↑/↓ navigate • ") + StyleCommand.Render("type to filter") + StyleDim.Render(" • enter select • esc back")
	}
	b.WriteString(helpText + "\n")

	return b.String()
}

// filteredResults returns results matching the current filter.
func (m searchPicker) filteredResults() []types.SearchResult {
	if m.filter == "" {
		return m.results
	}
	lower := strings.ToLower(m.filter)
	var out []types.SearchResult
	for _, r := range m.results {
		if strings.Contains(strings.ToLower(r.Title), lower) ||
			strings.Contains(strings.ToLower(r.Provider), lower) {
			out = append(out, r)
		}
	}
	return out
}

// runStreamingSearch launches a parallel search and runs the streaming picker.
// Returns the selected URL, or "" if no results were found. Returns ErrUserBack on esc.
func runStreamingSearch(ctx context.Context, query string) (string, error) {
	ch := autotitle.SearchStream(ctx, query)
	picker := newSearchPicker(ch)

	p := tea.NewProgram(picker, tea.WithFilter(wizardFilter))
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("search picker failed: %w", err)
	}

	m := finalModel.(searchPicker)

	if m.aborted {
		if interceptedKey == "ctrl+c" {
			fmt.Println()
			logger.Info(StyleDim.Render("Init cancelled"))
			return "", huh.ErrUserAborted
		}
		return "", huh.ErrUserAborted
	}

	if m.chosen {
		return m.selected, nil
	}

	// Done but no results selected (no results found)
	return "", nil
}
