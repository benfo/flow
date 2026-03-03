package ui

import (
	"fmt"

	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskSearchModel is the child model for viewSearch. It holds a text input for
// the query and the result slice returned by the provider's Search() call.
// There are no focus modes — the input always accepts typing, and arrow keys
// always navigate the results list.
type TaskSearchModel struct {
	input   textinput.Model
	results []tasks.Task
	cursor  int // -1 means no result selected
	width   int
	height  int
}

// NewTaskSearchModel builds an empty search model ready for user input.
func NewTaskSearchModel() TaskSearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search tasks…"
	ti.Focus()
	ti.PromptStyle = dimStyle
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	return TaskSearchModel{input: ti, cursor: -1}
}

// Query returns the current input value.
func (m TaskSearchModel) Query() string {
	return m.input.Value()
}

// SetResults stores search results and resets the cursor to -1 so the next
// enter fires a new search rather than opening a stale result.
func (m TaskSearchModel) SetResults(results []tasks.Task) TaskSearchModel {
	m.results = results
	m.cursor = -1
	return m
}

// ResetCursor clears the result selection when the query changes.
func (m TaskSearchModel) ResetCursor() TaskSearchModel {
	m.cursor = -1
	return m
}

// Selected returns the task under the cursor, or false if none is selected.
func (m TaskSearchModel) Selected() (tasks.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.results) {
		return tasks.Task{}, false
	}
	return m.results[m.cursor], true
}

func (m TaskSearchModel) MoveUp() TaskSearchModel {
	if m.cursor > 0 {
		m.cursor--
	} else {
		m.cursor = -1 // deselect when moving above first result
	}
	return m
}

func (m TaskSearchModel) MoveDown() TaskSearchModel {
	if len(m.results) == 0 {
		return m
	}
	if m.cursor < len(m.results)-1 {
		m.cursor++
	}
	return m
}

func (m TaskSearchModel) PageDown() TaskSearchModel {
	if len(m.results) == 0 {
		return m
	}
	start := max(m.cursor, 0)
	m.cursor = min(start+m.pageSize(), len(m.results)-1)
	return m
}

func (m TaskSearchModel) PageUp() TaskSearchModel {
	if m.cursor <= 0 {
		m.cursor = -1
		return m
	}
	m.cursor = max(m.cursor-m.pageSize(), 0)
	return m
}

func (m TaskSearchModel) pageSize() int {
	ps := m.height - 10
	if ps < 1 {
		ps = 5
	}
	return ps
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m TaskSearchModel) View() string {
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(m.width - 6).
		Render(m.input.View())

	parts := []string{
		lipgloss.NewStyle().Padding(1, 2).Render(inputBox),
	}

	if len(m.results) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	// Results header.
	resultsHeader := dimStyle.Padding(0, 2).
		Render(fmt.Sprintf("── Results (%d)", len(m.results)))
	parts = append(parts, resultsHeader)

	// Result rows — sliding window that keeps the selected row visible.
	maxVisible := m.pageSize()
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.results))

	for i := start; i < end; i++ {
		t := m.results[i]
		cursorStr := "  "
		idStyle := lipgloss.NewStyle().Foreground(colorPrimary)
		rowStyle := dimStyle

		if i == m.cursor {
			cursorStr = lipgloss.NewStyle().Foreground(colorPrimary).Render("▶ ")
			rowStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
		}

		id := idStyle.Render(t.ID)
		title := rowStyle.Render(t.Title)
		status := lipgloss.NewStyle().Foreground(statusColor(t.Status)).Render("● " + t.Status.String())
		assignee := dimStyle.Render(t.Assignee)

		row := cursorStr + id + "  " + title + "  " + status + "  " + assignee
		parts = append(parts, lipgloss.NewStyle().Padding(0, 2).Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ── Search view ───────────────────────────────────────────────────────────────

func (m Model) openSearchView() (tea.Model, tea.Cmd) {
	if _, ok := m.provider.(tasks.Searcher); !ok {
		m.statusMessage = "✗  This provider does not support search"
		return m, nil
	}
	sm := NewTaskSearchModel()
	sm.width = m.width
	sm.height = m.height
	sm.input.Width = m.width - 10
	m.searchModel = sm
	m.searchLoading = false
	m.searchReturnState = m.state
	m.state = viewSearch
	m.statusMessage = ""
	return m, textinput.Blink
}

func searchCmd(s tasks.Searcher, query string) tea.Cmd {
	return func() tea.Msg {
		ts, err := s.Search(query)
		return searchResultsMsg{tasks: ts, err: err}
	}
}

func (m Model) handleSearchResults(msg searchResultsMsg) (tea.Model, tea.Cmd) {
	m.searchLoading = false
	if msg.err != nil {
		return m, nil
	}
	m.searchModel = m.searchModel.SetResults(msg.tasks)
	return m, nil
}

func (m Model) updateSearchView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.searchLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.state = m.searchReturnState
			return m, nil
		case "enter":
			// Open the selected result if one is highlighted; otherwise search.
			if t, ok := m.searchModel.Selected(); ok {
				return m.openDetailForTask(t, viewSearch)
			}
			q := m.searchModel.Query()
			if q == "" {
				return m, nil
			}
			m.searchLoading = true
			searcher := m.provider.(tasks.Searcher)
			return m, tea.Batch(m.spinner.Tick, searchCmd(searcher, q))
		case "up":
			m.searchModel = m.searchModel.MoveUp()
			return m, nil
		case "down":
			m.searchModel = m.searchModel.MoveDown()
			return m, nil
		case "pgdown":
			m.searchModel = m.searchModel.PageDown()
			return m, nil
		case "pgup":
			m.searchModel = m.searchModel.PageUp()
			return m, nil
		}
	}

	// All other keys go to the text input. If the value changes, reset the
	// cursor so the next enter runs a fresh search rather than opening a
	// stale result.
	prev := m.searchModel.Query()
	var cmd tea.Cmd
	m.searchModel.input, cmd = m.searchModel.input.Update(msg)
	if m.searchModel.Query() != prev {
		m.searchModel = m.searchModel.ResetCursor()
	}
	return m, cmd
}

func (m Model) renderSearchView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  find", m.width)

	var footerText string
	if m.searchLoading {
		footerText = "searching…"
	} else {
		footerText = "enter  open   esc  back"
	}
	footer := renderFooterBar(footerText, m.width)

	var content string
	if m.searchLoading {
		content = lipgloss.NewStyle().Padding(2, 2).Foreground(colorText).
			Render(m.spinner.View() + "  Searching…")
	} else {
		content = m.searchModel.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}


