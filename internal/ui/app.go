package ui

import (
	"strings"

	"github.com/ben-fourie/flow-cli/internal/config"
	igit "github.com/ben-fourie/flow-cli/internal/git"
	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewState tracks which top-level view is currently active.
type viewState int

const (
	viewLoading  viewState = iota
	viewList     viewState = iota
	viewDetail   viewState = iota
	viewBranch   viewState = iota
	viewEdit     viewState = iota
	viewCreate   viewState = iota
	viewSearch   viewState = iota
	viewStatus   viewState = iota
	viewComments viewState = iota
	viewError    viewState = iota
)

// verticalOverhead is the number of rows consumed by the header, two separator
// lines, and the footer bar that surround the main content area.
const verticalOverhead = 4 // header(1) + separator(1) + separator(1) + footer(1)

// tasksLoadedMsg carries the result of an async task fetch.
type tasksLoadedMsg struct {
	tasks []tasks.Task
	err   error
}

// taskSavedMsg carries the result of an async task update.
type taskSavedMsg struct {
	task tasks.Task
	err  error
}

// taskCreatedMsg carries the result of an async task creation.
type taskCreatedMsg struct {
	task tasks.Task
	err  error
}

// subtasksLoadedMsg carries the result of an async subtask fetch.
type subtasksLoadedMsg struct {
	tasks []tasks.Task
	err   error
}

// transitionsLoadedMsg carries the result of an async transitions fetch.
type transitionsLoadedMsg struct {
	transitions []tasks.StatusTransition
	err         error
}

// taskTransitionedMsg carries the result of an async status transition.
type taskTransitionedMsg struct {
	task tasks.Task
	err  error
}

// searchResultsMsg carries the result of an async task search.
type searchResultsMsg struct {
	tasks []tasks.Task
	err   error
}

// selfAssignedMsg carries the result of an async self-assign operation.
type selfAssignedMsg struct {
	task tasks.Task
	err  error
}

type commentsLoadedMsg struct {
	comments []tasks.Comment
	err      error
}

type commentSavedMsg struct {
	comment tasks.Comment
	err     error
}

type commentDeletedMsg struct {
	commentID string
	err       error
}

// detailFocusArea identifies which region of the detail view has keyboard focus.
type detailFocusArea int

const (
	detailFocusViewport  detailFocusArea = iota
	detailFocusSubtasks  detailFocusArea = iota
)

// ── Root model ────────────────────────────────────────────────────────────────

// Model is the root Bubble Tea model for the flow dashboard.
type Model struct {
	list          list.Model
	detail        viewport.Model
	branchInput   textinput.Model
	editModel     TaskEditModel
	createModel   TaskCreateModel
	spinner       spinner.Model
	cfg           config.Config
	provider      tasks.Provider
	state         viewState
	tasks         []tasks.Task
	selectedTask  *tasks.Task
	subtasks      []tasks.Task    // subtasks for the currently selected task
	subtaskCursor int             // selected row in the subtask mini-list
	detailFocus   detailFocusArea // which region of detail view has focus
	statusModel   TaskStatusModel // picker for workflow transitions
	statusLoading bool            // true while transitions are being fetched
	searchModel       TaskSearchModel
	searchLoading     bool      // true while search is in flight
	searchReturnState viewState // view to return to when search is dismissed
	detailReturnState       viewState   // view to return to when detail is dismissed
	detailReturnTask        *tasks.Task // parent task to restore when returning to viewDetail
	detailParentReturnState viewState   // detailReturnState of the parent task

	commentsModel TaskCommentsModel
	statusMessage string          // transient feedback shown in the footer
	loadErr       string          // shown in viewError
	width         int
	height        int
}

// New constructs the Model. Task loading is deferred to Init() so the UI
// can show a spinner while the network request is in flight.
func New(provider tasks.Provider, cfg config.Config) (Model, error) {
	l := list.New(nil, taskDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.StatusBar             = dimStyle
	l.Styles.FilterPrompt          = dimStyle.Bold(true)
	l.Styles.FilterCursor          = lipgloss.NewStyle().Foreground(colorPrimary)
	l.Styles.ActivePaginationDot   = lipgloss.NewStyle().Foreground(colorPrimary)
	l.Styles.InactivePaginationDot = lipgloss.NewStyle().Foreground(colorBorder)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	return Model{
		list:     l,
		spinner:  sp,
		cfg:      cfg,
		provider: provider,
		state:    viewLoading,
	}, nil
}

// ── tea.Model interface ───────────────────────────────────────────────────────

// Init kicks off the async task load and starts the spinner animation.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadTasksCmd(m.provider))
}

// loadTasksCmd returns a Cmd that fetches tasks in the background.
func loadTasksCmd(p tasks.Provider) tea.Cmd {
	return func() tea.Msg {
		t, err := p.GetTasks()
		return tasksLoadedMsg{tasks: t, err: err}
	}
}


// Update is the single entry point for all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Window resize is handled regardless of active view.
	if resize, ok := msg.(tea.WindowSizeMsg); ok {
		return m.handleResize(resize), nil
	}

	// Handle async task load result.
	if loaded, ok := msg.(tasksLoadedMsg); ok {
		return m.handleTasksLoaded(loaded)
	}

	// Handle async task save result.
	if saved, ok := msg.(taskSavedMsg); ok {
		return m.handleTaskSaved(saved)
	}

	// Handle async task creation result.
	if created, ok := msg.(taskCreatedMsg); ok {
		return m.handleTaskCreated(created)
	}

	// Handle async subtask fetch result.
	if loaded, ok := msg.(subtasksLoadedMsg); ok {
		return m.handleSubtasksLoaded(loaded)
	}

	// Handle async transitions fetch result.
	if loaded, ok := msg.(transitionsLoadedMsg); ok {
		return m.handleTransitionsLoaded(loaded)
	}

	// Handle async task transition result.
	if transitioned, ok := msg.(taskTransitionedMsg); ok {
		return m.handleTaskTransitioned(transitioned)
	}

	// Handle async search result.
	if results, ok := msg.(searchResultsMsg); ok {
		return m.handleSearchResults(results)
	}

	// Handle async self-assign result.
	if assigned, ok := msg.(selfAssignedMsg); ok {
		return m.handleSelfAssigned(assigned)
	}

	// Handle async comment results.
	if loaded, ok := msg.(commentsLoadedMsg); ok {
		return m.handleCommentsLoaded(loaded)
	}
	if saved, ok := msg.(commentSavedMsg); ok {
		return m.handleCommentSaved(saved)
	}
	if deleted, ok := msg.(commentDeletedMsg); ok {
		return m.handleCommentDeleted(deleted)
	}

	// Keep the spinner ticking while loading.
	if m.state == viewLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	switch m.state {
	case viewList:
		return m.updateListView(msg)
	case viewDetail:
		return m.updateDetailView(msg)
	case viewBranch:
		return m.updateBranchView(msg)
	case viewEdit:
		return m.updateEditView(msg)
	case viewCreate:
		return m.updateCreateView(msg)
	case viewSearch:
		return m.updateSearchView(msg)
	case viewStatus:
		return m.updateStatusView(msg)
	case viewComments:
		return m.updateCommentsView(msg)
	case viewError:
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "r":
				m.state = viewLoading
				return m, tea.Batch(m.spinner.Tick, loadTasksCmd(m.provider))
			}
		}
	}
	return m, nil
}

// handleTasksLoaded populates the list when the async fetch completes.
func (m Model) handleTasksLoaded(msg tasksLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.loadErr = msg.err.Error()
		m.state = viewError
		return m, nil
	}

	items := make([]list.Item, len(msg.tasks))
	for i, t := range msg.tasks {
		items[i] = taskItem{task: t}
	}
	m.list.SetItems(items)
	m.tasks = msg.tasks
	m.state = viewList
	return m, nil
}

// View renders the current view to a string for the terminal.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	switch m.state {
	case viewLoading:
		return m.renderLoadingView()
	case viewList:
		return m.renderListView()
	case viewDetail:
		return m.renderDetailView()
	case viewBranch:
		return m.renderBranchView()
	case viewEdit:
		return m.renderEditView()
	case viewCreate:
		return m.renderCreateView()
	case viewSearch:
		return m.renderSearchView()
	case viewStatus:
		return m.renderStatusView()
	case viewComments:
		return m.renderCommentsView()
	case viewError:
		return m.renderErrorView()
	}
	return ""
}

// ── Resize ────────────────────────────────────────────────────────────────────

func (m Model) handleResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	contentHeight := msg.Height - verticalOverhead
	m.list.SetSize(msg.Width, contentHeight)

	if m.state == viewDetail {
		subtaskSectionH := 0
		if len(m.subtasks) > 0 {
			subtaskSectionH = subtaskSectionHeight(len(m.subtasks))
		}
		m.detail.Width = msg.Width
		m.detail.Height = max(3, contentHeight-subtaskSectionH)
	}

	m.editModel.width = msg.Width
	m.editModel.height = msg.Height
	m.editModel.applyWidths()

	m.createModel.width = msg.Width
	m.createModel.height = msg.Height
	m.createModel.applyWidths()

	m.statusModel.width = msg.Width
	m.statusModel.height = msg.Height

	m.searchModel.width = msg.Width
	m.searchModel.height = msg.Height

	return m
}

// ── List view ─────────────────────────────────────────────────────────────────

func (m Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.list.FilterState() != list.Filtering {
			switch key.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				return m.openDetail()
			case "n":
				return m.openCreateView(nil)
			case "f":
				return m.openSearchView()
			case "r":
				m.state = viewLoading
				return m, tea.Batch(m.spinner.Tick, loadTasksCmd(m.provider))
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) openDetail() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(taskItem)
	if !ok {
		return m, nil
	}
	return m.openDetailForTask(item.task, viewList)
}

func (m Model) openDetailForTask(t tasks.Task, returnTo viewState) (tea.Model, tea.Cmd) {
	m.selectedTask = &t
	m.state = viewDetail
	m.detailReturnState = returnTo
	m.subtasks = nil
	m.subtaskCursor = 0
	m.detailFocus = detailFocusViewport

	contentHeight := m.height - verticalOverhead
	m.detail = viewport.New(m.width, contentHeight)
	m.detail.SetContent(renderTaskDetail(t, m.width))

	// Lazily fetch subtasks if the provider supports it.
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

func (m Model) renderLoadingView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow", m.width)
	footer := renderFooterBar("loading…", m.width)

	content := lipgloss.NewStyle().
		Padding(2, 2).
		Foreground(colorText).
		Render(m.spinner.View() + "  Fetching tasks…")

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}

func (m Model) renderErrorView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow", m.width)
	footer := renderFooterBar("r  retry   q  quit", m.width)

	content := lipgloss.NewStyle().Padding(2, 2).Render(
		lipgloss.NewStyle().Foreground(colorPriorityCritical).Bold(true).Render("✗  Failed to load tasks") +
			"\n\n" +
			dimStyle.Render(m.loadErr),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}

func (m Model) renderListView() string {
	header := renderHeaderBar("⚡ flow", m.width)
	sep := renderSeparator(m.width)

	footerText := "↑/↓  navigate   enter  open   /  filter   esc  clear filter   r  refresh   f  find"
	if _, canCreate := m.provider.(tasks.Creator); canCreate {
		footerText += "   n  new"
	}
	footerText += "   q  quit"
	footer := renderFooterBar(footerText, m.width)

	var content string
	if len(m.list.VisibleItems()) == 0 && m.list.FilterState() == list.FilterApplied {
		content = emptyStateStyle.Render("No tasks match your filter.")
	} else {
		content = m.list.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		content,
		sep,
		footer,
	)
}

// ── Detail view ───────────────────────────────────────────────────────────────

func (m Model) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.state = m.detailReturnState
			return m, nil
		case "o":
			if m.selectedTask != nil && m.selectedTask.URL != "" {
				return m, openURL(m.selectedTask.URL)
			}
		case "b":
			return m.openBranchView()
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

	footerText := "esc  back   ↑/↓  scroll   o  open in browser   b  create branch"
	if _, canEdit := m.provider.(tasks.Updater); canEdit {
		footerText += "   e  edit"
	}
	if _, canCreate := m.provider.(tasks.Creator); canCreate {
		footerText += "   n  new subtask"
	}
	if _, canStatus := m.provider.(tasks.StatusUpdater); canStatus {
		footerText += "   s  change status"
	}
	if _, canAssign := m.provider.(tasks.SelfAssigner); canAssign {
		footerText += "   a  assign to me"
	}
	if _, canComment := m.provider.(tasks.CommentLister); canComment {
		footerText += "   c  comments"
	}
	if len(m.subtasks) > 0 {
		footerText += "   tab  focus subtasks"
	}
	footerText += "   q  quit"
	if m.statusMessage != "" {
		footerText = m.statusMessage
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
	for i := 0; i < shown; i++ {
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

// ── Branch creation view ──────────────────────────────────────────────────────

func (m Model) openBranchView() (tea.Model, tea.Cmd) {
	if m.selectedTask == nil {
		return m, nil
	}

	if !igit.IsRepo() {
		m.statusMessage = "✗  Not inside a Git repository"
		return m, nil
	}

	ti := textinput.New()
	ti.SetValue(m.cfg.Branch.Apply(*m.selectedTask))
	ti.CursorEnd()
	ti.Focus()
	ti.Width = m.width - 6
	ti.Prompt = "  "
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.PromptStyle = dimStyle

	m.branchInput = ti
	m.state = viewBranch
	m.statusMessage = ""

	return m, textinput.Blink
}

func (m Model) updateBranchView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.state = viewDetail
			m.statusMessage = ""
			return m, nil
		case "enter":
			return m.confirmBranch()
		}
	}

	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

func (m Model) confirmBranch() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.branchInput.Value())
	if name == "" {
		m.statusMessage = "✗  Branch name cannot be empty"
		return m, nil
	}

	if err := igit.CreateBranch(name); err != nil {
		m.statusMessage = "✗  " + err.Error()
		return m, nil
	}

	m.statusMessage = "✓  Branch created: " + name
	m.state = viewDetail
	return m, nil
}

func (m Model) renderBranchView() string {
	if m.selectedTask == nil {
		return ""
	}

	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  new branch", m.width)
	footer := renderFooterBar("enter  confirm   esc  cancel", m.width)

	label := lipgloss.NewStyle().
		Foreground(colorSubtle).
		Padding(1, 2).
		Render("Branch name:")

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(m.width - 6).
		Render(m.branchInput.View())

	hint := dimStyle.Padding(0, 2).Render("Edit the branch name above, then press enter to create and switch to the branch.")

	content := lipgloss.JoinVertical(lipgloss.Left,
		label,
		lipgloss.NewStyle().Padding(0, 2).Render(inputBox),
		"",
		lipgloss.NewStyle().Padding(0, 2).Render(hint),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		content,
		sep,
		footer,
	)
}

// ── Edit view ─────────────────────────────────────────────────────────────────

func (m Model) openEditView() (tea.Model, tea.Cmd) {
	if m.selectedTask == nil {
		return m, nil
	}
	if _, ok := m.provider.(tasks.Updater); !ok {
		m.statusMessage = "✗  This provider does not support editing"
		return m, nil
	}

	em := NewTaskEditModel(*m.selectedTask)
	em.width = m.width
	em.height = m.height
	em.applyWidths()
	m.editModel = em
	m.state = viewEdit
	m.statusMessage = ""
	return m, textinput.Blink
}

func (m Model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Handle discard confirmation mode first.
		if m.editModel.confirming {
			switch key.String() {
			case "y", "enter":
				m.editModel.confirming = false
				m.state = viewDetail
			case "n", "esc":
				m.editModel.confirming = false
			}
			return m, nil
		}
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.editModel.HasChanges() {
				m.editModel.confirming = true
				return m, nil
			}
			m.state = viewDetail
			return m, nil
		case "ctrl+s":
			if m.editModel.saving {
				return m, nil
			}
			updater, ok := m.provider.(tasks.Updater)
			if !ok {
				return m, nil
			}
			m.editModel = m.editModel.SetSaving(true)
			return m, tea.Batch(
				m.editModel.spinner.Tick,
				saveTaskCmd(updater, m.editModel.EditedTask()),
			)
		}
	}

	var cmd tea.Cmd
	m.editModel, cmd = m.editModel.Update(msg)
	return m, cmd
}

func saveTaskCmd(u tasks.Updater, t tasks.Task) tea.Cmd {
	return func() tea.Msg {
		err := u.Update(t)
		return taskSavedMsg{task: t, err: err}
	}
}

func (m Model) handleTaskSaved(msg taskSavedMsg) (tea.Model, tea.Cmd) {
	m.editModel = m.editModel.SetSaving(false)
	if msg.err != nil {
		m.editModel = m.editModel.SetError(msg.err.Error())
		return m, nil
	}

	// Update the task in the local slice and refresh the list.
	for i := range m.tasks {
		if m.tasks[i].ID == msg.task.ID {
			m.tasks[i] = msg.task
			m.selectedTask = &m.tasks[i]
			break
		}
	}
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = taskItem{task: t}
	}
	m.list.SetItems(items)

	// Refresh the detail view content with updated task.
	m.detail.SetContent(renderTaskDetail(*m.selectedTask, m.width))

	m.statusMessage = "✓  Saved"
	m.state = viewDetail
	return m, nil
}

func (m Model) renderEditView() string {
	if m.selectedTask == nil {
		return ""
	}
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  edit", m.width)
	footer := renderFooterBar("tab  switch field   ctrl+s  save   esc  discard", m.width)
	if m.editModel.confirming {
		footer = renderFooterBar("Discard changes?   y  yes   n / esc  keep editing", m.width)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		m.editModel.View(),
		sep,
		footer,
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
		items[i] = taskItem{task: t}
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
	footer := renderFooterBar("tab  next field   shift+tab  prev field   ◀/▶  priority   space  toggle assign   ctrl+s  save   esc  discard", m.width)
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
		items[i] = taskItem{task: t}
	}
	m.list.SetItems(items)
	m.detail.SetContent(renderTaskDetail(*m.selectedTask, m.width))

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

// ── Search view ───────────────────────────────────────────────────────────────

func (m Model) openSearchView() (tea.Model, tea.Cmd) {
	if _, ok := m.provider.(tasks.Searcher); !ok {
		m.statusMessage = "✗  This provider does not support search"
		return m, nil
	}
	sm := NewTaskSearchModel()
	sm.width = m.width
	sm.height = m.height
	sm.input.Width = m.width - 10
	m.searchModel = sm
	m.searchLoading = false
	m.searchReturnState = m.state
	m.state = viewSearch
	m.statusMessage = ""
	return m, textinput.Blink
}

func searchCmd(s tasks.Searcher, query string) tea.Cmd {
	return func() tea.Msg {
		ts, err := s.Search(query)
		return searchResultsMsg{tasks: ts, err: err}
	}
}

func (m Model) handleSearchResults(msg searchResultsMsg) (tea.Model, tea.Cmd) {
	m.searchLoading = false
	if msg.err != nil {
		return m, nil
	}
	m.searchModel = m.searchModel.SetResults(msg.tasks)
	return m, nil
}

func (m Model) updateSearchView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.searchLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.state = m.searchReturnState
			return m, nil
		case "enter":
			// Open the selected result if one is highlighted; otherwise search.
			if t, ok := m.searchModel.Selected(); ok {
				return m.openDetailForTask(t, viewSearch)
			}
			q := m.searchModel.Query()
			if q == "" {
				return m, nil
			}
			m.searchLoading = true
			searcher := m.provider.(tasks.Searcher)
			return m, tea.Batch(m.spinner.Tick, searchCmd(searcher, q))
		case "up":
			m.searchModel = m.searchModel.MoveUp()
			return m, nil
		case "down":
			m.searchModel = m.searchModel.MoveDown()
			return m, nil
		case "pgdown":
			m.searchModel = m.searchModel.PageDown()
			return m, nil
		case "pgup":
			m.searchModel = m.searchModel.PageUp()
			return m, nil
		}
	}

	// All other keys go to the text input. If the value changes, reset the
	// cursor so the next enter runs a fresh search rather than opening a
	// stale result.
	prev := m.searchModel.Query()
	var cmd tea.Cmd
	m.searchModel.input, cmd = m.searchModel.input.Update(msg)
	if m.searchModel.Query() != prev {
		m.searchModel = m.searchModel.ResetCursor()
	}
	return m, cmd
}

func (m Model) renderSearchView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  find", m.width)

	var footerText string
	if m.searchLoading {
		footerText = "searching…"
	} else {
		footerText = "enter  search/open   ↑/↓  navigate results   pgup/pgdn  page   esc  back"
	}
	footer := renderFooterBar(footerText, m.width)

	var content string
	if m.searchLoading {
		content = lipgloss.NewStyle().Padding(2, 2).Foreground(colorText).
			Render(m.spinner.View() + "  Searching…")
	} else {
		content = m.searchModel.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}

// ── Assign to self ────────────────────────────────────────────────────────────

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

	// Update in local task slice.
	for i := range m.tasks {
		if m.tasks[i].ID == msg.task.ID {
			m.tasks[i] = msg.task
			m.selectedTask = &m.tasks[i]
			break
		}
	}
	items := make([]list.Item, len(m.tasks))
	for i, t := range m.tasks {
		items[i] = taskItem{task: t}
	}
	m.list.SetItems(items)
	m.detail.SetContent(renderTaskDetail(*m.selectedTask, m.width))

	m.statusMessage = "✓  Assigned to you"
	return m, nil
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
				if m.commentsModel.HasComposeChanges() && !m.commentsModel.confirming {
					m.commentsModel.confirming = true
					return m, nil
				}
				if m.commentsModel.confirming {
					m.commentsModel.confirming = false
					return m, nil
				}
				m.commentsModel.mode = commentModeList
				return m, nil
			case "y":
				if m.commentsModel.confirming {
					m.commentsModel.confirming = false
					m.commentsModel.mode = commentModeList
					return m, nil
				}
			case "n":
				if m.commentsModel.confirming {
					m.commentsModel.confirming = false
					return m, nil
				}
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
		case "d":
			if _, ok := m.commentsModel.SelectedComment(); ok {
				m.commentsModel.mode = commentModeDelete
			}
			return m, nil
		}
	}

	// Delete confirmation mode.
	if m.commentsModel.mode == commentModeDelete {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "enter":
				if c, ok := m.commentsModel.SelectedComment(); ok {
					deleter := m.provider.(tasks.CommentDeleter)
					m.commentsModel.mode = commentModeList
					return m, deleteCommentCmd(deleter, m.commentsModel.taskID, c.ID)
				}
			case "n", "esc":
				m.commentsModel.mode = commentModeList
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
	header := renderHeaderBar("⚡ flow  /  "+title, m.width)

	var footer string
	switch m.commentsModel.mode {
	case commentModeCompose:
		if m.commentsModel.confirming {
			footer = renderFooterBar("Discard changes?   y  yes   n / esc  keep editing", m.width)
		} else {
			footer = renderFooterBar("ctrl+s  save   esc  cancel", m.width)
		}
	case commentModeDelete:
		footer = renderFooterBar("Delete this comment?   y  yes   n / esc  cancel", m.width)
	default:
		hints := "esc  back   ↑/↓  navigate"
		if _, ok := m.provider.(tasks.CommentAdder); ok {
			hints += "   n  new comment"
		}
		if _, ok := m.provider.(tasks.CommentEditor); ok {
			hints += "   e  edit"
		}
		if _, ok := m.provider.(tasks.CommentDeleter); ok {
			hints += "   d  delete"
		}
		footer = renderFooterBar(hints, m.width)
	}

	content := m.commentsModel.View()

	// Delete confirmation overlay.
	if m.commentsModel.mode == commentModeDelete {
		if c, ok := m.commentsModel.SelectedComment(); ok {
			prompt := lipgloss.NewStyle().
				Foreground(colorPriorityCritical).Bold(true).Padding(1, 2).
				Render("Delete comment by " + c.Author + "?   y  yes   n / esc  cancel")
			content = prompt + "\n" + content
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}

func renderHeaderBar(title string, width int) string {	return appHeaderStyle.Width(width).Render(title)
}

func renderFooterBar(help string, width int) string {
	return appFooterStyle.Width(width).Render(help)
}

func renderSeparator(width int) string {
	return separatorStyle.Render(strings.Repeat("─", width))
}
