package ui

import (
	"fmt"
	"strings"

	"github.com/benfo/flow/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Theme picker view ─────────────────────────────────────────────────────────

func (m Model) openThemeView() (tea.Model, tea.Cmd) {
	themes := AvailableThemes()
	cursor := 0
	for i, name := range themes {
		if strings.EqualFold(name, m.cfg.Theme) {
			cursor = i
			break
		}
	}
	m.themePreviousName = m.cfg.Theme
	m.themePickerCursor = cursor
	m.state = viewTheme
	return m, nil
}

func (m Model) updateThemeView(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	themes := AvailableThemes()
	switch key.String() {
	case "up", "k":
		if m.themePickerCursor > 0 {
			m.themePickerCursor--
			m = m.applyPickerTheme(themes[m.themePickerCursor])
		}
	case "down", "j":
		if m.themePickerCursor < len(themes)-1 {
			m.themePickerCursor++
			m = m.applyPickerTheme(themes[m.themePickerCursor])
		}
	case "enter":
		chosen := themes[m.themePickerCursor]
		m.cfg.Theme = chosen
		m.state = viewList
		return m, saveThemeCmd(m.cfg)
	case "esc", "q":
		// Revert to the theme that was active before opening.
		m = m.applyPickerTheme(m.themePreviousName)
		m.cfg.Theme = m.themePreviousName
		m.state = viewList
	}
	return m, nil
}

// applyPickerTheme switches the active theme and rebuilds all styles live.
func (m Model) applyPickerTheme(name string) Model {
	SetTheme(name)
	initStyles()
	// Re-apply list widget styles that were set from color vars at init time.
	m.list.Styles.StatusBar = dimStyle
	m.list.Styles.FilterPrompt = dimStyle.Bold(true)
	m.list.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorPrimary)
	m.list.Styles.ActivePaginationDot = lipgloss.NewStyle().Foreground(colorPrimary)
	m.list.Styles.InactivePaginationDot = lipgloss.NewStyle().Foreground(colorBorder)
	m.spinner.Style = lipgloss.NewStyle().Foreground(colorPrimary)
	return m
}

func (m Model) renderThemeView() string {
	header := renderHeaderBar("⚡ flow  /  theme", m.headerRight(), m.width)
	sep := renderSeparator(m.width)

	themes := AvailableThemes()

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(listTitleStyle.Render("Choose a theme"))
	sb.WriteString("\n\n")

	for i, name := range themes {
		label := name
		if strings.EqualFold(name, m.cfg.Theme) {
			label = fmt.Sprintf("%s  (current)", name)
		}
		if i == m.themePickerCursor {
			sb.WriteString(selectedItemStyle.Render(fmt.Sprintf("▶ %s", label)))
		} else {
			sb.WriteString(dimStyle.Render(fmt.Sprintf("  %s", label)))
		}
		sb.WriteString("\n")
	}

	hints := []string{"enter  save", "esc  cancel"}
	footer := renderFooterBar(fitHints(hints, "   ", m.width-2), m.width)

	content := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - lipgloss.Height(header) - lipgloss.Height(sep)*2 - lipgloss.Height(footer)).
		Render(sb.String())

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		content,
		sep,
		footer,
	)
}

// saveThemeCmd writes the updated theme to the global config file.
func saveThemeCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		_, err := config.SaveGlobal(cfg)
		return themeSavedMsg{err: err}
	}
}

func (m Model) handleThemeSaved(msg themeSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMessage = fmt.Sprintf("⚠ could not save theme: %s", msg.err)
	}
	return m, nil
}
