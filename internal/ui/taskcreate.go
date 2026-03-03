package ui

import (
	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// createFocus identifies which field in the create form is active.
type createFocus int

const (
	createFocusTitle    createFocus = iota
	createFocusDesc     createFocus = iota
	createFocusPriority createFocus = iota
	createFocusAssign   createFocus = iota
)

// priorityOrder is the cycle order for the priority picker.
var priorityOrder = []tasks.Priority{
	tasks.PriorityLow,
	tasks.PriorityMedium,
	tasks.PriorityHigh,
	tasks.PriorityCritical,
}

// TaskCreateModel is the embedded child model used by viewCreate. It handles
// title, description, and priority inputs, delegating the actual Create call
// to the parent (app.go) so the provider dependency stays outside the UI layer.
type TaskCreateModel struct {
	titleInput textinput.Model
	descInput  textarea.Model
	priority   tasks.Priority
	focused    createFocus
	saving     bool
	errMsg     string
	spinner    spinner.Model

	parentTask   *tasks.Task // non-nil when creating a subtask
	showAssign   bool        // true when provider supports SelfAssigner
	assignToSelf bool        // whether to assign task to current user on create
	confirming   bool        // true when asking user to confirm discard
	width        int
	height       int
}

// HasChanges reports whether the user has entered any content.
func (m TaskCreateModel) HasChanges() bool {
	return m.titleInput.Value() != "" || m.descInput.Value() != ""
}

// NewTaskCreateModel builds an empty create form. Pass a non-nil parentTask
// to pre-populate the "subtask of" context banner.
func NewTaskCreateModel(parent *tasks.Task) TaskCreateModel {
	ti := textinput.New()
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.PlaceholderStyle = dimStyle
	ti.Placeholder = "Task title"
	ti.Focus()

	ta := textarea.New()
	ta.Placeholder = "Task description (optional)"
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorText)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(colorText)
	ta.FocusedStyle.Placeholder = dimStyle
	ta.BlurredStyle.Placeholder = dimStyle

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	return TaskCreateModel{
		titleInput: ti,
		descInput:  ta,
		priority:   tasks.PriorityMedium,
		focused:    createFocusTitle,
		spinner:    sp,
		parentTask: parent,
	}
}

// SetSaving toggles the saving state (called by app.go when dispatch starts/ends).
func (m TaskCreateModel) SetSaving(saving bool) TaskCreateModel {
	m.saving = saving
	return m
}

// SetError sets an inline error message returned from the provider.
func (m TaskCreateModel) SetError(msg string) TaskCreateModel {
	m.errMsg = msg
	m.saving = false
	return m
}

// BuildInput returns the CreateInput to pass to the provider.
func (m TaskCreateModel) BuildInput() tasks.CreateInput {
	parentID := ""
	if m.parentTask != nil {
		parentID = m.parentTask.ID
	}
	return tasks.CreateInput{
		Title:        m.titleInput.Value(),
		Description:  m.descInput.Value(),
		Priority:     m.priority,
		ParentID:     parentID,
		AssignToSelf: m.assignToSelf,
	}
}

// applyWidths sets component widths from m.width/m.height.
func (m *TaskCreateModel) applyWidths() {
	if m.titleInput.Placeholder == "" {
		return // model not yet initialised; skip to avoid nil-pointer in textarea
	}
	contentW := max(20, m.width-8)
	m.titleInput.Width = contentW
	m.descInput.SetWidth(contentW)
	// Reserve rows for priority (3), optional assign checkbox (3), and context banner (4).
	overhead := 20
	if m.parentTask != nil {
		overhead += 4
	}
	if m.showAssign {
		overhead += 3
	}
	m.descInput.SetHeight(max(4, m.height-overhead))
}

// ── tea.Model (child — called by app.go) ──────────────────────────────────────

func (m TaskCreateModel) Update(msg tea.Msg) (TaskCreateModel, tea.Cmd) {
	if m.saving {
		if tick, ok := msg.(spinner.TickMsg); ok {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(tick)
			return m, cmd
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyWidths()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			return m.cycleFocus(true), textinput.Blink
		case "shift+tab":
			return m.cycleFocus(false), textinput.Blink
		case "left":
			if m.focused == createFocusPriority {
				return m.shiftPriority(false), nil
			}
		case "right":
			if m.focused == createFocusPriority {
				return m.shiftPriority(true), nil
			}
		case " ":
			if m.focused == createFocusAssign {
				m.assignToSelf = !m.assignToSelf
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch m.focused {
	case createFocusTitle:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case createFocusDesc:
		m.descInput, cmd = m.descInput.Update(msg)
	}
	return m, cmd
}

func (m TaskCreateModel) cycleFocus(forward bool) TaskCreateModel {
	m.titleInput.Blur()
	m.descInput.Blur()
	total := 3
	if m.showAssign {
		total = 4
	}
	if forward {
		m.focused = createFocus((int(m.focused) + 1) % total)
	} else {
		m.focused = createFocus((int(m.focused) + total - 1) % total)
	}
	switch m.focused {
	case createFocusTitle:
		m.titleInput.Focus()
	case createFocusDesc:
		m.descInput.Focus()
	}
	return m
}

func (m TaskCreateModel) shiftPriority(inc bool) TaskCreateModel {
	idx := 0
	for i, p := range priorityOrder {
		if p == m.priority {
			idx = i
			break
		}
	}
	if inc {
		idx = min(idx+1, len(priorityOrder)-1)
	} else {
		idx = max(idx-1, 0)
	}
	m.priority = priorityOrder[idx]
	return m
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m TaskCreateModel) View() string {
	var parts []string

	// Context banner shown when creating a subtask.
	if m.parentTask != nil {
		banner := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1).
			Width(m.width - 8).
			Render(
				dimStyle.Render("Subtask of  ") +
					lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(m.parentTask.ID) +
					"  " + dimStyle.Render(m.parentTask.Title),
			)
		parts = append(parts, lipgloss.NewStyle().Padding(1, 2, 0).Render(banner))
	}

	parts = append(parts,
		renderFormField("Title", m.titleInput.View(), m.focused == createFocusTitle),
		renderFormField("Description", m.descInput.View(), m.focused == createFocusDesc),
		m.renderPriorityField(),
	)

	if m.showAssign {
		parts = append(parts, m.renderAssignField())
	}

	switch {
	case m.confirming:
		parts = append(parts, renderDiscardConfirm())
	case m.saving:
		parts = append(parts,
			lipgloss.NewStyle().Foreground(colorPrimary).Padding(0, 2).
				Render(m.spinner.View()+"  Creating…"))
	case m.errMsg != "":
		parts = append(parts,
			lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(0, 2).
				Render("✗  "+m.errMsg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m TaskCreateModel) renderAssignField() string {
	focused := m.focused == createFocusAssign

	check := "☐"
	if m.assignToSelf {
		check = lipgloss.NewStyle().Foreground(colorPrimary).Render("☑")
	}
	labelStyle := dimStyle
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(colorText).Bold(true)
	}
	value := check + "  " + labelStyle.Render("Assign to me")

	borderColor := colorBorder
	if focused {
		borderColor = colorPrimary
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(value)

	return lipgloss.JoinVertical(lipgloss.Left,
		editLabelStyle(focused).Padding(1, 2, 0, 2).Render("Assign"),
		lipgloss.NewStyle().Padding(0, 2).Render(box),
	)
}

func (m TaskCreateModel) renderPriorityField() string {
	focused := m.focused == createFocusPriority

	arrowStyle := dimStyle
	if focused {
		arrowStyle = lipgloss.NewStyle().Foreground(colorPrimary)
	}

	value := arrowStyle.Render("◀") +
		"  " +
		lipgloss.NewStyle().Foreground(priorityColor(m.priority)).Bold(true).Render(m.priority.String()) +
		"  " +
		arrowStyle.Render("▶")

	borderColor := colorBorder
	if focused {
		borderColor = colorPrimary
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(value)

	return lipgloss.JoinVertical(lipgloss.Left,
		editLabelStyle(focused).Padding(1, 2, 0, 2).Render("Priority"),
		lipgloss.NewStyle().Padding(0, 2).Render(box),
	)
}

// ── Create view ───────────────────────────────────────────────────────────────

func (m Model) openCreateView(parent *tasks.Task) (tea.Model, tea.Cmd) {
	if _, ok := m.provider.(tasks.Creator); !ok {
		m.statusMessage = "✗  This provider does not support creating tasks"
		return m, nil
	}

	cm := NewTaskCreateModel(parent)
	cm.width = m.width
	cm.height = m.height
	if _, ok := m.provider.(tasks.SelfAssigner); ok {
		cm.showAssign = true
	}
	cm.applyWidths()
	m.createModel = cm
	m.state = viewCreate
	m.statusMessage = ""
	return m, textinput.Blink
}

func (m Model) updateCreateView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Handle discard confirmation mode first.
		if m.createModel.confirming {
			switch key.String() {
			case "y", "enter":
				m.createModel.confirming = false
				if m.createModel.parentTask != nil {
					m.state = viewDetail
				} else {
					m.state = viewList
				}
			case "n", "esc":
				m.createModel.confirming = false
			}
			return m, nil
		}
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.createModel.HasChanges() {
				m.createModel.confirming = true
				return m, nil
			}
			if m.createModel.parentTask != nil {
				m.state = viewDetail
			} else {
				m.state = viewList
			}
			return m, nil
		case "ctrl+s":
			if m.createModel.saving {
				return m, nil
			}
			if m.createModel.BuildInput().Title == "" {
				m.createModel = m.createModel.SetError("Title is required")
				return m, nil
			}
			creator := m.provider.(tasks.Creator)
			m.createModel = m.createModel.SetSaving(true)
			return m, tea.Batch(
				m.createModel.spinner.Tick,
				createTaskCmd(creator, m.createModel.BuildInput()),
			)
		}
	}

	var cmd tea.Cmd
	m.createModel, cmd = m.createModel.Update(msg)
	return m, cmd
}

func createTaskCmd(c tasks.Creator, input tasks.CreateInput) tea.Cmd {
	return func() tea.Msg {
		t, err := c.Create(input)
		return taskCreatedMsg{task: t, err: err}
	}
}

func deleteTaskCmd(d tasks.TaskDeleter, t tasks.Task) tea.Cmd {
	return func() tea.Msg {
		err := d.DeleteTask(t.ID)
		return taskDeletedMsg{taskID: t.ID, parentID: t.ParentID, err: err}
	}
}

func (m Model) handleTaskDeleted(msg taskDeletedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = "✗  " + msg.err.Error()
		m.state = viewDetail
		return m, nil
	}

	// Remove the task (and any subtasks) from the local slice.
	var updated []tasks.Task
	for _, t := range m.tasks {
		if t.ID == msg.taskID || t.ParentID == msg.taskID {
			continue
		}
		updated = append(updated, t)
	}
	// Update parent HasChildren flag if this was a subtask.
	if msg.parentID != "" {
		hasChild := false
		for _, t := range updated {
			if t.ParentID == msg.parentID {
				hasChild = true
				break
			}
		}
		if !hasChild {
			for i := range updated {
				if updated[i].ID == msg.parentID {
					updated[i].HasChildren = false
					break
				}
			}
		}
	}
	m.tasks = updated
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = m.makeTaskItem(t)
	}
	m.list.SetItems(items)

	if msg.parentID != "" {
		// Return to parent task detail and refresh subtasks.
		if m.detailReturnTask != nil {
			parent := *m.detailReturnTask
			parentReturnState := m.detailParentReturnState
			m.detailReturnTask = nil
			m.subtasks = nil
			m.subtaskCursor = 0
			m.statusMessage = "✓  Deleted " + msg.taskID
			return m.openDetailForTask(parent, parentReturnState)
		}
		m.state = viewList
		return m, nil
	}

	m.selectedTask = nil
	m.state = m.detailReturnState
	m.statusMessage = ""
	return m, nil
}

func (m Model) handleTaskCreated(msg taskCreatedMsg) (tea.Model, tea.Cmd) {
	m.createModel = m.createModel.SetSaving(false)
	if msg.err != nil {
		m.createModel = m.createModel.SetError(msg.err.Error())
		return m, nil
	}

	// Add new task to the local slice and refresh the list.
	m.tasks = append(m.tasks, msg.task)
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = m.makeTaskItem(t)
	}
	m.list.SetItems(items)

	// If this was a subtask, go back to the parent detail and reload subtasks.
	if msg.task.ParentID != "" {
		// Update HasChildren flag on the parent task.
		for i := range m.tasks {
			if m.tasks[i].ID == msg.task.ParentID {
				m.tasks[i].HasChildren = true
				if m.selectedTask != nil && m.selectedTask.ID == msg.task.ParentID {
					m.selectedTask = &m.tasks[i]
				}
			}
		}
		m.subtasks = append(m.subtasks, msg.task)
		// Adjust viewport height for the new subtask.
		subtaskSectionH := subtaskSectionHeight(len(m.subtasks))
		m.detail.Height = max(3, m.height-verticalOverhead-subtaskSectionH)
		m.state = viewDetail
		m.statusMessage = "✓  Subtask created: " + msg.task.ID
		return m, nil
	}

	// For top-level tasks, open the detail view immediately.
	m.statusMessage = "✓  Task created: " + msg.task.ID
	return m.openDetailForTask(msg.task, viewList)
}

func (m Model) renderCreateView() string {
	isSubtask := m.createModel.parentTask != nil

	var breadcrumb string
	if isSubtask && m.selectedTask != nil {
		breadcrumb = "⚡ flow  /  " + m.selectedTask.ID + "  /  new subtask"
	} else {
		breadcrumb = "⚡ flow  /  new task"
	}

	sep := renderSeparator(m.width)
	header := renderHeaderBar(breadcrumb, m.width)
	footer := renderFooterBar(fitHints([]string{"ctrl+s  save", "esc  discard"}, "   ", m.width-2), m.width)
	if m.createModel.confirming {
		footer = renderFooterBar("Discard changes?   y  yes   n / esc  keep editing", m.width)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		m.createModel.View(),
		sep,
		footer,
	)
}

