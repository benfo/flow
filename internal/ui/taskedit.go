package ui

import (
	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// editFocus identifies which field in the edit form is active.
type editFocus int

const (
	editFocusTitle editFocus = iota
	editFocusDesc
)

// TaskEditModel is the embedded child model used by viewEdit. It handles the
// title and description inputs and the saving spinner, but delegates save
// dispatching to the parent (app.go) so the provider dependency stays outside
// the UI layer.
type TaskEditModel struct {
	titleInput textinput.Model
	descInput  textarea.Model
	focused    editFocus
	saving     bool
	errMsg     string
	spinner    spinner.Model

	originalTask tasks.Task
	confirming   bool // true when asking user to confirm discard
	width        int
	height       int
}

// NewTaskEditModel builds the edit form pre-filled with the task's current
// title and description.
func NewTaskEditModel(task tasks.Task) TaskEditModel {
	ti := textinput.New()
	ti.SetValue(task.Title)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.PlaceholderStyle = dimStyle
	ti.Placeholder = "Task title"
	ti.Focus()

	ta := textarea.New()
	ta.SetValue(task.Description)
	ta.Placeholder = "Task description"
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	// Remove built-in border — we render the border manually for consistent
	// width handling across both fields.
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	// Colour the text content.
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorText)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(colorText)
	ta.FocusedStyle.Placeholder = dimStyle
	ta.BlurredStyle.Placeholder = dimStyle

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	return TaskEditModel{
		titleInput:   ti,
		descInput:    ta,
		focused:      editFocusTitle,
		spinner:      sp,
		originalTask: task,
	}
}

// SetSaving toggles the saving state (called by app.go when dispatch starts/ends).
func (m TaskEditModel) SetSaving(saving bool) TaskEditModel {
	m.saving = saving
	return m
}

// SetError sets an inline error message returned from the provider.
func (m TaskEditModel) SetError(msg string) TaskEditModel {
	m.errMsg = msg
	m.saving = false
	return m
}

// EditedTask returns a copy of the original task with updated title and description.
func (m TaskEditModel) EditedTask() tasks.Task {
	t := m.originalTask
	t.Title = m.titleInput.Value()
	t.Description = m.descInput.Value()
	return t
}

// HasChanges reports whether the user has modified title or description.
func (m TaskEditModel) HasChanges() bool {
	return m.titleInput.Value() != m.originalTask.Title ||
		m.descInput.Value() != m.originalTask.Description
}

// ── tea.Model (child — called by app.go) ──────────────────────────────────────

func (m TaskEditModel) Update(msg tea.Msg) (TaskEditModel, tea.Cmd) {
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
		case "tab", "shift+tab":
			return m.toggleFocus(), textinput.Blink
		}
	}

	var cmd tea.Cmd
	if m.focused == editFocusTitle {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else {
		m.descInput, cmd = m.descInput.Update(msg)
	}
	return m, cmd
}

// applyWidths sets component widths from m.width/m.height.
// Content width = terminal width − outer padding (4) − border (2) − inner padding (2).
func (m *TaskEditModel) applyWidths() {
	if m.originalTask.ID == "" {
		return // model not yet initialised; skip to avoid nil-pointer in textarea
	}
	contentW := max(20, m.width-8)
	m.titleInput.Width = contentW
	m.descInput.SetWidth(contentW)
	m.descInput.SetHeight(max(8, m.height-14))
}

func (m TaskEditModel) toggleFocus() TaskEditModel {
	if m.focused == editFocusTitle {
		m.titleInput.Blur()
		m.focused = editFocusDesc
		m.descInput.Focus()
	} else {
		m.descInput.Blur()
		m.focused = editFocusTitle
		m.titleInput.Focus()
	}
	return m
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// editLabelStyle returns a label style for the edit form.
// Unlike detailLabelStyle it has no fixed width, so "Description" doesn't wrap.
func editLabelStyle(focused bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(colorSubtle)
	if focused {
		s = s.Foreground(colorPrimary).Bold(true)
	}
	return s
}

func (m TaskEditModel) View() string {
	titleField := m.renderEditField("Title", m.titleInput.View(), m.focused == editFocusTitle)
	descField := m.renderEditField("Description", m.descInput.View(), m.focused == editFocusDesc)

	var statusLine string
	switch {
	case m.confirming:
		statusLine = renderDiscardConfirm()
	case m.saving:
		statusLine = lipgloss.NewStyle().Foreground(colorPrimary).Padding(0, 2).
			Render(m.spinner.View() + "  Saving…")
	case m.errMsg != "":
		statusLine = lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(0, 2).
			Render("✗  " + m.errMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleField, descField, statusLine)
}

func (m TaskEditModel) renderEditField(label, content string, focused bool) string {
	return renderFormField(label, content, focused)
}

