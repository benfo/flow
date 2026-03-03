package ui

import (
	"fmt"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JiraConfigModel is a focused TUI for updating Jira filter settings without
// going through the full auth wizard. It requires Jira to already be
// authenticated (i.e. config.Providers.Jira must exist).
type JiraConfigModel struct {
	projectsInput textinput.Model
	scopeIdx      int // 0=global, 1=repo
	scopeFocused  bool
	hasRepo       bool

	errMsg  string
	savedTo string
	done    bool

	cfg      config.Config
	repoRoot string
	width    int
	height   int
}

// NewJiraConfigModel builds the model. Returns an error message string (not
// a Go error) when Jira has not been authenticated so the caller can print
// a helpful hint and exit early.
func NewJiraConfigModel(cfg config.Config, repoRoot string) (JiraConfigModel, string) {
	if cfg.Providers.Jira == nil {
		return JiraConfigModel{}, "Jira is not configured. Run 'flow auth jira' first."
	}

	// Pre-populate projects from the effective (merged) config.
	existing := strings.Join(cfg.Providers.Jira.Projects, ", ")

	ti := textinput.New()
	ti.Placeholder = "PROJ, TEAM  (leave empty for all projects)"
	ti.EchoMode = textinput.EchoNormal
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.PlaceholderStyle = dimStyle
	ti.SetValue(existing)
	ti.Focus()

	return JiraConfigModel{
		projectsInput: ti,
		hasRepo:       repoRoot != "",
		cfg:           cfg,
		repoRoot:      repoRoot,
	}, ""
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (m JiraConfigModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m JiraConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.projectsInput.Width = msg.Width - 10
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "down":
			return m.focusNext(), nil
		case "shift+tab", "up":
			return m.focusPrev(), nil
		case "left", "right":
			if m.scopeFocused && m.hasRepo {
				m.scopeIdx = 1 - m.scopeIdx
			}
			return m, nil
		case "enter":
			return m.save()
		}
	}

	if !m.scopeFocused {
		var cmd tea.Cmd
		m.projectsInput, cmd = m.projectsInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m JiraConfigModel) View() string {
	if m.width == 0 {
		return ""
	}
	if m.done {
		return m.renderDoneView()
	}
	return m.renderFormView()
}

// ── Focus ─────────────────────────────────────────────────────────────────────

func (m JiraConfigModel) focusNext() JiraConfigModel {
	if !m.scopeFocused {
		if m.hasRepo {
			m.projectsInput.Blur()
			m.scopeFocused = true
		}
		// No repo — stay on projects input.
	} else {
		m.scopeFocused = false
		m.projectsInput.Focus()
	}
	return m
}

func (m JiraConfigModel) focusPrev() JiraConfigModel {
	return m.focusNext() // only two focusable elements; toggle is symmetric
}

// ── Save ──────────────────────────────────────────────────────────────────────

func (m JiraConfigModel) save() (JiraConfigModel, tea.Cmd) {
	projects := parseProjects(m.projectsInput.Value())

	if m.hasRepo && m.scopeIdx == 1 {
		// Per-repo: write only the projects filter to .flow.yaml.
		// Load any existing repo config to avoid clobbering branch settings.
		repoCfg, _ := config.Load(m.repoRoot)
		if repoCfg.Providers.Jira == nil {
			repoCfg.Providers.Jira = &config.JiraConfig{}
		}
		repoCfg.Providers.Jira.Projects = projects
		// Never write credentials into the repo config.
		repoCfg.Providers.Jira.BaseURL = ""
		repoCfg.Providers.Jira.Email = ""
		repoCfg.Providers.Active = nil
		path, err := config.SaveRepo(repoCfg, m.repoRoot)
		if err != nil {
			m.errMsg = fmt.Errorf("saving repo config: %w", err).Error()
			return m, nil
		}
		m.savedTo = path
	} else {
		// Global: update the existing global Jira config in place.
		globalCfg, err := config.Load("")
		if err != nil {
			globalCfg = m.cfg
		}
		if globalCfg.Providers.Jira == nil {
			globalCfg.Providers.Jira = &config.JiraConfig{}
		}
		globalCfg.Providers.Jira.Projects = projects
		path, err := config.SaveGlobal(globalCfg)
		if err != nil {
			m.errMsg = fmt.Errorf("saving global config: %w", err).Error()
			return m, nil
		}
		m.savedTo = path
	}

	m.done = true
	return m, nil
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m JiraConfigModel) renderFormView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  setup  /  jira", m.width)
	footer := renderFooterBar("tab  toggle scope   ←/→  switch scope   enter  save   esc  cancel", m.width)

	domain := m.cfg.Providers.Jira.BaseURL
	hint := dimStyle.Padding(1, 2).Render("Connected to: " + domain)

	projectsField := m.renderInputField("Projects", m.projectsInput, !m.scopeFocused)

	var scopeField string
	if m.hasRepo {
		scopeField = m.renderScopeField(m.scopeFocused)
	}

	var errLine string
	if m.errMsg != "" {
		errLine = lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(0, 2).
			Render("✗  " + m.errMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, hint, projectsField, scopeField, errLine, sep, footer)
}

func (m JiraConfigModel) renderInputField(label string, input textinput.Model, focused bool) string {
	labelStyle := detailLabelStyle
	if focused {
		labelStyle = labelStyle.Foreground(colorPrimary).Bold(true)
	}
	borderColor := colorBorder
	if focused {
		borderColor = colorPrimary
	}
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(input.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Padding(1, 2, 0, 2).Render(label),
		lipgloss.NewStyle().Padding(0, 2).Render(inputBox),
	)
}

func (m JiraConfigModel) renderScopeField(focused bool) string {
	labelStyle := detailLabelStyle
	if focused {
		labelStyle = labelStyle.Foreground(colorPrimary).Bold(true)
	}

	scopes := []string{"Global (~/.config/flow-cli/config.yaml)", "This repository (.flow.yaml)"}
	var opts []string
	for i, s := range scopes {
		if i == m.scopeIdx {
			opts = append(opts, lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("● "+s))
		} else {
			opts = append(opts, dimStyle.Render("○ "+s))
		}
	}

	borderColor := colorBorder
	if focused {
		borderColor = colorPrimary
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(strings.Join(opts, "   "))

	return lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Padding(1, 2, 0, 2).Render("Save to"),
		lipgloss.NewStyle().Padding(0, 2).Render(box),
	)
}

func (m JiraConfigModel) renderDoneView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  setup  /  jira", m.width)
	footer := renderFooterBar("press any key to exit", m.width)

	heading := lipgloss.NewStyle().
		Foreground(colorStatusDone).Bold(true).Padding(1, 2).
		Render("✓  Jira filters updated")

	projects := m.projectsInput.Value()
	if projects == "" {
		projects = "(all projects)"
	}
	scopeLabel := "global"
	if m.hasRepo && m.scopeIdx == 1 {
		scopeLabel = "this repository"
	}

	rows := strings.Join([]string{
		renderDoneRow("Projects", projects),
		renderDoneRow("Scope", scopeLabel),
		renderDoneRow("Config", m.savedTo),
	}, "\n")

	next := dimStyle.Padding(1, 2).Render("Run 'flow' to apply changes.")

	body := lipgloss.JoinVertical(lipgloss.Left, heading, rows, next)
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, body, sep, footer)
}
