package ui

import (
	"fmt"

	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commentMode tracks what the comments view is currently doing.
type commentMode int

const (
	commentModeList    commentMode = iota // browsing the comment list
	commentModeCompose                    // composing a new or edited comment
	commentModeDelete                     // confirming a delete
)

// TaskCommentsModel is the sub-model for viewComments.
type TaskCommentsModel struct {
	taskID   string
	comments []tasks.Comment
	cursor   int
	mode     commentMode

	// compose fields
	input      textarea.Model
	editingID  string // empty = new comment, non-empty = editing existing
	saving     bool
	confirming bool   // true when confirming discard of compose changes
	errMsg     string
	spinner    spinner.Model

	width  int
	height int
}

// NewTaskCommentsModel builds an empty model for the given task.
func NewTaskCommentsModel(taskID string) TaskCommentsModel {
	ta := textarea.New()
	ta.Placeholder = "Write a comment…"
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

	return TaskCommentsModel{
		taskID:  taskID,
		input:   ta,
		spinner: sp,
	}
}

// SetComments loads comments into the model (called after async fetch).
func (m TaskCommentsModel) SetComments(comments []tasks.Comment) TaskCommentsModel {
	m.comments = comments
	m.cursor = 0
	return m
}

// openCompose switches to compose mode. Pass an existing comment to edit it,
// or a zero-value Comment to compose a new one.
func (m TaskCommentsModel) openCompose(existing tasks.Comment) TaskCommentsModel {
	m.mode = commentModeCompose
	m.editingID = existing.ID
	m.errMsg = ""
	m.confirming = false
	m.saving = false
	m.input.SetValue(existing.Body)
	m.input.Focus()
	m.applyWidths()
	return m
}

// HasComposeChanges reports whether the compose textarea has content.
func (m TaskCommentsModel) HasComposeChanges() bool {
	if m.editingID != "" {
		// editing: changed if differs from original
		for _, c := range m.comments {
			if c.ID == m.editingID {
				return m.input.Value() != c.Body
			}
		}
	}
	return m.input.Value() != ""
}

// ComposeBody returns the trimmed textarea value.
func (m TaskCommentsModel) ComposeBody() string { return m.input.Value() }

// SelectedComment returns the comment under the cursor, or zero value.
func (m TaskCommentsModel) SelectedComment() (tasks.Comment, bool) {
	if len(m.comments) == 0 || m.cursor < 0 || m.cursor >= len(m.comments) {
		return tasks.Comment{}, false
	}
	return m.comments[m.cursor], true
}

func (m *TaskCommentsModel) applyWidths() {
	contentW := max(20, m.width-8)
	m.input.SetWidth(contentW)
	// compose height = total height minus header/sep/footer/label overhead
	m.input.SetHeight(max(4, m.height-16))
}

// ── tea.Model (child) ─────────────────────────────────────────────────────────

func (m TaskCommentsModel) Update(msg tea.Msg) (TaskCommentsModel, tea.Cmd) {
	// Spinner tick while saving.
	if m.saving {
		if tick, ok := msg.(spinner.TickMsg); ok {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(tick)
			return m, cmd
		}
		return m, nil
	}

	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
		m.applyWidths()
		return m, nil
	}

	if m.mode == commentModeCompose {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m TaskCommentsModel) View() string {
	switch m.mode {
	case commentModeCompose:
		return m.renderCompose()
	default:
		return m.renderList()
	}
}

func (m TaskCommentsModel) renderList() string {
	if len(m.comments) == 0 {
		return dimStyle.Padding(2, 2).Render("No comments yet.  Press n to add one.")
	}

	maxVisible := max(1, m.height-10)
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.comments))

	var rows []string
	for idx := start; idx < end; idx++ {
		c := m.comments[idx]
		selected := idx == m.cursor

		authorStyle := dimStyle
		if selected {
			authorStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
		}
		meta := authorStyle.Render(c.Author) + dimStyle.Render("  "+c.CreatedAt)

		bodyStyle := lipgloss.NewStyle().Foreground(colorText)
		if selected {
			bodyStyle = bodyStyle.Bold(true)
		}
		// Truncate long bodies to two lines for list view.
		body := truncateComment(c.Body, m.width-10)

		cursor := "  "
		if selected {
			cursor = lipgloss.NewStyle().Foreground(colorPrimary).Render("▶ ")
		}

		row := lipgloss.JoinVertical(lipgloss.Left,
			cursor+meta,
			"  "+bodyStyle.Render(body),
		)
		borderColor := colorBorder
		if selected {
			borderColor = colorPrimary
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			Width(m.width - 8).
			Render(row)
		rows = append(rows, lipgloss.NewStyle().Padding(0, 2).Render(box))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m TaskCommentsModel) renderCompose() string {
	label := "New comment"
	if m.editingID != "" {
		label = "Edit comment"
	}

	field := renderFormField(label, m.input.View(), true)

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

	return lipgloss.JoinVertical(lipgloss.Left, field, statusLine)
}

// truncateComment wraps or truncates comment body to fit in the list view.
func truncateComment(body string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 60
	}
	// Show at most 2 lines of ~maxWidth chars.
	lines := splitLines(body, maxWidth)
	if len(lines) > 2 {
		return lines[0] + "\n" + lines[1] + dimStyle.Render(" …")
	}
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

func splitLines(s string, width int) []string {
	var out []string
	for len(s) > width {
		out = append(out, s[:width])
		s = s[width:]
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// countLabel returns "1 comment" / "N comments".
func countLabel(n int) string {
	if n == 1 {
		return "1 comment"
	}
	return fmt.Sprintf("%d comments", n)
}
