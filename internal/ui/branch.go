package ui

import (
	"regexp"
	"strings"

	igit "github.com/ben-fourie/flow-cli/internal/git"
	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Branch creation view ──────────────────────────────────────────────────────

// taskIDPattern matches provider-style task IDs such as PROJ-42 or FLOW-001.
var taskIDPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-[0-9]+`)

// extractTaskID scans a branch name for a task ID (e.g. "feature/PROJ-42-..." → "PROJ-42").
func extractTaskID(branch string) string {
	return taskIDPattern.FindString(strings.ToUpper(branch))
}

// currentBranchCmd detects the checked-out branch and extracts the task ID.
func currentBranchCmd() tea.Cmd {
	return func() tea.Msg {
		branch := igit.CurrentBranch()
		return currentBranchMsg{branch: branch, activeTask: extractTaskID(branch)}
	}
}

// loadTransitionsForAutoCmd fetches transitions with the intent of auto-applying In Progress.
func loadTransitionsForAutoCmd(u tasks.StatusUpdater, taskID string) tea.Cmd {
	return func() tea.Msg {
		ts, err := u.GetTransitions(taskID)
		return autoTransitionMsg{updater: u, taskID: taskID, transitions: ts, err: err}
	}
}

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
	// Handle "checkout existing branch?" prompt.
	if m.confirmingCheckout {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "enter":
				m.confirmingCheckout = false
				name := strings.TrimSpace(m.branchInput.Value())
				if err := igit.CheckoutBranch(name); err != nil {
					m.statusMessage = "✗  " + err.Error()
					m.state = viewDetail
					return m, nil
				}
				m.activeBranch = name
				m.activeTaskID = extractTaskID(name)
				m.statusMessage = "✓  Switched to branch: " + name
				m.state = viewDetail
				return m, nil
			case "n", "esc":
				m.confirmingCheckout = false
				return m, nil
			}
		}
		return m, nil
	}

	// Handle "move to In Progress?" prompt.
	if m.pendingTransitionPrompt {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "y", "enter":
				m.pendingTransitionPrompt = false
				name := strings.TrimSpace(m.branchInput.Value())
				m.statusMessage = "✓  Branch created: " + name
				m.state = viewDetail
				if su, ok := m.provider.(tasks.StatusUpdater); ok && m.selectedTask != nil {
					return m, loadTransitionsForAutoCmd(su, m.selectedTask.ID)
				}
				return m, nil
			case "n", "esc":
				m.pendingTransitionPrompt = false
				name := strings.TrimSpace(m.branchInput.Value())
				m.statusMessage = "✓  Branch created: " + name
				m.state = viewDetail
				return m, nil
			}
		}
		return m, nil
	}

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
		if strings.Contains(err.Error(), "already exists") {
			m.confirmingCheckout = true
			m.statusMessage = ""
			return m, nil
		}
		m.statusMessage = "✗  " + err.Error()
		return m, nil
	}

	m.activeBranch = name
	m.activeTaskID = extractTaskID(name)

	if m.selectedTask != nil {
		if _, canTransition := m.provider.(tasks.StatusUpdater); canTransition &&
			m.selectedTask.Status != tasks.StatusInProgress {
			m.pendingTransitionPrompt = true
			m.statusMessage = ""
			return m, nil
		}
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

	var footerText string
	switch {
	case m.confirmingCheckout:
		footerText = renderBranchPrompt("Branch exists — switch to it?")
	case m.pendingTransitionPrompt:
		footerText = renderBranchPrompt("Move " + m.selectedTask.ID + " to In Progress?")
	default:
		footerText = "enter  confirm   esc  cancel"
	}
	footer := renderFooterBar(footerText, m.width)

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
