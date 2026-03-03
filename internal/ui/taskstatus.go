package ui

import (
	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/lipgloss"
)

// TaskStatusModel is the embedded child model used by viewStatus.
// It shows the available workflow transitions for the selected task and lets
// the user pick one with ↑/↓ + Enter.
type TaskStatusModel struct {
	transitions []tasks.StatusTransition
	cursor      int
	width       int
	height      int
}

// NewTaskStatusModel builds a status picker pre-loaded with transitions.
func NewTaskStatusModel(transitions []tasks.StatusTransition) TaskStatusModel {
	return TaskStatusModel{transitions: transitions}
}

// Selected returns the transition under the cursor, or the zero value if the
// list is empty.
func (m TaskStatusModel) Selected() (tasks.StatusTransition, bool) {
	if len(m.transitions) == 0 {
		return tasks.StatusTransition{}, false
	}
	return m.transitions[m.cursor], true
}

func (m TaskStatusModel) MoveUp() TaskStatusModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

func (m TaskStatusModel) MoveDown() TaskStatusModel {
	if m.cursor < len(m.transitions)-1 {
		m.cursor++
	}
	return m
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m TaskStatusModel) View() string {
	if len(m.transitions) == 0 {
		return lipgloss.NewStyle().Padding(2, 2).Foreground(colorSubtle).
			Render("No transitions available for this task.")
	}

	var rows []string
	for i, tr := range m.transitions {
		selected := i == m.cursor

		cursor := "  "
		labelStyle := dimStyle
		if selected {
			cursor = lipgloss.NewStyle().Foreground(colorPrimary).Render("▶ ")
			labelStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
		}

		statusDot := lipgloss.NewStyle().Foreground(statusColor(tr.To)).Render("●")
		label := labelStyle.Render(tr.Name)
		row := lipgloss.NewStyle().Padding(0, 2).
			Render(cursor + statusDot + "  " + label)
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
