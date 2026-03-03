package ui

import (
	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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

// ── Status view ───────────────────────────────────────────────────────────────

func (m Model) openStatusView() (tea.Model, tea.Cmd) {
	if m.selectedTask == nil {
		return m, nil
	}
	updater, ok := m.provider.(tasks.StatusUpdater)
	if !ok {
		m.statusMessage = "✗  This provider does not support status changes"
		return m, nil
	}
	m.statusLoading = true
	m.statusMessage = ""
	m.state = viewStatus
	return m, tea.Batch(m.spinner.Tick, loadTransitionsCmd(updater, m.selectedTask.ID))
}

func loadTransitionsCmd(u tasks.StatusUpdater, taskID string) tea.Cmd {
	return func() tea.Msg {
		ts, err := u.GetTransitions(taskID)
		return transitionsLoadedMsg{transitions: ts, err: err}
	}
}

// autoTransitionMsg carries transitions loaded specifically to auto-apply In Progress.
type autoTransitionMsg struct {
	updater     tasks.StatusUpdater
	taskID      string
	transitions []tasks.StatusTransition
	err         error
}

// loadTransitionsForAutoCmd is defined in branch.go.

func transitionTaskCmd(u tasks.StatusUpdater, taskID, transitionID string) tea.Cmd {
	return func() tea.Msg {
		t, err := u.TransitionTask(taskID, transitionID)
		return taskTransitionedMsg{task: t, err: err}
	}
}

func (m Model) handleTransitionsLoaded(msg transitionsLoadedMsg) (tea.Model, tea.Cmd) {
	m.statusLoading = false
	if msg.err != nil {
		m.statusMessage = "✗  " + msg.err.Error()
		m.state = viewDetail
		return m, nil
	}
	sm := NewTaskStatusModel(msg.transitions)
	sm.width = m.width
	sm.height = m.height
	m.statusModel = sm
	return m, nil
}

func (m Model) handleTaskTransitioned(msg taskTransitionedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = "✗  " + msg.err.Error()
		m.state = viewDetail
		return m, nil
	}

	// Update the task in local state.
	for i := range m.tasks {
		if m.tasks[i].ID == msg.task.ID {
			m.tasks[i] = msg.task
			m.selectedTask = &m.tasks[i]
			break
		}
	}
	// Also update subtasks if the transitioned task was a subtask.
	for i := range m.subtasks {
		if m.subtasks[i].ID == msg.task.ID {
			m.subtasks[i] = msg.task
			break
		}
	}
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = m.makeTaskItem(t)
	}
	m.list.SetItems(items)
	m.detail.SetContent(renderTaskDetail(*m.selectedTask, m.width, m.branchForTask(m.selectedTask.ID)))

	m.statusMessage = "✓  Status changed to: " + msg.task.Status.String()
	m.state = viewDetail
	return m, nil
}

func (m Model) updateStatusView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Keep spinner ticking while loading transitions.
	if m.statusLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.state = viewDetail
			return m, nil
		case "up", "k":
			m.statusModel = m.statusModel.MoveUp()
		case "down", "j":
			m.statusModel = m.statusModel.MoveDown()
		case "enter":
			tr, ok := m.statusModel.Selected()
			if !ok {
				return m, nil
			}
			updater := m.provider.(tasks.StatusUpdater)
			m.statusLoading = true
			return m, tea.Batch(m.spinner.Tick, transitionTaskCmd(updater, m.selectedTask.ID, tr.ID))
		}
	}
	return m, nil
}

func (m Model) renderStatusView() string {
	if m.selectedTask == nil {
		return ""
	}
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  change status", m.width)
	footer := renderFooterBar("↑/↓  navigate   enter  confirm   esc  cancel", m.width)

	var content string
	if m.statusLoading {
		content = lipgloss.NewStyle().Padding(2, 2).Foreground(colorText).
			Render(m.spinner.View() + "  Loading…")
	} else {
		content = m.statusModel.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		content,
		sep,
		footer,
	)
}

