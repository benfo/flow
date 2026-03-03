package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── List item ─────────────────────────────────────────────────────────────────

// taskItem wraps a Task to satisfy the list.Item interface.
type taskItem struct {
	task         tasks.Task
	activeBranch string // set when this task's ID matches the checked-out branch
}

// FilterValue is used by the Bubbles list's built-in fuzzy filter.
// We include the ID so users can search by ticket number as well as title.
func (t taskItem) FilterValue() string {
	return t.task.ID + " " + t.task.Title
}

// ── Custom delegate ───────────────────────────────────────────────────────────

// taskDelegate renders each task list item as a two-line row:
//
//	Line 1:  [selector] ID   Title
//	Line 2:             [Status badge]  [Priority badge]
type taskDelegate struct{}

func (d taskDelegate) Height() int                              { return 2 }
func (d taskDelegate) Spacing() int                            { return 1 }
func (d taskDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d taskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	t, ok := item.(taskItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Build the selector glyph and apply title styling.
	var selector, titleStr string
	if isSelected {
		selector = selectedItemStyle.Render("▶")
		titleStr = selectedItemStyle.Render(t.task.Title)
	} else {
		selector = "  "
		titleStr = normalItemStyle.Render(t.task.Title)
	}

	idStr := dimStyle.Render(t.task.ID)
	statusBadge := renderStatusBadge(t.task.Status)
	priorityBadge := renderPriorityBadge(t.task.Priority)

	// selector(2) + space(1) + ID + space(2) = fixed prefix; badges indent to match.
	prefixWidth := 2 + 1 + lipgloss.Width(idStr) + 2
	indent := strings.Repeat(" ", prefixWidth)

	row1 := lipgloss.JoinHorizontal(lipgloss.Left,
		selector+" ",
		idStr+"  ",
		titleStr,
	)

	row2 := indent + statusBadge + "   " + priorityBadge
	if t.activeBranch != "" {
		branchLabel := lipgloss.NewStyle().Foreground(colorSubtle).Render("⎇  " + t.activeBranch)
		row2 += "   " + branchLabel
	}

	fmt.Fprintln(w, row1)
	fmt.Fprint(w, row2)
}

// ── List view orchestration ───────────────────────────────────────────────────

// makeTaskItem builds a taskItem, populating activeBranch when the task ID
// matches the currently checked-out branch.
func (m Model) makeTaskItem(t tasks.Task) taskItem {
	return taskItem{task: t, activeBranch: m.branchForTask(t.ID)}
}

// branchForTask returns activeBranch if it matches taskID, otherwise "".
func (m Model) branchForTask(taskID string) string {
	if m.activeTaskID != "" && taskID == m.activeTaskID {
		return m.activeBranch
	}
	return ""
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
		items[i] = m.makeTaskItem(t)
	}
	m.list.SetItems(items)
	m.tasks = msg.tasks
	m.state = viewList
	return m, nil
}

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
			case "T":
				return m.openThemeView()
			case "y":
				if item, ok := m.list.SelectedItem().(taskItem); ok {
					text := item.task.URL
					if text == "" {
						text = item.task.ID
					}
					return m, copyToClipboardCmd(text)
				}
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

func (m Model) renderListView() string {
	header := renderHeaderBar("⚡ flow", m.width)
	sep := renderSeparator(m.width)

	hints := []string{"↑/↓  navigate", "enter  open", "y  copy", "/  filter", "esc  clear filter", "r  refresh", "f  find"}
	if _, canCreate := m.provider.(tasks.Creator); canCreate {
		hints = append(hints, "n  new")
	}
	hints = append(hints, "T  theme", "?  help", "q  quit")
	footer := renderFooterBar(fitHints(hints, "   ", m.width-2), m.width)

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
