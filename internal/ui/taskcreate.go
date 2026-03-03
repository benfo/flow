package ui

import (
	"github.com/ben-fourie/flow-cli/internal/tasks"
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

	parentTask *tasks.Task // non-nil when creating a subtask
	showAssign bool        // true when provider supports SelfAssigner
	assignToSelf bool      // whether to assign task to current user on create
	width      int
	height     int
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
