package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── List item ─────────────────────────────────────────────────────────────────

// taskItem wraps a Task to satisfy the list.Item interface.
type taskItem struct {
	task tasks.Task
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

	fmt.Fprintln(w, row1)
	fmt.Fprint(w, row2)
}
