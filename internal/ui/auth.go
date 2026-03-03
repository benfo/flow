package ui

import (
	"fmt"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/config"
	"github.com/ben-fourie/flow-cli/internal/keychain"
	"github.com/ben-fourie/flow-cli/internal/providers/jira"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Field indices ─────────────────────────────────────────────────────────────

const (
	authFieldDomain = 0
	authFieldEmail  = 1
	authFieldToken  = 2
	authFieldCount  = 3
)

// ── Messages ──────────────────────────────────────────────────────────────────

type authVerifyMsg struct {
	displayName string
	err         error
}

// ── Model ─────────────────────────────────────────────────────────────────────

// JiraAuthModel is the Bubble Tea model for the Jira credential setup form.
// It presents a 3-field form (domain, email, API token), validates the
// credentials against Jira, then stores the token in the OS keychain and
// updates the config file.
type JiraAuthModel struct {
	inputs  [authFieldCount]textinput.Model
	focused int

	spinner  spinner.Model
	testing  bool   // API call in flight
	done     bool   // auth succeeded
	errMsg   string // validation error shown inline
	savedTo  string // path of updated config file
	userName string // display name returned by /myself

	repoRoot string // for per-repo config scope (empty = global only)
	width    int
	height   int
}

// NewJiraAuthModel builds the auth form, pre-populating fields from any
// existing Jira config so users can update without re-entering everything.
func NewJiraAuthModel(cfg config.Config, repoRoot string) JiraAuthModel {
	domain := ""
	email := ""
	if cfg.Providers.Jira != nil {
		domain = strings.TrimPrefix(cfg.Providers.Jira.BaseURL, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimRight(domain, "/")
		email = cfg.Providers.Jira.Email
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

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	return JiraAuthModel{
		inputs:   inputs,
		focused:  authFieldDomain,
		spinner:  sp,
		repoRoot: repoRoot,
	}
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (m JiraAuthModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m JiraAuthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for i := range m.inputs {
			m.inputs[i].Width = msg.Width - 10
		}
		return m, nil

	case authVerifyMsg:
		m.testing = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.userName = msg.displayName
		savedTo, err := m.saveCredentials()
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.savedTo = savedTo
		m.done = true
		return m, nil

	case spinner.TickMsg:
		if m.testing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.testing {
			return m, nil // ignore keypresses while verifying
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, tea.Quit
		case "tab", "down":
			return m.focusNext(), nil
		case "shift+tab", "up":
			return m.focusPrev(), nil
		case "enter":
			if m.focused == authFieldToken {
				return m.submit()
			}
			return m.focusNext(), nil
		}
	}

	// Route keystrokes to the focused input.
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m JiraAuthModel) View() string {
	if m.width == 0 {
		return ""
	}
	if m.done {
		return m.renderDoneView()
	}
	return m.renderFormView()
}

// ── Focus helpers ─────────────────────────────────────────────────────────────

func (m JiraAuthModel) focusNext() JiraAuthModel {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % authFieldCount
	m.inputs[m.focused].Focus()
	return m
}

func (m JiraAuthModel) focusPrev() JiraAuthModel {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused - 1 + authFieldCount) % authFieldCount
	m.inputs[m.focused].Focus()
	return m
}

// ── Submit & save ─────────────────────────────────────────────────────────────

func (m JiraAuthModel) submit() (JiraAuthModel, tea.Cmd) {
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
		// We use the Jira client directly here to avoid a full provider init.
		c := jira.NewTestClient(baseURL, email, token)
		me, err := c.Myself()
		if err != nil {
			return authVerifyMsg{err: err}
		}
		return authVerifyMsg{displayName: me.DisplayName}
	}
}

func (m JiraAuthModel) saveCredentials() (string, error) {
	domain := strings.TrimSpace(m.inputs[authFieldDomain].Value())
	email := strings.TrimSpace(m.inputs[authFieldEmail].Value())
	token := m.inputs[authFieldToken].Value()
	baseURL := "https://" + strings.TrimPrefix(domain, "https://")

	kr := keychain.New()
	if err := kr.Set("jira", email, token); err != nil {
		return "", fmt.Errorf("storing token in keychain: %w", err)
	}

	cfg, err := config.Load(m.repoRoot)
	if err != nil {
		cfg = config.Default
	}
	cfg.Providers.Jira = &config.JiraConfig{
		BaseURL: baseURL,
		Email:   email,
	}
	active := cfg.Providers.Active
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
	cfg.Providers.Active = filtered

	savedTo, err := config.SaveGlobal(cfg)
	if err != nil {
		return "", fmt.Errorf("saving config: %w", err)
	}
	return savedTo, nil
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func (m JiraAuthModel) renderFormView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  auth  /  jira", m.width)

	hint := dimStyle.Padding(1, 2).Render(
		"Create a token at: https://id.atlassian.com/manage-api-tokens",
	)

	fields := lipgloss.JoinVertical(lipgloss.Left,
		m.renderField("Jira Domain", authFieldDomain),
		m.renderField("Email", authFieldEmail),
		m.renderField("API Token", authFieldToken),
	)

	var status string
	if m.testing {
		status = lipgloss.NewStyle().Foreground(colorPrimary).Padding(0, 2).
			Render(m.spinner.View() + "  Verifying credentials…")
	} else if m.errMsg != "" {
		status = lipgloss.NewStyle().Foreground(colorPriorityCritical).Padding(0, 2).
			Render("✗  " + m.errMsg)
	}

	footerText := "tab  next field   enter  submit   esc  cancel"
	footer := renderFooterBar(footerText, m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		header, sep, hint, fields, status, sep, footer,
	)
}

func (m JiraAuthModel) renderField(label string, idx int) string {
	isFocused := idx == m.focused
	labelStyle := detailLabelStyle
	if isFocused {
		labelStyle = labelStyle.Foreground(colorPrimary).Bold(true)
	}

	borderColor := colorBorder
	if isFocused {
		borderColor = colorPrimary
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(m.inputs[idx].View())

	return lipgloss.JoinVertical(lipgloss.Left,
		labelStyle.Padding(1, 2, 0, 2).Render(label),
		lipgloss.NewStyle().Padding(0, 2).Render(inputBox),
	)
}

func (m JiraAuthModel) renderDoneView() string {
	sep := renderSeparator(m.width)
	header := renderHeaderBar("⚡ flow  /  auth  /  jira", m.width)
	footer := renderFooterBar("press any key to exit", m.width)

	heading := lipgloss.NewStyle().
		Foreground(colorStatusDone).Bold(true).Padding(1, 2).
		Render(fmt.Sprintf("✓  Connected as %s", m.userName))

	rows := strings.Join([]string{
		renderDoneRow("Domain", m.inputs[authFieldDomain].Value()),
		renderDoneRow("Email", m.inputs[authFieldEmail].Value()),
		renderDoneRow("Token", "stored in OS keychain"),
		renderDoneRow("Config", m.savedTo),
	}, "\n")

	next := dimStyle.Padding(1, 2).Render("Run 'flow' to open the dashboard.")

	body := lipgloss.JoinVertical(lipgloss.Left, heading, rows, next)
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, body, sep, footer)
}
