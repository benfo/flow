package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Help overlay ──────────────────────────────────────────────────────────────

type helpBinding struct {
	key  string
	desc string
}

type helpSection struct {
	title    string
	bindings []helpBinding
}

var helpSections = []helpSection{
	{
		title: "Global",
		bindings: []helpBinding{
			{"?", "toggle this help screen"},
			{"q / ctrl+c", "quit"},
		},
	},
	{
		title: "Task List",
		bindings: []helpBinding{
			{"↑ / ↓ / j / k", "navigate tasks"},
			{"enter", "open task detail"},
			{"f", "search tasks"},
			{"b", "git branch view"},
			{"T", "open theme picker"},
		},
	},
	{
		title: "Task Detail",
		bindings: []helpBinding{
			{"esc / backspace", "go back"},
			{"↑ / ↓", "scroll content"},
			{"tab", "toggle subtask list focus"},
			{"enter", "open selected subtask"},
			{"p", "navigate to parent task"},
			{"o", "open task URL in browser"},
			{"b", "create or checkout git branch"},
			{"e", "edit task"},
			{"n", "create subtask"},
			{"s", "change status"},
			{"a", "assign to self"},
			{"c", "view / manage comments"},
			{"D", "delete task (prompts confirmation)"},
		},
	},
	{
		title: "Edit / Create",
		bindings: []helpBinding{
			{"tab / shift+tab", "next / previous field"},
			{"ctrl+s", "save"},
			{"esc", "cancel (prompts if there are unsaved changes)"},
		},
	},
	{
		title: "Status Picker",
		bindings: []helpBinding{
			{"↑ / ↓", "navigate transitions"},
			{"enter", "apply selected status"},
			{"esc", "cancel"},
		},
	},
	{
		title: "Comments",
		bindings: []helpBinding{
			{"↑ / ↓", "navigate comments"},
			{"a", "add a new comment"},
			{"e", "edit selected comment"},
			{"D", "delete selected comment"},
			{"esc", "go back"},
		},
	},
	{
		title: "Comment Compose",
		bindings: []helpBinding{
			{"ctrl+s", "save comment"},
			{"esc", "cancel (prompts if there are unsaved changes)"},
		},
	},
	{
		title: "Search",
		bindings: []helpBinding{
			{"type", "filter tasks by text"},
			{"enter", "open selected result"},
			{"esc", "go back"},
		},
	},
	{
		title: "Branch",
		bindings: []helpBinding{
			{"type", "enter branch name"},
			{"enter", "create or checkout branch"},
			{"esc", "cancel"},
		},
	},
}

// buildHelpContent renders the full help text for the viewport.
func buildHelpContent(width int) string {
	keyW := 24 // column width for the key
	innerW := max(0, width-4)

	keyStyle := lipgloss.NewStyle().Width(keyW).Foreground(colorPrimary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorText)
	sectionStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Padding(0, 1)
	divStyle := dimStyle

	var sb strings.Builder

	for _, sec := range helpSections {
		// Section header with divider line.
		divLen := max(0, innerW-lipgloss.Width(sec.title)-3)
		div := divStyle.Render("── " + sec.title + " " + strings.Repeat("─", divLen))
		sb.WriteString(sectionStyle.Render(div))
		sb.WriteString("\n")

		for _, b := range sec.bindings {
			row := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render(b.key),
				descStyle.Render(b.desc),
			)
			sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(row))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// openHelpView activates the help overlay and initialises its viewport.
func (m Model) openHelpView() (tea.Model, tea.Cmd) {
	contentH := m.height - verticalOverhead
	vp := viewport.New(m.width, max(3, contentH))
	vp.SetContent(buildHelpContent(m.width))
	m.helpViewport = vp
	m.showHelp = true
	return m, nil
}

// updateHelpView handles keyboard input while the help overlay is visible.
func (m Model) updateHelpView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "?":
			m.showHelp = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.helpViewport, cmd = m.helpViewport.Update(msg)
	return m, cmd
}

// renderHelpView renders the full-screen help overlay.
func (m Model) renderHelpView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  keyboard shortcuts", m.width)

	scrollPct := int(m.helpViewport.ScrollPercent() * 100)
	footerHint := fitHints(
		[]string{"↑/↓  scroll", "esc / ?  close"},
		"   ",
		m.width-12,
	)
	footer := renderFooterBar(
		lipgloss.JoinHorizontal(lipgloss.Left,
			lipgloss.NewStyle().Width(m.width-10).Render(footerHint),
			dimStyle.Render(lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Render(
				fmt.Sprintf("%d%%", scrollPct),
			)),
		),
		m.width,
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header, sep, m.helpViewport.View(), sep, footer,
	)
}
