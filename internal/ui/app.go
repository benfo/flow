package ui

import (
	"fmt"
	"strings"

	igit "github.com/ben-fourie/flow-cli/internal/git"
	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewState tracks which top-level view is currently active.
type viewState int

const (
	viewList   viewState = iota
	viewDetail viewState = iota
	viewBranch viewState = iota
)

// verticalOverhead is the number of rows consumed by the header, two separator
// lines, and the footer bar that surround the main content area.
const verticalOverhead = 4 // header(1) + separator(1) + separator(1) + footer(1)

// ── Root model ────────────────────────────────────────────────────────────────

// Model is the root Bubble Tea model for the flow dashboard.
// It owns the list and detail sub-models and handles view-switching.
type Model struct {
	list          list.Model
	detail        viewport.Model
	branchInput   textinput.Model
	state         viewState
	tasks         []tasks.Task
	selectedTask  *tasks.Task
	statusMessage string // transient feedback shown in the footer
	width         int
	height        int
}

// New constructs the Model by fetching tasks from the given provider.
// Returns an error if the provider fails so the caller can surface it cleanly.
func New(provider tasks.Provider) (Model, error) {
	taskList, err := provider.GetTasks()
	if err != nil {
		return Model{}, fmt.Errorf("loading tasks from %s provider: %w", provider.Name(), err)
	}

	items := make([]list.Item, len(taskList))
	for i, t := range taskList {
		items[i] = taskItem{task: t}
	}

	l := list.New(items, taskDelegate{}, 0, 0)
	l.SetShowTitle(false)        // we render our own header
	l.SetShowHelp(false)         // we render our own footer
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.StatusBar        = dimStyle
	l.Styles.FilterPrompt     = dimStyle.Bold(true)
	l.Styles.FilterCursor     = lipgloss.NewStyle().Foreground(colorPrimary)
	l.Styles.ActivePaginationDot   = lipgloss.NewStyle().Foreground(colorPrimary)
	l.Styles.InactivePaginationDot = lipgloss.NewStyle().Foreground(colorBorder)

	return Model{
		list:  l,
		tasks: taskList,
		state: viewList,
	}, nil
}

// ── tea.Model interface ───────────────────────────────────────────────────────

// Init satisfies tea.Model. No IO commands are needed at startup.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update is the single entry point for all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Window resize is handled regardless of active view.
	if resize, ok := msg.(tea.WindowSizeMsg); ok {
		return m.handleResize(resize), nil
	}

	switch m.state {
	case viewList:
		return m.updateListView(msg)
	case viewDetail:
		return m.updateDetailView(msg)
	case viewBranch:
		return m.updateBranchView(msg)
	}
	return m, nil
}

// View renders the current view to a string for the terminal.
func (m Model) View() string {
	// Show a blank frame until the terminal size is known.
	if m.width == 0 {
		return ""
	}

	switch m.state {
	case viewList:
		return m.renderListView()
	case viewDetail:
		return m.renderDetailView()
	case viewBranch:
		return m.renderBranchView()
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
		m.detail.Width = msg.Width
		m.detail.Height = contentHeight
	}

	return m
}

// ── List view ─────────────────────────────────────────────────────────────────

func (m Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Only intercept keys when the list is not in filter-input mode,
		// so that 'q' and 'enter' are available to the filter prompt.
		if m.list.FilterState() != list.Filtering {
			switch key.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				return m.openDetail()
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

	t := item.task
	m.selectedTask = &t
	m.state = viewDetail
	m.detail = viewport.New(m.width, m.height-verticalOverhead)
	m.detail.SetContent(renderTaskDetail(t, m.width))

	return m, nil
}

func (m Model) renderListView() string {
	header := renderHeaderBar("⚡ flow", m.width)
	sep := renderSeparator(m.width)
	footer := renderFooterBar("↑/↓  navigate   enter  open   /  filter   esc  clear filter   q  quit", m.width)

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
			m.state = viewList
			return m, nil
		case "o":
			if m.selectedTask != nil && m.selectedTask.URL != "" {
				return m, openURL(m.selectedTask.URL)
			}
		case "b":
			return m.openBranchView()
		}
	}

	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m Model) renderDetailView() string {
	id := ""
	if m.selectedTask != nil {
		id = m.selectedTask.ID
	}
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  "+id, m.width)

	footerText := "esc  back   ↑/↓  scroll   o  open in browser   b  create branch   q  quit"
	if m.statusMessage != "" {
		footerText = m.statusMessage
	}
	footer := renderFooterBar(footerText, m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		m.detail.View(),
		sep,
		footer,
	)
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
	ti.SetValue(igit.GenerateBranchName(*m.selectedTask))
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

// ── Shared layout helpers ─────────────────────────────────────────────────────

func renderHeaderBar(title string, width int) string {
	return appHeaderStyle.Width(width).Render(title)
}

func renderFooterBar(help string, width int) string {
	return appFooterStyle.Width(width).Render(help)
}

func renderSeparator(width int) string {
	return separatorStyle.Render(strings.Repeat("─", width))
}
