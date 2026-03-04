package ui

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	igit "github.com/benfo/flow/internal/git"
	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)
// ── Browser helper ────────────────────────────────────────────────────────────

// openURL returns a tea.Cmd that opens the given URL in the system's default
// browser using the platform-appropriate command. The result is fire-and-forget;
// errors are silently ignored because there is no meaningful recovery path in
// a TUI context.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			// Covers Linux and other Unix-like systems.
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}

// clipboardCopyMsg carries the result of a clipboard write.
type clipboardCopyMsg struct{ err error }

// copyToClipboardCmd writes text to the system clipboard.
func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardCopyMsg{err: clipboard.WriteAll(text)}
	}
}

// openPRCmd builds the PR creation URL for the given branch and opens it in
// the system browser. It reads the "origin" remote URL via git, then maps the
// host to the correct GitHub / GitLab / Bitbucket compare path.
func openPRCmd(branch string) tea.Cmd {
	return func() tea.Msg {
		remoteURL, err := igit.RemoteURL("origin")
		if err != nil {
			return prOpenedMsg{err: err}
		}
		prURL, err := igit.PRCreateURL(remoteURL, branch)
		if err != nil {
			return prOpenedMsg{err: err}
		}
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", prURL)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", prURL)
		default:
			cmd = exec.Command("xdg-open", prURL)
		}
		_ = cmd.Start()
		return prOpenedMsg{}
	}
}

// ── Detail renderer ───────────────────────────────────────────────────────────

// renderTaskDetail builds the full string content for the detail viewport.
// width is used to draw horizontal dividers that span the available terminal width.
// activeBranch is the currently checked-out branch (bold + primary colour).
// localBranch is a branch that exists locally but isn't checked out (subtle).
func renderTaskDetail(t tasks.Task, width int, activeBranch, localBranch string) string {
	var sb strings.Builder

	divider := dividerStyle.Render(strings.Repeat("─", max(0, width-4)))

	// Title
	sb.WriteString(detailTitleStyle.Render(t.Title))
	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n\n")

	// Metadata fields
	writeField(&sb, "ID", t.ID)
	writeField(&sb, "Status", renderStatusBadge(t.Status))
	writeField(&sb, "Priority", renderPriorityBadge(t.Priority))
	writeField(&sb, "Project", t.Project)
	writeField(&sb, "Assignee", t.Assignee)

	if t.ParentID != "" {
		writeField(&sb, "Parent", dimStyle.Render(t.ParentID))
	}

	if activeBranch != "" {
		branchVal := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("● " + activeBranch)
		summary := igit.LastCommit(activeBranch)
		if summary.Hash != "" {
			branchVal += dimStyle.Render("   " + summary.Hash + " · " + summary.Subject + " (" + summary.When + ")")
		}
		writeField(&sb, "Branch", branchVal)
	} else if localBranch != "" {
		branchVal := lipgloss.NewStyle().Foreground(colorSubtle).Render("⎇  " + localBranch + "  (not checked out)")
		writeField(&sb, "Branch", branchVal)
	}

	if t.URL != "" {
		writeField(&sb, "URL", dimStyle.Render(t.URL))
	}

	if len(t.Labels) > 0 {
		badges := make([]string, len(t.Labels))
		for i, l := range t.Labels {
			badges[i] = labelBadgeStyle.Render(l)
		}
		writeField(&sb, "Labels", lipgloss.JoinHorizontal(lipgloss.Top, badges...))
	}

	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n\n")

	// Description
	sb.WriteString(detailSectionHeaderStyle.Render("Description"))
	sb.WriteString("\n\n")

	// Wrap description text to fit the available width with a small margin.
	descWidth := max(0, width-6)
	sb.WriteString(lipgloss.NewStyle().Width(descWidth).Padding(0, 1).Foreground(colorText).Render(t.Description))
	sb.WriteString("\n")

	return sb.String()
}

// writeField appends a single label/value pair to the string builder.
func writeField(sb *strings.Builder, label, value string) {
	sb.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Left,
		detailLabelStyle.Render(label),
		detailValueStyle.Render(value),
	))
	sb.WriteString("\n")
}

// max returns the larger of two ints. Replaces the built-in for Go < 1.21.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── Detail view orchestration ─────────────────────────────────────────────────

func (m Model) openDetailForTask(t tasks.Task, returnTo viewState) (tea.Model, tea.Cmd) {
	m.selectedTask = &t
	m.state = viewDetail
	m.detailReturnState = returnTo
	m.subtasks = nil
	m.subtaskCursor = 0
	m.detailFocus = detailFocusViewport
	m.confirmingDelete = false
	m.statusMessage = "" // clear any transient message (e.g. "Loading parent…")

	contentHeight := m.height - verticalOverhead
	m.detail = viewport.New(m.width, contentHeight)
	m.detail.SetContent(renderTaskDetail(t, m.width, m.branchForTask(t.ID), m.localBranchForTask(t.ID)))

	var cmd tea.Cmd
	if fetcher, ok := m.provider.(tasks.SubtaskFetcher); ok {
		cmd = loadSubtasksCmd(fetcher, t.ID)
	}
	return m, cmd
}

// loadSubtasksCmd returns a Cmd that fetches subtasks in the background.
func loadSubtasksCmd(f tasks.SubtaskFetcher, parentID string) tea.Cmd {
	return func() tea.Msg {
		ts, err := f.GetSubtasks(parentID)
		return subtasksLoadedMsg{tasks: ts, err: err}
	}
}

// ── Self-assign (accessible from detail view) ─────────────────────────────────

func assignToSelfCmd(a tasks.SelfAssigner, taskID string) tea.Cmd {
	return func() tea.Msg {
		t, err := a.AssignToSelf(taskID)
		return selfAssignedMsg{task: t, err: err}
	}
}

func (m Model) assignToSelf() (tea.Model, tea.Cmd) {
	if m.selectedTask == nil {
		return m, nil
	}
	assigner, ok := m.provider.(tasks.SelfAssigner)
	if !ok {
		m.statusMessage = "✗  This provider does not support self-assign"
		return m, nil
	}
	m.statusMessage = ""
	return m, assignToSelfCmd(assigner, m.selectedTask.ID)
}

func (m Model) handleSelfAssigned(msg selfAssignedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = "✗  " + msg.err.Error()
		return m, nil
	}

	for i := range m.tasks {
		if m.tasks[i].ID == msg.task.ID {
			m.tasks[i] = msg.task
			m.selectedTask = &m.tasks[i]
			break
		}
	}
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = m.makeTaskItem(t)
	}
	m.list.SetItems(items)
	m.detail.SetContent(renderTaskDetail(*m.selectedTask, m.width, m.branchForTask(m.selectedTask.ID), m.localBranchForTask(m.selectedTask.ID)))

	m.statusMessage = "✓  Assigned to you"
	return m, clearStatusCmd()
}

// ── Detail view ───────────────────────────────────────────────────────────────

func (m Model) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle delete confirmation overlay first.
	if m.confirmingDelete {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "enter":
				m.confirmingDelete = false
				if del, ok := m.provider.(tasks.TaskDeleter); ok && m.selectedTask != nil {
					t := *m.selectedTask
					return m, deleteTaskCmd(del, t)
				}
			case "n", "esc":
				m.confirmingDelete = false
			}
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			if m.detailReturnState == viewDetail && m.detailReturnTask != nil {
				// Returning from a subtask — fully re-open the parent detail view.
				parent := *m.detailReturnTask
				parentReturnState := m.detailParentReturnState
				m.detailReturnTask = nil
				return m.openDetailForTask(parent, parentReturnState)
			}
			m.statusMessage = ""
			m.state = m.detailReturnState
			return m, nil
		case "o":
			if m.selectedTask != nil && m.selectedTask.URL != "" {
				return m, openURL(m.selectedTask.URL)
			}
		case "y":
			if m.selectedTask != nil {
				text := m.selectedTask.URL
				if text == "" {
					text = m.selectedTask.ID
				}
				return m, copyToClipboardCmd(text)
			}
		case "b":
			return m.openBranchView()
		case "P":
			if m.selectedTask != nil {
				if branch := m.branchForTask(m.selectedTask.ID); branch != "" {
					return m, openPRCmd(branch)
				}
				m.statusMessage = "✗  No active branch for this task"
			}
		case "e":
			return m.openEditView()
		case "f":
			return m.openSearchView()
		case "s":
			return m.openStatusView()
		case "a":
			return m.assignToSelf()
		case "c":
			return m.openCommentsView()
		case "n":
			return m.openCreateView(m.selectedTask)
		case "p":
			if m.selectedTask != nil && m.selectedTask.ParentID != "" {
				return m.openParentDetail()
			}
		case "D":
			if _, canDelete := m.provider.(tasks.TaskDeleter); canDelete && m.selectedTask != nil {
				m.confirmingDelete = true
				m.statusMessage = ""
				return m, nil
			}
		case "tab":
			// Toggle focus between viewport and subtask list (only if subtasks exist).
			if len(m.subtasks) > 0 {
				if m.detailFocus == detailFocusViewport {
					m.detailFocus = detailFocusSubtasks
				} else {
					m.detailFocus = detailFocusViewport
				}
			}
			return m, nil
		case "up", "k":
			if m.detailFocus == detailFocusSubtasks {
				if m.subtaskCursor > 0 {
					m.subtaskCursor--
				}
				return m, nil
			}
		case "down", "j":
			if m.detailFocus == detailFocusSubtasks {
				if m.subtaskCursor < len(m.subtasks)-1 {
					m.subtaskCursor++
				}
				return m, nil
			}
		case "enter":
			if m.detailFocus == detailFocusSubtasks && len(m.subtasks) > 0 {
				return m.openSubtaskDetail(m.subtasks[m.subtaskCursor])
			}
		}
	}

	if m.detailFocus == detailFocusViewport {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}
	return m, nil
}

// openSubtaskDetail opens a subtask's detail view (same view, different task).
func (m Model) openSubtaskDetail(t tasks.Task) (tea.Model, tea.Cmd) {
	m.detailReturnTask = m.selectedTask        // remember parent to restore on esc
	m.detailParentReturnState = m.detailReturnState // remember parent's own return state
	m.subtasks = nil
	m.subtaskCursor = 0
	return m.openDetailForTask(t, viewDetail)
}

// openParentDetail navigates from a subtask up to its parent task.
// It looks up the parent in m.tasks first; if not found and the provider
// supports ParentFetcher, it fires an async fetch.
func (m Model) openParentDetail() (tea.Model, tea.Cmd) {
	parentID := m.selectedTask.ParentID
	// Fast path: parent already in the loaded task list.
	for _, t := range m.tasks {
		if t.ID == parentID {
			return m.openSubtaskDetail(t) // reuse same nav-stack pattern
		}
	}
	// Slow path: fetch the parent asynchronously.
	if fetcher, ok := m.provider.(tasks.ParentFetcher); ok {
		m.statusMessage = "Loading parent…"
		return m, loadParentTaskCmd(fetcher, parentID)
	}
	m.statusMessage = "Parent not available in current view"
	return m, nil
}

// loadParentTaskCmd fetches a single task by ID asynchronously.
func loadParentTaskCmd(f tasks.ParentFetcher, taskID string) tea.Cmd {
	return func() tea.Msg {
		t, err := f.GetTask(taskID)
		return parentTaskLoadedMsg{task: t, err: err}
	}
}

// handleParentLoaded handles the async result of fetching a parent task.
func (m Model) handleParentLoaded(msg parentTaskLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = "Could not load parent: " + msg.err.Error()
		return m, nil
	}
	return m.openSubtaskDetail(msg.task)
}

// handleSubtasksLoaded stores fetched subtasks and adjusts viewport height.
func (m Model) handleSubtasksLoaded(msg subtasksLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil || len(msg.tasks) == 0 {
		return m, nil
	}
	m.subtasks = msg.tasks
	m.subtaskCursor = 0
	// Shrink viewport to make room for the subtask section.
	subtaskSectionH := subtaskSectionHeight(len(m.subtasks))
	contentH := m.height - verticalOverhead - subtaskSectionH
	m.detail.Height = max(3, contentH)
	return m, nil
}

// subtaskSectionHeight returns the number of terminal rows the subtask section
// occupies: 1 header line + 1 line per subtask, capped at 8 subtasks shown.
func subtaskSectionHeight(n int) int {
	shown := min(n, 8)
	return shown + 2 // header row + blank separator
}

func (m Model) renderDetailView() string {
	id := ""
	if m.selectedTask != nil {
		id = m.selectedTask.ID
	}
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  "+id, m.width)

	// Show the 4 most relevant actions for this task + help.
	// Navigation (↑/↓, esc) and less-used keys live in the ? overlay.
	var hints []string
	if _, canEdit := m.provider.(tasks.Updater); canEdit {
		hints = append(hints, "e  edit")
	}
	if _, canStatus := m.provider.(tasks.StatusUpdater); canStatus {
		hints = append(hints, "s  status")
	}
	if _, canComment := m.provider.(tasks.CommentLister); canComment {
		hints = append(hints, "c  comments")
	}
	if _, canCreate := m.provider.(tasks.Creator); canCreate {
		hints = append(hints, "n  subtask")
	}
	hints = append(hints, "y  copy")
	if m.selectedTask != nil {
		active := m.branchForTask(m.selectedTask.ID)
		local := m.localBranchForTask(m.selectedTask.ID)
		switch {
		case active != "":
			// Already on branch — omit b hint, show P instead.
			hints = append(hints, "P  open PR")
		case local != "":
			hints = append(hints, "b  switch branch")
		default:
			hints = append(hints, "b  new branch")
		}
	}
	hints = append(hints, "?  help")

	var footerText string
	if m.confirmingDelete && m.selectedTask != nil {
		footerText = renderDeleteConfirm(m.selectedTask.ID)
	} else if m.statusMessage != "" {
		footerText = m.statusMessage
	} else {
		footerText = fitHints(hints, "   ", m.width-2)
	}
	footer := renderFooterBar(footerText, m.width)

	parts := []string{header, sep, m.detail.View()}

	// Render the subtask mini-list when subtasks are available.
	if len(m.subtasks) > 0 {
		parts = append(parts, m.renderSubtaskSection())
	}

	parts = append(parts, sep, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderSubtaskSection() string {
	focused := m.detailFocus == detailFocusSubtasks

	headerLabel := dimStyle.Render("── Subtasks")
	if focused {
		headerLabel = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("── Subtasks")
	}
	header := lipgloss.NewStyle().Padding(0, 1).Render(headerLabel)

	var rows []string
	shown := min(len(m.subtasks), 8)
	// Scroll the window so the cursor is always visible.
	start := 0
	if m.subtaskCursor >= shown {
		start = m.subtaskCursor - shown + 1
	}
	for i := start; i < start+shown; i++ {
		t := m.subtasks[i]
		cursor := "  "
		style := dimStyle
		if focused && i == m.subtaskCursor {
			cursor = lipgloss.NewStyle().Foreground(colorPrimary).Render("▶ ")
			style = lipgloss.NewStyle().Foreground(colorText).Bold(true)
		}
		id := lipgloss.NewStyle().Foreground(colorPrimary).Render(t.ID)
		title := style.Render(t.Title)
		status := lipgloss.NewStyle().Foreground(statusColor(t.Status)).Render(t.Status.String())
		row := cursor + id + "  " + title + "  " + dimStyle.Render(status)
		rows = append(rows, lipgloss.NewStyle().Padding(0, 1).Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...)
}


