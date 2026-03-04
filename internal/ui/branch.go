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

	// When triggered from the detail view, return there after the branch op
	// so the status message is visible. openBranchViewFromList overrides this.
	if m.state == viewDetail {
		m.detailReturnState = viewDetail
	}

	if !igit.IsRepo() {
		m.statusMessage = "✗  Not inside a Git repository"
		return m, clearStatusCmd()
	}

	// If this task's branch is already checked out, say so and stay put.
	if m.branchForTask(m.selectedTask.ID) != "" {
		m.statusMessage = "✓  Already on branch: " + m.activeBranch
		return m, clearStatusCmd()
	}

	// If a local branch for this task already exists (but isn't checked out),
	// offer to check it out via the confirm prompt without entering viewBranch.
	if found := igit.FindLocalBranch(m.selectedTask.ID); found != "" {
		m.confirm = m.makeCheckoutConfirmPrompt(found)
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
	m.state = viewBranch
	m.statusMessage = ""

	return m, textinput.Blink
}

// openBranchViewFromList opens the branch view for the currently selected list
// item without first navigating to the detail view.
func (m Model) openBranchViewFromList() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(taskItem)
	if !ok {
		return m, nil
	}
	// Temporarily set selectedTask so openBranchView can use it.
	// detailReturnState will send us back to the list, not detail.
	m.selectedTask = &item.task
	m.detailReturnState = viewList
	return m.openBranchView()
}

// makeCheckoutConfirmPrompt builds the confirmPrompt chain for checking out
// an existing local branch, handling the dirty-worktree stash sub-prompt.
func (m Model) makeCheckoutConfirmPrompt(name string) *confirmPrompt {
	return &confirmPrompt{
		question: "Switch to existing branch: " + name + "?",
		onConfirm: func(m Model) (tea.Model, tea.Cmd) {
			if igit.IsDirty() {
				m.confirm = &confirmPrompt{
					question: "Uncommitted changes — stash and switch?",
					onConfirm: func(m Model) (tea.Model, tea.Cmd) {
						if err := igit.StashAndCheckout(name); err != nil {
							m.statusMessage = "✗  " + err.Error()
							return m, clearStatusCmd()
						}
						m.activeBranch = name
						m.activeTaskID = extractTaskID(name)
						m.statusMessage = "✓  Stashed, switched to: " + name
						return m, tea.Batch(currentBranchCmd(), scanLocalBranchesCmd(), gitDirtyCmd(), clearStatusCmd())
					},
					onCancel: func(m Model) (tea.Model, tea.Cmd) { return m, nil },
				}
				return m, nil
			}
			if err := igit.CheckoutBranch(name); err != nil {
				m.statusMessage = "✗  " + err.Error()
				return m, clearStatusCmd()
			}
			m.activeBranch = name
			m.activeTaskID = extractTaskID(name)
			m.statusMessage = "✓  Switched to branch: " + name
			return m, tea.Batch(currentBranchCmd(), scanLocalBranchesCmd(), gitDirtyCmd(), clearStatusCmd())
		},
		onCancel: func(m Model) (tea.Model, tea.Cmd) { return m, nil },
	}
}

func (m Model) updateBranchView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.state = m.detailReturnState
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
			m.confirm = m.makeCheckoutConfirmPrompt(name)
			m.state = m.detailReturnState
			m.statusMessage = ""
			return m, nil
		}
		m.statusMessage = "✗  " + err.Error()
		return m, nil
	}

	m.activeBranch = name
	m.activeTaskID = extractTaskID(name)

	if m.selectedTask != nil {
		if su, canTransition := m.provider.(tasks.StatusUpdater); canTransition &&
			m.selectedTask.Status != tasks.StatusInProgress {
			taskID := m.selectedTask.ID
			m.confirm = &confirmPrompt{
				question: "Move " + taskID + " to In Progress?",
				onConfirm: func(m Model) (tea.Model, tea.Cmd) {
					m.statusMessage = "✓  Branch created: " + name
					return m, tea.Batch(loadTransitionsForAutoCmd(su, taskID), clearStatusCmd())
				},
				onCancel: func(m Model) (tea.Model, tea.Cmd) {
					m.statusMessage = "✓  Branch created: " + name
					return m, clearStatusCmd()
				},
			}
			m.state = m.detailReturnState
			return m, tea.Batch(currentBranchCmd(), scanLocalBranchesCmd(), gitDirtyCmd())
		}
	}

	m.statusMessage = "✓  Branch created: " + name
	m.state = m.detailReturnState
	return m, tea.Batch(currentBranchCmd(), scanLocalBranchesCmd(), gitDirtyCmd(), clearStatusCmd())
}

func (m Model) renderBranchView() string {
	if m.selectedTask == nil {
		return ""
	}

	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  "+m.selectedTask.ID+"  /  new branch", m.headerRight(), m.width)

	var footerText string
	if m.confirm != nil {
		footerText = renderConfirmFooter(m.confirm.question, m.confirm.destructive)
	} else {
		footerText = "enter  confirm   esc  cancel"
	}

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

	footer := renderFooterBar(footerText, m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, content, sep, footer)
}
