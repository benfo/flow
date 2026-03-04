package ui

import (
	"fmt"
	"strings"

	"github.com/benfo/flow/internal/config"
	"github.com/benfo/flow/internal/keychain"
	"github.com/benfo/flow/internal/providers/jira"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Credential field indices ───────────────────────────────────────────────────

const (
	authFieldDomain = 0
	authFieldEmail  = 1
	authFieldToken  = 2
	authFieldCount  = 3
)

// authStage tracks which page of the wizard is active.
type authStage int

const (
	authStageCreds   authStage = iota // step 1: domain / email / token
	authStageFilters                  // step 2: projects + scope
	authStageDone                     // step 3: success summary
)

// filterFocus tracks which element in the filter stage is focused.
const (
	filterFocusProjects = 0
	filterFocusScope    = 1
	filterFocusCount    = 2
)

// ── Messages ──────────────────────────────────────────────────────────────────

type authVerifyMsg struct {
	displayName string
	err         error
}

// ── Model ─────────────────────────────────────────────────────────────────────

// JiraAuthModel is the Bubble Tea model for the Jira two-stage setup wizard:
//  1. Credentials (domain, email, API token) — validated against Jira
//  2. Filters (project keys, save scope) — saved to global or per-repo config
type JiraAuthModel struct {
	// Stage 1: credential inputs
	inputs  [authFieldCount]textinput.Model
	focused int // focused credential field

	// Stage 2: filter inputs
	projectsInput textinput.Model
	filterFocused int // filterFocusProjects or filterFocusScope
	scopeIdx      int // 0=global, 1=repo
	hasRepo       bool

	// Shared state
	stage    authStage
	spinner  spinner.Model
	testing  bool   // credential API call in flight
	errMsg   string // inline error
	savedTo  string // primary path config was saved to
	userName string // from /rest/api/3/myself

	repoRoot   string
	standalone bool // true when running as a standalone command (flow auth jira)
	width      int
	height     int
}

// NewJiraAuthModel builds the wizard, pre-populating fields from any existing config.
func NewJiraAuthModel(cfg config.Config, repoRoot string) JiraAuthModel {
	domain := ""
	email := ""
	existingProjects := ""
	if cfg.Providers.Jira != nil {
		domain = strings.TrimPrefix(cfg.Providers.Jira.BaseURL, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimRight(domain, "/")
		email = cfg.Providers.Jira.Email
		existingProjects = strings.Join(cfg.Providers.Jira.Projects, ", ")
	}

	makeInput := func(placeholder string, echo textinput.EchoMode) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.EchoMode = echo
		ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
		ti.PlaceholderStyle = dimStyle
		return ti
	}

	inputs := [authFieldCount]textinput.Model{
		makeInput("yourcompany.atlassian.net", textinput.EchoNormal),
		makeInput("you@company.com", textinput.EchoNormal),
		makeInput("paste your API token here", textinput.EchoPassword),
	}
	inputs[authFieldDomain].SetValue(domain)
	inputs[authFieldEmail].SetValue(email)
	inputs[authFieldDomain].Focus()

	projectsInput := makeInput("PROJ, TEAM  (leave empty for all projects)", textinput.EchoNormal)
	projectsInput.SetValue(existingProjects)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	return JiraAuthModel{
		inputs:        inputs,
		focused:       authFieldDomain,
		projectsInput: projectsInput,
		hasRepo:       repoRoot != "",
		spinner:       sp,
		repoRoot:      repoRoot,
		standalone:    true,
	}
}

// NewJiraAuthModelEmbedded builds the wizard for use embedded inside the main app.
// When cancelled or completed it sends jiraAuthCancelledMsg / jiraAuthDoneMsg
// instead of quitting the program.
func NewJiraAuthModelEmbedded(cfg config.Config, repoRoot string) JiraAuthModel {
	m := NewJiraAuthModel(cfg, repoRoot)
	m.standalone = false
	return m
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (m JiraAuthModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m JiraAuthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputWidth := msg.Width - 10
		for i := range m.inputs {
			m.inputs[i].Width = inputWidth
		}
		m.projectsInput.Width = inputWidth
		return m, nil

	case authVerifyMsg:
		m.testing = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.userName = msg.displayName
		m.errMsg = ""
		// Advance to filter stage and focus the projects input.
		m.inputs[m.focused].Blur()
		m.stage = authStageFilters
		m.filterFocused = filterFocusProjects
		m.projectsInput.Focus()
		return m, nil

	case spinner.TickMsg:
		if m.testing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		switch m.stage {
		case authStageCreds:
			return m.updateCredsStage(msg)
		case authStageFilters:
			return m.updateFiltersStage(msg)
		case authStageDone:
			if m.standalone {
				return m, tea.Quit
			}
			return m, func() tea.Msg { return jiraAuthDoneMsg{} }
		}
	}

	// Route remaining input to the active text field.
	switch m.stage {
	case authStageCreds:
		if !m.testing {
			var cmd tea.Cmd
			m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
			return m, cmd
		}
	case authStageFilters:
		if m.filterFocused == filterFocusProjects {
			var cmd tea.Cmd
			m.projectsInput, cmd = m.projectsInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m JiraAuthModel) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.stage {
	case authStageCreds:
		return m.renderCredsView()
	case authStageFilters:
		return m.renderFiltersView()
	case authStageDone:
		return m.renderDoneView()
	}
	return ""
}

// ── Stage 1: credentials ──────────────────────────────────────────────────────

func (m JiraAuthModel) updateCredsStage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.testing {
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.standalone {
			return m, tea.Quit
		}
		return m, func() tea.Msg { return jiraAuthCancelledMsg{} }
	case "tab", "down":
		return m.credsFocusNext(), nil
	case "shift+tab", "up":
		return m.credsFocusPrev(), nil
	case "enter":
		if m.focused == authFieldToken {
			return m.submitCreds()
		}
		return m.credsFocusNext(), nil
	}
	// Delegate unrecognized keys to the focused text input.
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m JiraAuthModel) credsFocusNext() JiraAuthModel {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % authFieldCount
	m.inputs[m.focused].Focus()
	return m
}

func (m JiraAuthModel) credsFocusPrev() JiraAuthModel {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused - 1 + authFieldCount) % authFieldCount
	m.inputs[m.focused].Focus()
	return m
}

func (m JiraAuthModel) submitCreds() (JiraAuthModel, tea.Cmd) {
	domain := strings.TrimSpace(m.inputs[authFieldDomain].Value())
	email := strings.TrimSpace(m.inputs[authFieldEmail].Value())
	token := m.inputs[authFieldToken].Value()

	if domain == "" || email == "" || token == "" {
		m.errMsg = "All fields are required."
		return m, nil
	}

	m.errMsg = ""
	m.testing = true
	baseURL := "https://" + strings.TrimPrefix(domain, "https://")
	return m, tea.Batch(m.spinner.Tick, verifyJiraCmd(baseURL, email, token))
}

func verifyJiraCmd(baseURL, email, token string) tea.Cmd {
	return func() tea.Msg {
		c := jira.NewTestClient(baseURL, email, token)
		me, err := c.Myself()
		if err != nil {
			return authVerifyMsg{err: err}
		}
		return authVerifyMsg{displayName: me.DisplayName}
	}
}

// ── Stage 2: filters ──────────────────────────────────────────────────────────

func (m JiraAuthModel) updateFiltersStage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Go back to creds stage.
		m.projectsInput.Blur()
		m.stage = authStageCreds
		m.inputs[m.focused].Focus()
		return m, nil
	case "tab", "down":
		return m.filterFocusNext(), nil
	case "shift+tab", "up":
		return m.filterFocusPrev(), nil
	case "left", "right":
		if m.filterFocused == filterFocusScope && m.hasRepo {
			m.scopeIdx = 1 - m.scopeIdx
		}
		return m, nil
	case "enter":
		return m.saveFilters()
	}
	// Delegate unrecognized keys to the projects input when it is focused.
	if m.filterFocused == filterFocusProjects {
		var cmd tea.Cmd
		m.projectsInput, cmd = m.projectsInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m JiraAuthModel) filterFocusNext() JiraAuthModel {
	if m.filterFocused == filterFocusProjects {
		m.projectsInput.Blur()
	}
	m.filterFocused = (m.filterFocused + 1) % filterFocusCount
	if m.filterFocused == filterFocusProjects {
		m.projectsInput.Focus()
	}
	// If no repo, skip scope selector.
	if m.filterFocused == filterFocusScope && !m.hasRepo {
		m.filterFocused = filterFocusProjects
		m.projectsInput.Focus()
	}
	return m
}

func (m JiraAuthModel) filterFocusPrev() JiraAuthModel {
	if m.filterFocused == filterFocusProjects {
		m.projectsInput.Blur()
	}
	m.filterFocused = (m.filterFocused - 1 + filterFocusCount) % filterFocusCount
	if m.filterFocused == filterFocusProjects {
		m.projectsInput.Focus()
	}
	if m.filterFocused == filterFocusScope && !m.hasRepo {
		m.filterFocused = filterFocusProjects
		m.projectsInput.Focus()
	}
	return m
}

// parseProjects splits a comma/space-separated string into project key tokens.
func parseProjects(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, strings.ToUpper(p))
		}
	}
	return out
}

func (m JiraAuthModel) saveFilters() (JiraAuthModel, tea.Cmd) {
	domain := strings.TrimSpace(m.inputs[authFieldDomain].Value())
	email := strings.TrimSpace(m.inputs[authFieldEmail].Value())
	token := m.inputs[authFieldToken].Value()
	baseURL := "https://" + strings.TrimPrefix(domain, "https://")
	projects := parseProjects(m.projectsInput.Value())

	// Credentials always go to global config.
	kr := keychain.New()
	if err := kr.Set("jira", email, token); err != nil {
		m.errMsg = fmt.Errorf("storing token in keychain: %w", err).Error()
		return m, nil
	}

	globalCfg, err := config.Load("")
	if err != nil {
		globalCfg = config.Default
	}
	globalCfg.Providers.Jira = &config.JiraConfig{
		BaseURL: baseURL,
		Email:   email,
	}
	// Activate Jira, drop mock from global.
	globalCfg.Providers.Active = activateJira(globalCfg.Providers.Active)

	savedTo, err := config.SaveGlobal(globalCfg)
	if err != nil {
		m.errMsg = fmt.Errorf("saving global config: %w", err).Error()
		return m, nil
	}
	m.savedTo = savedTo

	// Projects can go to global or per-repo depending on scope selection.
	if len(projects) > 0 {
		if m.hasRepo && m.scopeIdx == 1 {
			// Per-repo: write a minimal .flow.yaml with just the projects filter.
			repoCfg, _ := config.Load(m.repoRoot)
			if repoCfg.Providers.Jira == nil {
				repoCfg.Providers.Jira = &config.JiraConfig{}
			}
			repoCfg.Providers.Jira.Projects = projects
			// Clear credentials from repo config — they live in global only.
			repoCfg.Providers.Jira.BaseURL = ""
			repoCfg.Providers.Jira.Email = ""
			// Don't touch Active in repo config to avoid duplicates.
			repoCfg.Providers.Active = nil
			repoPath, err := config.SaveRepo(repoCfg, m.repoRoot)
			if err != nil {
				m.errMsg = fmt.Errorf("saving repo config: %w", err).Error()
				return m, nil
			}
			m.savedTo = repoPath
		} else {
			// Global: update the Jira config we already wrote.
			globalCfg.Providers.Jira.Projects = projects
			if savedTo, err = config.SaveGlobal(globalCfg); err != nil {
				m.errMsg = fmt.Errorf("saving global config: %w", err).Error()
				return m, nil
			}
			m.savedTo = savedTo
		}
	}

	m.stage = authStageDone
	return m, nil
}

// activateJira ensures "jira" is in the active list and "mock" is removed.
func activateJira(active []string) []string {
	hasJira := false
	filtered := active[:0]
	for _, a := range active {
		if a == "jira" {
			hasJira = true
		}
		if a != "mock" {
			filtered = append(filtered, a)
		}
	}
	if !hasJira {
		filtered = append(filtered, "jira")
	}
	return filtered
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m JiraAuthModel) renderCredsView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  auth  /  jira  (1/2  credentials)", "", m.width)
	footer := renderFooterBar("tab  next field   enter  verify   esc  cancel", m.width)

	hint := dimStyle.Padding(1, 2).Render(
		"Create a token at: https://id.atlassian.com/manage-api-tokens",
	)

	fields := lipgloss.JoinVertical(lipgloss.Left,
		m.renderField("Jira Domain", authFieldDomain, m.focused == authFieldDomain),
		m.renderField("Email", authFieldEmail, m.focused == authFieldEmail),
		m.renderField("API Token", authFieldToken, m.focused == authFieldToken),
	)

	var status string
	if m.testing {
		status = lipgloss.NewStyle().Foreground(colorPrimary).Padding(0, 2).
			Render(m.spinner.View() + "  Verifying credentials…")
	} else if m.errMsg != "" {
		status = lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(0, 2).
			Render("✗  " + m.errMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, hint, fields, status, sep, footer)
}

func (m JiraAuthModel) renderFiltersView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  auth  /  jira  (2/2  filters)", "", m.width)
	footer := renderFooterBar("tab  next   ←/→  toggle scope   enter  save   esc  back", m.width)

	hint := dimStyle.Padding(1, 2).Render(
		"Filter which issues appear in the dashboard. Leave projects empty to show all.",
	)

	projectsFocused := m.filterFocused == filterFocusProjects
	projectsField := m.renderInputField("Projects", m.projectsInput, projectsFocused)

	var scopeField string
	if m.hasRepo {
		scopeFocused := m.filterFocused == filterFocusScope
		scopeField = m.renderScopeField(scopeFocused)
	}

	var errLine string
	if m.errMsg != "" {
		errLine = lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(0, 2).
			Render("✗  " + m.errMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, hint, projectsField, scopeField, errLine, sep, footer)
}

func (m JiraAuthModel) renderInputField(label string, input textinput.Model, focused bool) string {
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

// renderField renders a credential-stage field (uses inputs array).
func (m JiraAuthModel) renderField(label string, idx int, focused bool) string {
	return m.renderInputField(label, m.inputs[idx], focused)
}

func (m JiraAuthModel) renderScopeField(focused bool) string {
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
		labelStyle.Padding(1, 2, 0, 2).Render("Save projects filter to"),
		lipgloss.NewStyle().Padding(0, 2).Render(box),
	)
}

func (m JiraAuthModel) renderDoneView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  auth  /  jira", "", m.width)
	footer := renderFooterBar("press any key to continue", m.width)

	heading := lipgloss.NewStyle().
		Foreground(colorStatusDone).Bold(true).Padding(1, 2).
		Render(fmt.Sprintf("✓  Connected as %s", m.userName))

	projects := m.projectsInput.Value()
	if projects == "" {
		projects = "(all projects)"
	}
	scopeLabel := "global"
	if m.hasRepo && m.scopeIdx == 1 {
		scopeLabel = "this repository"
	}

	rows := strings.Join([]string{
		renderDoneRow("Domain", m.inputs[authFieldDomain].Value()),
		renderDoneRow("Email", m.inputs[authFieldEmail].Value()),
		renderDoneRow("Token", "stored in OS keychain"),
		renderDoneRow("Projects", projects),
		renderDoneRow("Scope", scopeLabel),
		renderDoneRow("Config", m.savedTo),
	}, "\n")

	nextText := "Run 'flow' to open the dashboard."
	if !m.standalone {
		nextText = "Press any key to continue to the dashboard."
	}
	next := dimStyle.Padding(1, 2).Render(nextText)

	body := lipgloss.JoinVertical(lipgloss.Left, heading, rows, next)
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, body, sep, footer)
}
