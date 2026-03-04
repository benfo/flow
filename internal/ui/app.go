package ui

import (
	"strings"
	"time"

	"github.com/benfo/flow/internal/config"
	"github.com/benfo/flow/internal/git"
	"github.com/benfo/flow/internal/tasks"
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
	viewTheme    viewState = iota
	viewError    viewState = iota
	viewOnboard  viewState = iota // shown on first run, before any provider is configured
	viewAuthJira viewState = iota // embedded Jira auth wizard
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

type taskDeletedMsg struct {
	taskID   string
	parentID string // non-empty if this was a subtask
	err      error
}

// parentTaskLoadedMsg carries the result of asynchronously fetching a parent task.
type parentTaskLoadedMsg struct {
	task tasks.Task
	err  error
}

// prOpenedMsg carries the result of attempting to open a PR URL in the browser.
type prOpenedMsg struct {
	err error
}

// clearStatusMsg is sent by clearStatusCmd to erase the transient status bar
// message after a short delay, restoring the normal footer shortcuts.
type clearStatusMsg struct{}

// clearStatusCmd schedules a clearStatusMsg to be delivered after 2.5 seconds.
// Call it with tea.Batch whenever you set m.statusMessage to a transient value.
func clearStatusCmd() tea.Cmd {
	return tea.Tick(2500*time.Millisecond, func(_ time.Time) tea.Msg { return clearStatusMsg{} })
}

// themeSavedMsg carries the result of saving the theme to the global config.
type themeSavedMsg struct {
	err error
}

// jiraAuthDoneMsg is sent by the embedded JiraAuthModel when Jira auth completes.
type jiraAuthDoneMsg struct{}

// jiraAuthCancelledMsg is sent by the embedded JiraAuthModel when the user cancels.
type jiraAuthCancelledMsg struct{}

// currentBranchMsg carries the result of the async git branch detection.
type currentBranchMsg struct {
	branch     string // raw branch name, empty if not in a repo
	activeTask string // task ID extracted from branch name, may be empty
}

// gitDirtyMsg carries the result of the async git dirty-state check.
type gitDirtyMsg struct{ dirty bool }

// gitDirtyCmd checks whether the working tree has uncommitted changes.
func gitDirtyCmd() tea.Cmd {
	return func() tea.Msg {
		return gitDirtyMsg{dirty: git.IsDirty()}
	}
}

// detailNavEntry is one frame in the detail navigation back-stack.
type detailNavEntry struct {
	task        tasks.Task
	returnState viewState // detailReturnState active when this task was showing
}

// detailFocusArea identifies which region of the detail view has keyboard focus.
type detailFocusArea int

const (
	detailFocusViewport  detailFocusArea = iota
	detailFocusSubtasks  detailFocusArea = iota
)

// confirmPrompt holds a pending yes/no confirmation.
// Set m.confirm to show a prompt; cleared automatically when answered.
type confirmPrompt struct {
	question    string
	destructive bool // true = render question in red/warning colour
	onConfirm   func(Model) (tea.Model, tea.Cmd)
	onCancel    func(Model) (tea.Model, tea.Cmd)
}

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
	detailReturnState viewState        // view to return to when the detail nav stack is empty
	detailNavStack    []detailNavEntry // LIFO back-stack; supports arbitrary navigation depth

	commentsModel TaskCommentsModel
	confirm       *confirmPrompt // non-nil while waiting for a yes/no answer

	showHelp     bool           // true when the help overlay is visible
	helpViewport viewport.Model // scrollable content for the help overlay

	themePickerCursor int    // selected row in the theme picker
	themePreviousName string // theme name before picker opened (for esc revert)

	activeBranch  string            // currently checked-out git branch name
	activeTaskID  string            // task ID extracted from activeBranch
	gitDirty      bool              // true when working tree has uncommitted changes
	localBranches map[string]string // taskID → local branch name (not active)

	jiraAuthModel   JiraAuthModel                                    // embedded auth wizard (viewAuthJira)
	providerFactory func(config.Config) (tasks.Provider, error)      // rebuilds provider after auth

	statusMessage string // transient feedback shown in the footer
	loadErr       string // shown in viewError
	width         int
	height        int
}

// New constructs the Model. Task loading is deferred to Init() so the UI
// can show a spinner while the network request is in flight.
func New(provider tasks.Provider, cfg config.Config, factory func(config.Config) (tasks.Provider, error)) (Model, error) {
	SetTheme(cfg.Theme)
	initStyles()

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

	initialState := viewLoading
	if isFirstRun(cfg) {
		initialState = viewOnboard
	}

	return Model{
		list:            l,
		spinner:         sp,
		cfg:             cfg,
		provider:        provider,
		state:           initialState,
		providerFactory: factory,
	}, nil
}

// isFirstRun returns true when no real provider has been configured yet.
func isFirstRun(cfg config.Config) bool {
	active := cfg.Providers.Active
	return len(active) == 0 || (len(active) == 1 && active[0] == "mock")
}

// ── tea.Model interface ───────────────────────────────────────────────────────

// Init kicks off the async task load and starts the spinner animation.
func (m Model) Init() tea.Cmd {
	if m.state == viewOnboard {
		// Don't load tasks or start the spinner on first run; wait for user choice.
		return tea.Batch(currentBranchCmd(), scanLocalBranchesCmd(), gitDirtyCmd())
	}
	return tea.Batch(m.spinner.Tick, loadTasksCmd(m.provider), currentBranchCmd(), scanLocalBranchesCmd(), gitDirtyCmd())
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

	// Handle async task delete result.
	if taskDel, ok := msg.(taskDeletedMsg); ok {
		return m.handleTaskDeleted(taskDel)
	}

	// Handle theme save result.
	if ts, ok := msg.(themeSavedMsg); ok {
		return m.handleThemeSaved(ts)
	}

	// Handle clipboard copy result.
	if cc, ok := msg.(clipboardCopyMsg); ok {
		if cc.err != nil {
			m.statusMessage = "⚠ clipboard unavailable"
		} else {
			m.statusMessage = "✓ copied to clipboard"
		}
		return m, clearStatusCmd()
	}

	// Clear the transient status message after the auto-dismiss timer fires.
	if _, ok := msg.(clearStatusMsg); ok {
		m.statusMessage = ""
		return m, nil
	}

	// Handle active confirmation prompt before routing to any view.
	if m.confirm != nil {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "enter":
				fn := m.confirm.onConfirm
				m.confirm = nil
				return fn(m)
			case "n", "esc":
				fn := m.confirm.onCancel
				m.confirm = nil
				return fn(m)
			}
			return m, nil // swallow all other keys while confirming
		}
	}

	// Handle async parent task fetch result.
	if parent, ok := msg.(parentTaskLoadedMsg); ok {
		return m.handleParentLoaded(parent)
	}

	// Handle git branch detection result.
	if branch, ok := msg.(currentBranchMsg); ok {
		m.activeBranch = branch.branch
		m.activeTaskID = branch.activeTask
		return m, nil
	}

	// Handle git dirty-state result.
	if d, ok := msg.(gitDirtyMsg); ok {
		m.gitDirty = d.dirty
		return m, nil
	}

	// Handle local branch scan result.
	if scanned, ok := msg.(localBranchesScannedMsg); ok {
		m.localBranches = scanned.branches
		// Rebuild list items so branch badges reflect the new data.
		if len(m.tasks) > 0 {
			items := make([]list.Item, len(m.tasks))
			for i, t := range m.tasks {
				items[i] = m.makeTaskItem(t)
			}
			m.list.SetItems(items)
		}
		return m, nil
	}

	// Handle auto-transition after branch create (find + apply In Progress transition).
	if auto, ok := msg.(autoTransitionMsg); ok {
		if auto.err != nil {
			m.statusMessage = "✓  Branch created  (could not load transitions)"
			return m, nil
		}
		for _, t := range auto.transitions {
			if t.To == tasks.StatusInProgress {
				return m, transitionTaskCmd(auto.updater, auto.taskID, t.ID)
			}
		}
		// No In Progress transition available — silently skip.
		return m, nil
	}

	// Handle PR open result.
	if pr, ok := msg.(prOpenedMsg); ok {
		if pr.err != nil {
			m.statusMessage = "✗  " + pr.err.Error()
		}
		return m, nil
	}

	// Handle embedded Jira auth completion: reload config and reinitialise provider.
	if _, ok := msg.(jiraAuthDoneMsg); ok {
		repoRoot, _ := git.RepoRoot()
		newCfg, err := config.Load(repoRoot)
		if err != nil {
			newCfg = m.cfg
		}
		m.cfg = newCfg
		if m.providerFactory != nil {
			if p, factoryErr := m.providerFactory(newCfg); factoryErr == nil {
				m.provider = p
			}
		}
		m.state = viewLoading
		return m, tea.Batch(m.spinner.Tick, loadTasksCmd(m.provider))
	}

	// Handle embedded Jira auth cancellation: return to the welcome screen.
	if _, ok := msg.(jiraAuthCancelledMsg); ok {
		m.state = viewOnboard
		return m, nil
	}

	// Toggle help overlay from non-input views.
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "?" {
		switch m.state {
		case viewList, viewDetail, viewStatus:
			return m.openHelpView()
		case viewComments:
			if m.commentsModel.mode == commentModeList {
				return m.openHelpView()
			}
		}
	}

	// Route all messages to the help overlay when it is visible.
	if m.showHelp {
		return m.updateHelpView(msg)
	}

	// Keep the spinner ticking while loading.
	if m.state == viewLoading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Delegate all messages to the embedded auth wizard when it is active.
	if m.state == viewAuthJira {
		newAuth, cmd := m.jiraAuthModel.Update(msg)
		if jam, ok := newAuth.(JiraAuthModel); ok {
			m.jiraAuthModel = jam
		}
		return m, cmd
	}

	switch m.state {
	case viewOnboard:
		return m.updateOnboardView(msg)
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
	case viewTheme:
		return m.updateThemeView(msg)
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

// View renders the current view to a string for the terminal.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	if m.showHelp {
		return m.renderHelpView()
	}

	switch m.state {
	case viewLoading:
		return m.renderLoadingView()
	case viewOnboard:
		return m.renderWelcomeView()
	case viewAuthJira:
		return m.jiraAuthModel.View()
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
	case viewTheme:
		return m.renderThemeView()
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

	m.jiraAuthModel.width = msg.Width
	m.jiraAuthModel.height = msg.Height
	inputWidth := msg.Width - 10
	for i := range m.jiraAuthModel.inputs {
		m.jiraAuthModel.inputs[i].Width = inputWidth
	}
	m.jiraAuthModel.projectsInput.Width = inputWidth

	return m
}

// headerRight builds the right-aligned context string for the header bar:
// provider name (+ first project if configured) and git branch (with dirty marker).
func (m Model) headerRight() string {
	var parts []string

	// Provider chip — skip "mock" (dev/test mode).
	if len(m.cfg.Providers.Active) > 0 {
		p := m.cfg.Providers.Active[0]
		if p != "mock" {
			label := strings.ToUpper(p[:1]) + p[1:]
			if p == "jira" && m.cfg.Providers.Jira != nil && len(m.cfg.Providers.Jira.Projects) > 0 {
				label += " · " + strings.Join(m.cfg.Providers.Jira.Projects, ", ")
			}
			parts = append(parts, label)
		}
	}

	// Git branch + dirty indicator.
	if m.activeBranch != "" {
		b := "⎇  " + m.activeBranch
		if m.gitDirty {
			b += " ✎"
		}
		parts = append(parts, b)
	}

	return strings.Join(parts, "   ")
}



func (m Model) renderLoadingView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow", m.headerRight(), m.width)
	footer := renderFooterBar("loading…", m.width)

	content := lipgloss.NewStyle().
		Padding(2, 2).
		Foreground(colorText).
		Render(m.spinner.View() + "  Fetching tasks…")

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}

func (m Model) renderErrorView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow", m.headerRight(), m.width)
	footer := renderFooterBar("r  retry   q  quit", m.width)

	content := lipgloss.NewStyle().Padding(2, 2).Render(
		lipgloss.NewStyle().Foreground(colorPriorityCritical).Bold(true).Render("✗  Failed to load tasks") +
			"\n\n" +
			dimStyle.Render(m.loadErr),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}

