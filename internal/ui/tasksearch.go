package ui

import (
	"fmt"

	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// searchFocus identifies which region of the search view has keyboard focus.
type searchFocus int

const (
	searchFocusInput   searchFocus = iota
	searchFocusResults searchFocus = iota
)

// TaskSearchModel is the child model for viewSearch. It holds a text input for
// the query and the result slice returned by the provider's Search() call.
type TaskSearchModel struct {
	input   textinput.Model
	results []tasks.Task
	cursor  int
	focus   searchFocus
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
	return TaskSearchModel{input: ti, focus: searchFocusInput}
}

// Query returns the current trimmed input value.
func (m TaskSearchModel) Query() string {
	return m.input.Value()
}

// SetResults stores search results and shifts focus to the results list.
func (m TaskSearchModel) SetResults(results []tasks.Task) TaskSearchModel {
	m.results = results
	m.cursor = 0
	if len(results) > 0 {
		m.focus = searchFocusResults
	}
	return m
}

// Selected returns the task under the cursor, if any.
func (m TaskSearchModel) Selected() (tasks.Task, bool) {
	if len(m.results) == 0 || m.cursor < 0 || m.cursor >= len(m.results) {
		return tasks.Task{}, false
	}
	return m.results[m.cursor], true
}

func (m TaskSearchModel) MoveUp() TaskSearchModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

func (m TaskSearchModel) MoveDown() TaskSearchModel {
	if m.cursor < len(m.results)-1 {
		m.cursor++
	}
	return m
}

func (m TaskSearchModel) FocusInput() TaskSearchModel {
	m.focus = searchFocusInput
	m.input.Focus()
	return m
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

	// Result rows — sliding window that keeps the cursor visible.
	focused := m.focus == searchFocusResults
	maxVisible := m.height - 10 // leave room for header/footer/input
	if maxVisible < 1 {
		maxVisible = 5
	}

	// Compute the start index so the cursor is always within the window.
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.results))

	for i := start; i < end; i++ {
		t := m.results[i]
		cursor := "  "
		idStyle := lipgloss.NewStyle().Foreground(colorPrimary)
		rowStyle := dimStyle

		if focused && i == m.cursor {
			cursor = lipgloss.NewStyle().Foreground(colorPrimary).Render("▶ ")
			rowStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
		}

		id := idStyle.Render(t.ID)
		title := rowStyle.Render(t.Title)
		status := lipgloss.NewStyle().Foreground(statusColor(t.Status)).Render("● " + t.Status.String())
		assignee := dimStyle.Render(t.Assignee)

		row := cursor + id + "  " + title + "  " + status + "  " + assignee
		parts = append(parts, lipgloss.NewStyle().Padding(0, 2).Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
