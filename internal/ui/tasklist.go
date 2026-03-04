package ui

import (
	"fmt"
	"io"
	"sort"
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
	activeBranch string // set when this task's branch is currently checked out
	localBranch  string // set when a local branch exists but is not checked out
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
	statusBadge := renderStatusBadge(t.task.Status, t.task.ProviderStatus)
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
		// Active branch — pill with primary background for high visibility.
		branchLabel := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSurface).
			Background(colorPrimary).
			Padding(0, 1).
			Render("⎇  " + t.activeBranch)
		row2 += "   " + branchLabel
	} else if t.localBranch != "" {
		// Local branch exists but not checked out — foreground only, subtle.
		branchLabel := lipgloss.NewStyle().
			Foreground(colorPrimary).
			Render("⎇  " + t.localBranch)
		row2 += "   " + branchLabel
	}

	fmt.Fprintln(w, row1)
	fmt.Fprint(w, row2)
}

// ── List view orchestration ───────────────────────────────────────────────────

// makeTaskItem builds a taskItem, populating activeBranch or localBranch.
func (m Model) makeTaskItem(t tasks.Task) taskItem {
	active := m.branchForTask(t.ID)
	local := ""
	if active == "" {
		local = m.localBranches[t.ID]
	}
	return taskItem{task: t, activeBranch: active, localBranch: local}
}

// branchForTask returns activeBranch if it matches taskID, otherwise "".
func (m Model) branchForTask(taskID string) string {
	if m.activeTaskID != "" && taskID == m.activeTaskID {
		return m.activeBranch
	}
	return ""
}

// localBranchForTask returns a local (non-active) branch for taskID, or "".
func (m Model) localBranchForTask(taskID string) string {
	if m.branchForTask(taskID) != "" {
		return "" // already active; don't show as "not checked out"
	}
	return m.localBranches[taskID]
}

// applyFilterSort applies m.statusFilter and m.sort to m.tasks and
// updates the list items. Call it whenever either setting changes.
func (m *Model) applyFilterSort() {
	filtered := make([]tasks.Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		if m.statusFilter == noStatusFilter || t.Status == m.statusFilter {
			filtered = append(filtered, t)
		}
	}

	// Sort the filtered slice.
	sort.SliceStable(filtered, func(i, j int) bool {
		switch m.sort {
		case sortPriority:
			return filtered[i].Priority > filtered[j].Priority
		case sortStatus:
			return filtered[i].Status < filtered[j].Status
		case sortUpdated:
			return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
		default: // sortProvider — preserve original order (already stable)
			return false
		}
	})

	items := make([]list.Item, len(filtered))
	for i, t := range filtered {
		items[i] = m.makeTaskItem(t)
	}
	m.list.SetItems(items)
}

// sortLabel returns a short display string for the active sort order.
func (m Model) sortLabel() string {
	switch m.sort {
	case sortPriority:
		return "priority ↓"
	case sortStatus:
		return "status"
	case sortUpdated:
		return "updated ↓"
	default:
		return ""
	}
}

// filterLabel returns a short display string for the active status filter.
func (m Model) filterLabel() string {
	if m.statusFilter == noStatusFilter {
		return ""
	}
	label := m.statusFilter.String()
	if item, ok := m.list.SelectedItem().(taskItem); ok {
		if item.task.Status == m.statusFilter && item.task.ProviderStatus != "" {
			label = item.task.ProviderStatus
		}
	}
	return label
}

// handleTasksLoaded populates the list when the async fetch completes.
func (m Model) handleTasksLoaded(msg tasksLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.loadErr = msg.err.Error()
		m.state = viewError
		return m, nil
	}
	m.tasks = msg.tasks
	m.applyFilterSort()
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
			case "b":
				return m.openBranchViewFromList()
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
			// ── Status filter: 1-4 = specific status, 0 = all ────────────
			case "0":
				m.statusFilter = noStatusFilter
				m.applyFilterSort()
				return m, nil
			case "1":
				m.statusFilter = tasks.StatusTodo
				m.applyFilterSort()
				return m, nil
			case "2":
				m.statusFilter = tasks.StatusInProgress
				m.applyFilterSort()
				return m, nil
			case "3":
				m.statusFilter = tasks.StatusInReview
				m.applyFilterSort()
				return m, nil
			case "4":
				m.statusFilter = tasks.StatusDone
				m.applyFilterSort()
				return m, nil
			// ── Tab cycles through status filters ─────────────────────────
			case "tab":
				switch m.statusFilter {
				case noStatusFilter:
					m.statusFilter = tasks.StatusTodo
				case tasks.StatusTodo:
					m.statusFilter = tasks.StatusInProgress
				case tasks.StatusInProgress:
					m.statusFilter = tasks.StatusInReview
				case tasks.StatusInReview:
					m.statusFilter = tasks.StatusDone
				default:
					m.statusFilter = noStatusFilter
				}
				m.applyFilterSort()
				return m, nil
			// ── S cycles sort orders ──────────────────────────────────────
			case "S":
				m.sort = (m.sort + 1) % 4
				m.applyFilterSort()
				return m, nil
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
	m.detailNavStack = nil // fresh navigation — clear any prior stack
	return m.openDetailForTask(item.task, viewList)
}

func (m Model) renderListView() string {
	header := renderHeaderBar("⚡ flow", m.headerRight(), m.width)
	sep := renderSeparator(m.width)

	// Primary hints: the 4 most contextual actions + help. Navigation keys
	// (↑/↓, /, esc) are omitted — users learn them quickly.
	hints := []string{"enter  open", "b  branch", "y  copy", "f  find"}
	if _, canCreate := m.provider.(tasks.Creator); canCreate {
		hints = append(hints, "n  new")
	}
	hints = append(hints, "1-4  filter", "S  sort", "?  help")

	// Append active filter/sort chips so the user knows the current state.
	if fl := m.filterLabel(); fl != "" {
		hints = append(hints, "["+fl+"]")
	}
	if sl := m.sortLabel(); sl != "" {
		hints = append(hints, "↕ "+sl)
	}

	var footerContent string
	if m.confirm != nil {
		footerContent = renderConfirmFooter(m.confirm.question, m.confirm.destructive)
	} else if m.statusMessage != "" {
		footerContent = m.statusMessage
	} else {
		footerContent = fitHints(hints, "   ", m.width-2)
	}
	footer := renderFooterBar(footerContent, m.width)

	var content string
	if len(m.list.VisibleItems()) == 0 {
		var msg string
		if m.statusFilter != noStatusFilter {
			msg = "No " + m.statusFilter.String() + " tasks."
		} else if m.list.FilterState() == list.FilterApplied {
			msg = "No tasks match your filter."
		} else {
			msg = "No tasks."
		}
		// Use the same height the list occupies so the footer stays pinned.
		content = lipgloss.NewStyle().
			Width(m.width).
			Height(m.height - verticalOverhead).
			Render(emptyStateStyle.Render(msg))
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
