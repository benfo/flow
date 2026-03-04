package ui

import (
	igit "github.com/benfo/flow/internal/git"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderWelcomeView renders the first-run onboarding screen.
func (m Model) renderWelcomeView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow", "", m.width)
	footer := renderFooterBar("j  set up Jira   d  try demo data   q  quit", m.width)

	title := lipgloss.NewStyle().
		Foreground(colorPrimary).Bold(true).Padding(2, 2).
		Render("Welcome to flow")

	subtitle := dimStyle.Padding(0, 2).
		Render("A keyboard-driven terminal dashboard for your tasks.")

	opt1 := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Padding(0, 1, 0, 2).Render("j"),
		lipgloss.NewStyle().Foreground(colorText).Render("  Set up Jira  "),
		dimStyle.Render("—  connect your Jira account and start working"),
	)

	opt2 := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Padding(0, 1, 0, 2).Render("d"),
		lipgloss.NewStyle().Foreground(colorText).Render("  Try demo data"),
		dimStyle.Render("—  explore the interface with sample tasks"),
	)

	options := lipgloss.NewStyle().Padding(1, 0).Render(
		lipgloss.JoinVertical(lipgloss.Left, opt1, "", opt2),
	)

	hint := dimStyle.Padding(1, 2).Render(
		"You can configure a provider at any time with  flow auth jira",
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, title, subtitle, options, hint, sep, footer)
}

// updateOnboardView handles key input on the welcome / onboarding screen.
func (m Model) updateOnboardView(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "j", "1", "enter":
		repoRoot, _ := igit.RepoRoot()
		auth := NewJiraAuthModelEmbedded(m.cfg, repoRoot)
		auth.width = m.width
		auth.height = m.height
		inputWidth := m.width - 10
		for i := range auth.inputs {
			auth.inputs[i].Width = inputWidth
		}
		auth.projectsInput.Width = inputWidth
		m.jiraAuthModel = auth
		m.state = viewAuthJira
		return m, m.jiraAuthModel.Init()
	case "d", "2":
		m.state = viewLoading
		return m, tea.Batch(m.spinner.Tick, loadTasksCmd(m.provider))
	}
	return m, nil
}
