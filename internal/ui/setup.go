package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/benfo/flow/internal/config"
	igit "github.com/benfo/flow/internal/git"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Setup wizard ──────────────────────────────────────────────────────────────

type setupStep int

const (
	setupTemplate  setupStep = iota // choose a branch naming template
	setupSeparator setupStep = iota // choose the word separator
	setupScope     setupStep = iota // global vs per-repo save
)

// setupItem is a selectable option in the setup wizard lists.
type setupItem struct {
	title string
	desc  string
	value string
}

func (s setupItem) FilterValue() string { return s.title }
func (s setupItem) Title() string       { return s.title }
func (s setupItem) Description() string { return s.desc }

// SetupModel is the Bubble Tea model for the branch naming setup wizard.
// It walks the user through choosing a template, separator, and save scope,
// then writes the resulting BranchConfig to disk.
type SetupModel struct {
	steps   []setupStep
	stepIdx int
	lists   map[setupStep]list.Model

	draft    config.BranchConfig
	repoRoot string
	inRepo   bool

	done    bool
	savedTo string
	saveErr error

	width  int
	height int
}

// NewSetupModel builds a SetupModel, detecting whether the working directory
// is inside a Git repository to conditionally offer the per-repo save option.
func NewSetupModel() SetupModel {
	repoRoot, err := igit.RepoRoot()
	inRepo := err == nil && repoRoot != ""

	draft := config.Default.Branch

	// ── Step 1: Template ──────────────────────────────────────────────────
	templateItems := make([]list.Item, len(config.Presets))
	for i, p := range config.Presets {
		templateItems[i] = setupItem{
			title: p.Name,
			desc:  p.PatternDesc,
			value: p.Template,
		}
	}

	// ── Step 2: Separator ─────────────────────────────────────────────────
	sepItems := []list.Item{
		setupItem{"Hyphen  ( - )", "feature/proj-42-implement-oauth-login", "-"},
		setupItem{"Underscore  ( _ )", "feature/proj_42_implement_oauth_login", "_"},
	}

	// ── Step 3: Scope ─────────────────────────────────────────────────────
	scopeItems := []list.Item{
		setupItem{
			title: "Global",
			desc:  "Applies to all repositories  (~/.config/flow-cli/config.yaml)",
			value: "global",
		},
	}
	if inRepo {
		scopeItems = append(scopeItems, setupItem{
			title: "This repository only",
			desc:  fmt.Sprintf("Overrides global for this repo  (%s)", config.RepoPath(repoRoot)),
			value: "repo",
		})
	}

	// Build steps — scope is always included; the list just has one item if
	// not in a repo, making "Global" the only choice.
	steps := []setupStep{setupTemplate, setupSeparator, setupScope}

	return SetupModel{
		steps:    steps,
		stepIdx:  0,
		lists:    buildSetupLists(templateItems, sepItems, scopeItems),
		draft:    draft,
		repoRoot: repoRoot,
		inRepo:   inRepo,
	}
}

// setupDelegate renders setup wizard list items with a ▶ cursor for the
// selected row, making selection unambiguous even in colour-limited terminals.
type setupDelegate struct{}

func (d setupDelegate) Height() int                              { return 2 }
func (d setupDelegate) Spacing() int                            { return 1 }
func (d setupDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d setupDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	s, ok := item.(setupItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	var cursor, titleStr, descStr string
	if isSelected {
		cursor = selectedItemStyle.Render("▶")
		titleStr = selectedItemStyle.Bold(true).Render(s.title)
		descStr = dimStyle.Render("  " + s.desc)
	} else {
		cursor = "  "
		titleStr = normalItemStyle.Render(s.title)
		descStr = dimStyle.Render("  " + s.desc)
	}

	fmt.Fprintln(w, cursor+" "+titleStr)
	fmt.Fprint(w, descStr)
}

func buildSetupLists(templateItems, sepItems, scopeItems []list.Item) map[setupStep]list.Model {
	newList := func(items []list.Item) list.Model {
		l := list.New(items, setupDelegate{}, 0, 0)
		l.SetShowTitle(false)
		l.SetShowHelp(false)
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		return l
	}
	return map[setupStep]list.Model{
		setupTemplate:  newList(templateItems),
		setupSeparator: newList(sepItems),
		setupScope:     newList(scopeItems),
	}
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for step, l := range m.lists {
			l.SetSize(msg.Width, m.listHeight())
			m.lists[step] = l
		}
		return m, nil

	case tea.KeyMsg:
		if m.done {
			return m, tea.Quit
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.confirmStep()
		}
	}

	if !m.done {
		step := m.currentStep()
		l, cmd := m.lists[step].Update(msg)
		m.lists[step] = l
		return m, cmd
	}
	return m, nil
}

func (m SetupModel) View() string {
	if m.width == 0 {
		return ""
	}
	if m.done {
		return m.renderDoneView()
	}
	return m.renderStepView()
}

// ── Step navigation ───────────────────────────────────────────────────────────

func (m SetupModel) currentStep() setupStep {
	return m.steps[m.stepIdx]
}

func (m SetupModel) confirmStep() (tea.Model, tea.Cmd) {
	step := m.currentStep()
	item, ok := m.lists[step].SelectedItem().(setupItem)
	if !ok {
		return m, nil
	}

	switch step {
	case setupTemplate:
		m.draft.Template = item.value
	case setupSeparator:
		m.draft.Separator = item.value
	case setupScope:
		return m.save(item.value)
	}

	m.stepIdx++
	return m, nil
}

func (m SetupModel) save(scope string) (tea.Model, tea.Cmd) {
	cfg := config.Config{Branch: m.draft}

	var savedTo string
	var err error

	if scope == "repo" && m.repoRoot != "" {
		savedTo, err = config.SaveRepo(cfg, m.repoRoot)
	} else {
		savedTo, err = config.SaveGlobal(cfg)
	}

	m.saveErr = err
	m.savedTo = savedTo
	m.done = true
	return m, nil
}

// ── Live preview ──────────────────────────────────────────────────────────────

// currentPreview computes a branch name preview based on the currently
// highlighted item combined with the choices made so far.
func (m SetupModel) currentPreview() string {
	draft := m.draft
	step := m.currentStep()

	if item, ok := m.lists[step].SelectedItem().(setupItem); ok {
		switch step {
		case setupTemplate:
			draft.Template = item.value
		case setupSeparator:
			draft.Separator = item.value
		}
	}
	return draft.Preview()
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m SetupModel) listHeight() int {
	// header(1) + sep(1) + stepLabel(3) + preview(2) + sep(1) + footer(1) = 9
	return m.height - 9
}

func (m SetupModel) renderStepView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  setup", "", m.width)

	step := m.currentStep()
	total := len(m.steps)
	current := m.stepIdx + 1

	stepLabels := map[setupStep]string{
		setupTemplate:  "Step %d of %d  –  Choose a branch naming template",
		setupSeparator: "Step %d of %d  –  Choose a word separator",
		setupScope:     "Step %d of %d  –  Where should this be saved?",
	}
	stepLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Padding(1, 2).
		Render(fmt.Sprintf(stepLabels[step], current, total))

	listView := lipgloss.NewStyle().Padding(0, 1).Render(m.lists[step].View())

	previewLine := lipgloss.JoinHorizontal(lipgloss.Left,
		dimStyle.Padding(0, 2).Render("Preview:"),
		lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Padding(0, 1).Render(m.currentPreview()),
	)

	footerText := "↑/↓  navigate   enter  confirm"
	if m.stepIdx > 0 {
		footerText += "   (esc exits without saving)"
	}
	footer := renderFooterBar(footerText, m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		stepLabel,
		listView,
		"",
		previewLine,
		sep,
		footer,
	)
}

func (m SetupModel) renderDoneView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  setup", "", m.width)
	footer := renderFooterBar("press any key to exit", m.width)

	var body string
	if m.saveErr != nil {
		body = lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(2, 2).
			Render("✗  Failed to save configuration:\n\n   " + m.saveErr.Error())
	} else {
		heading := lipgloss.NewStyle().
			Foreground(colorStatusDone).Bold(true).Padding(1, 2).
			Render("✓  Configuration saved!")

		rows := strings.Join([]string{
			renderDoneRow("Template", presetNameFor(m.draft.Template)),
			renderDoneRow("Separator", separatorLabel(m.draft.Separator)),
			renderDoneRow("Saved to", m.savedTo),
		}, "\n")

		preview := lipgloss.JoinHorizontal(lipgloss.Left,
			dimStyle.Padding(1, 2).Render("Preview:"),
			lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Padding(1, 1).Render(m.draft.Preview()),
		)

		body = lipgloss.JoinVertical(lipgloss.Left, heading, rows, preview)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, body, sep, footer)
}

func renderDoneRow(label, value string) string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		detailLabelStyle.Render(label),
		detailValueStyle.Render(value),
	)
}

func presetNameFor(template string) string {
	for _, p := range config.Presets {
		if p.Template == template {
			return p.Name + "  (" + template + ")"
		}
	}
	return template
}

func separatorLabel(sep string) string {
	switch sep {
	case "-":
		return "Hyphen  ( - )"
	case "_":
		return "Underscore  ( _ )"
	default:
		return sep
	}
}
