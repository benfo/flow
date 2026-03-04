package ui

import (
	"fmt"

	"github.com/benfo/flow/internal/tasks"
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
)

// TaskCommentsModel is the sub-model for viewComments.
type TaskCommentsModel struct {
	taskID   string
	comments []tasks.Comment
	cursor   int
	mode     commentMode

	// compose fields
	input     textarea.Model
	editingID string // empty = new comment, non-empty = editing existing
	saving    bool
	errMsg    string
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

// ── Comments ──────────────────────────────────────────────────────────────────

func (m Model) openCommentsView() (tea.Model, tea.Cmd) {
	if m.selectedTask == nil {
		return m, nil
	}
	lister, ok := m.provider.(tasks.CommentLister)
	if !ok {
		m.statusMessage = "✗  This provider does not support comments"
		return m, nil
	}
	m.commentsModel = NewTaskCommentsModel(m.selectedTask.ID)
	m.commentsModel.width = m.width
	m.commentsModel.height = m.height
	m.state = viewComments
	return m, loadCommentsCmd(lister, m.selectedTask.ID)
}

func loadCommentsCmd(l tasks.CommentLister, taskID string) tea.Cmd {
	return func() tea.Msg {
		comments, err := l.GetComments(taskID)
		return commentsLoadedMsg{comments: comments, err: err}
	}
}

func (m Model) handleCommentsLoaded(msg commentsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = "✗  " + msg.err.Error()
		return m, nil
	}
	m.commentsModel = m.commentsModel.SetComments(msg.comments)
	return m, nil
}

func (m Model) updateCommentsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Delegate resize and spinner to child model.
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		var cmd tea.Cmd
		m.commentsModel, cmd = m.commentsModel.Update(msg)
		return m, cmd
	}

	// While in compose mode, handle keys here; typing goes to child.
	if m.commentsModel.mode == commentModeCompose {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				if m.commentsModel.HasComposeChanges() {
					m.confirm = &confirmPrompt{
						question: "Discard changes?",
						onConfirm: func(m Model) (tea.Model, tea.Cmd) {
							m.commentsModel.mode = commentModeList
							return m, nil
						},
						onCancel: func(m Model) (tea.Model, tea.Cmd) { return m, nil },
					}
					return m, nil
				}
				m.commentsModel.mode = commentModeList
				return m, nil
			case "ctrl+s":
				if m.commentsModel.saving {
					return m, nil
				}
				body := m.commentsModel.ComposeBody()
				if body == "" {
					m.commentsModel.errMsg = "Comment cannot be empty"
					return m, nil
				}
				m.commentsModel.saving = true
				m.commentsModel.errMsg = ""
				taskID := m.commentsModel.taskID
				editingID := m.commentsModel.editingID
				var cmd tea.Cmd
				if editingID != "" {
					editor := m.provider.(tasks.CommentEditor)
					cmd = editCommentCmd(editor, taskID, editingID, body)
				} else {
					adder := m.provider.(tasks.CommentAdder)
					cmd = addCommentCmd(adder, taskID, body)
				}
				return m, tea.Batch(m.commentsModel.spinner.Tick, cmd)
			}
		}
		// Pass remaining keys (typing) to child model.
		var cmd tea.Cmd
		m.commentsModel, cmd = m.commentsModel.Update(msg)
		return m, cmd
	}

	// List mode.
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			m.state = viewDetail
			return m, nil
		case "up", "k":
			if m.commentsModel.cursor > 0 {
				m.commentsModel.cursor--
			}
			return m, nil
		case "down", "j":
			if m.commentsModel.cursor < len(m.commentsModel.comments)-1 {
				m.commentsModel.cursor++
			}
			return m, nil
		case "n":
			m.commentsModel = m.commentsModel.openCompose(tasks.Comment{})
			return m, nil
		case "e":
			if c, ok := m.commentsModel.SelectedComment(); ok {
				m.commentsModel = m.commentsModel.openCompose(c)
			}
			return m, nil
		case "D":
			if c, ok := m.commentsModel.SelectedComment(); ok {
				deleter := m.provider.(tasks.CommentDeleter)
				taskID := m.commentsModel.taskID
				commentID := c.ID
				m.confirm = &confirmPrompt{
					question:    "Delete this comment?",
					destructive: true,
					onConfirm: func(m Model) (tea.Model, tea.Cmd) {
						return m, deleteCommentCmd(deleter, taskID, commentID)
					},
					onCancel: func(m Model) (tea.Model, tea.Cmd) { return m, nil },
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func addCommentCmd(a tasks.CommentAdder, taskID, body string) tea.Cmd {
	return func() tea.Msg {
		c, err := a.AddComment(taskID, body)
		return commentSavedMsg{comment: c, err: err}
	}
}

func editCommentCmd(e tasks.CommentEditor, taskID, commentID, body string) tea.Cmd {
	return func() tea.Msg {
		c, err := e.EditComment(taskID, commentID, body)
		return commentSavedMsg{comment: c, err: err}
	}
}

func deleteCommentCmd(d tasks.CommentDeleter, taskID, commentID string) tea.Cmd {
	return func() tea.Msg {
		err := d.DeleteComment(taskID, commentID)
		return commentDeletedMsg{commentID: commentID, err: err}
	}
}

func (m Model) handleCommentSaved(msg commentSavedMsg) (tea.Model, tea.Cmd) {
	m.commentsModel.saving = false
	if msg.err != nil {
		m.commentsModel.errMsg = msg.err.Error()
		return m, nil
	}
	// Update or append in local list.
	found := false
	for i, c := range m.commentsModel.comments {
		if c.ID == msg.comment.ID {
			m.commentsModel.comments[i] = msg.comment
			found = true
			break
		}
	}
	if !found {
		m.commentsModel.comments = append(m.commentsModel.comments, msg.comment)
		m.commentsModel.cursor = len(m.commentsModel.comments) - 1
	}
	m.commentsModel.mode = commentModeList
	return m, nil
}

func (m Model) handleCommentDeleted(msg commentDeletedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = "✗  " + msg.err.Error()
		return m, nil
	}
	// Remove from local list.
	for i, c := range m.commentsModel.comments {
		if c.ID == msg.commentID {
			m.commentsModel.comments = append(
				m.commentsModel.comments[:i],
				m.commentsModel.comments[i+1:]...,
			)
			if m.commentsModel.cursor >= len(m.commentsModel.comments) && m.commentsModel.cursor > 0 {
				m.commentsModel.cursor--
			}
			break
		}
	}
	return m, nil
}

func (m Model) renderCommentsView() string {
	if m.selectedTask == nil {
		return ""
	}
	sep := renderSeparator(m.width)
	title := m.selectedTask.ID + "  /  comments"
	if len(m.commentsModel.comments) > 0 {
		title += "  (" + countLabel(len(m.commentsModel.comments)) + ")"
	}
	header := renderHeaderBar("⚡ flow  /  "+title, m.headerRight(), m.width)

	var footer string
	if m.confirm != nil {
		footer = renderFooterBar(renderConfirmFooter(m.confirm.question, m.confirm.destructive), m.width)
	} else {
		switch m.commentsModel.mode {
		case commentModeCompose:
			footer = renderFooterBar("ctrl+s  save   esc  cancel", m.width)
		default:
			commentHints := []string{"esc  back"}
			if _, ok := m.provider.(tasks.CommentAdder); ok {
				commentHints = append(commentHints, "n  new")
			}
			if _, ok := m.provider.(tasks.CommentEditor); ok {
				commentHints = append(commentHints, "e  edit")
			}
			if _, ok := m.provider.(tasks.CommentDeleter); ok {
				commentHints = append(commentHints, "D  delete")
			}
			commentHints = append(commentHints, "?  help")
			footer = renderFooterBar(fitHints(commentHints, "   ", m.width-2), m.width)
		}
	}

	content := m.commentsModel.View()

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}
