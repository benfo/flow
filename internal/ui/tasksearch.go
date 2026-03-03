package ui

import (
	"fmt"

	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/textinput"
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
