package ui

import (
	"regexp"
	"strings"

	igit "github.com/benfo/flow/internal/git"
	"github.com/benfo/flow/internal/tasks"
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

// localBranchesScannedMsg carries the result of scanning all local branches.
type localBranchesScannedMsg struct {
	branches map[string]string // taskID → branch name
}

// scanLocalBranchesCmd reads all local branches and maps each to its task ID (if any).
func scanLocalBranchesCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := igit.AllLocalBranches()
		if err != nil || len(out) == 0 {
			return localBranchesScannedMsg{branches: nil}
		}
		m := make(map[string]string, len(out))
		for _, b := range out {
			if id := extractTaskID(b); id != "" {
				m[id] = b
			}
		}
		return localBranchesScannedMsg{branches: m}
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

	// If this task's branch is already checked out, say so and stay in detail.
	if m.branchForTask(m.selectedTask.ID) != "" {
		m.statusMessage = "✓  Already on branch: " + m.activeBranch
		return m, nil
	}

	// If a local branch for this task already exists (but isn't checked out),
	// skip the creation form and go straight to the checkout prompt.
	if found := igit.FindLocalBranch(m.selectedTask.ID); found != "" {
		m.localBranch = found
		m.confirmingCheckout = true

		ti := textinput.New()
		ti.SetValue(found)
		m.branchInput = ti
		m.state = viewBranch
		m.statusMessage = ""
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
	m.localBranch = ""
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
					m.localBranch = ""
					return m, nil
				}
				m.activeBranch = name
				m.activeTaskID = extractTaskID(name)
				m.localBranch = ""
				m.statusMessage = "✓  Switched to branch: " + name
				m.state = viewDetail
				return m, scanLocalBranchesCmd()
			case "n", "esc":
				m.confirmingCheckout = false
				m.localBranch = ""
				m.state = viewDetail
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
			return m, scanLocalBranchesCmd()
		}
	}

	m.statusMessage = "✓  Branch created: " + name
	m.state = viewDetail
	return m, scanLocalBranchesCmd()
}

func (m Model) renderBranchView() string {
	if m.selectedTask == nil {
		return ""
	}

	sep := renderSeparator(m.width)

	var footerText string
	var header string
	var content string

	switch {
	case m.confirmingCheckout:
		header = renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  switch branch", m.width)
		footerText = renderBranchPrompt("Switch to this branch?")

		summary := igit.LastCommit(m.localBranch)
		branchLine := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Padding(1, 2).
			Render("⎇  " + m.localBranch)
		var commitLine string
		if summary.Hash != "" {
			commitLine = dimStyle.Padding(0, 2).Render(
				summary.Hash + "  " + summary.Subject + "  (" + summary.When + ")",
			)
		}
		content = lipgloss.JoinVertical(lipgloss.Left, branchLine, commitLine)

	case m.pendingTransitionPrompt:
		header = renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  new branch", m.width)
		footerText = renderBranchPrompt("Move " + m.selectedTask.ID + " to In Progress?")
		content = lipgloss.NewStyle().Padding(1, 2).Foreground(colorText).
			Render("Branch created: " + lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render(m.branchInput.Value()))

	default:
		header = renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  new branch", m.width)
		footerText = "enter  confirm   esc  cancel"

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

		content = lipgloss.JoinVertical(lipgloss.Left,
			label,
			lipgloss.NewStyle().Padding(0, 2).Render(inputBox),
			"",
			lipgloss.NewStyle().Padding(0, 2).Render(hint),
		)
	}

	footer := renderFooterBar(footerText, m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}
