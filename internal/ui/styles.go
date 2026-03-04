// Package ui contains all Bubble Tea models, views, and Lip Gloss styles
// for the flow dashboard.
package ui

import (
	"strings"

	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/lipgloss"
)

// ── Colour palette ────────────────────────────────────────────────────────────
// These vars are set by initStyles() from the active theme. Do not reference
// them before initStyles() has been called (i.e. before ui.New()).

var (
	colorSurface  lipgloss.Color
	colorText     lipgloss.Color
	colorSubtle   lipgloss.Color
	colorPrimary  lipgloss.Color
	colorBorder   lipgloss.Color

	colorStatusTodo       lipgloss.Color
	colorStatusInProgress lipgloss.Color
	colorStatusInReview   lipgloss.Color
	colorStatusDone       lipgloss.Color

	colorPriorityLow      lipgloss.Color
	colorPriorityMedium   lipgloss.Color
	colorPriorityHigh     lipgloss.Color
	colorPriorityCritical lipgloss.Color
)

// ── Layout styles ─────────────────────────────────────────────────────────────
// Declared here; assigned in initStyles() after the theme is loaded.

var (
	appHeaderStyle           lipgloss.Style
	appFooterStyle           lipgloss.Style
	separatorStyle           lipgloss.Style
	listTitleStyle           lipgloss.Style
	emptyStateStyle          lipgloss.Style
	selectedItemStyle        lipgloss.Style
	normalItemStyle          lipgloss.Style
	dimStyle                 lipgloss.Style
	detailTitleStyle         lipgloss.Style
	detailSectionHeaderStyle lipgloss.Style
	detailLabelStyle         lipgloss.Style
	detailValueStyle         lipgloss.Style
	dividerStyle             lipgloss.Style
	labelBadgeStyle          lipgloss.Style
)

// initStyles rebuilds every Lip Gloss style from the current activeTheme.
// Must be called once after SetTheme(), before any rendering.
func initStyles() {
	// Populate colour vars from the active theme.
	colorSurface  = activeTheme.Surface
	colorText     = activeTheme.Text
	colorSubtle   = activeTheme.Subtle
	colorPrimary  = activeTheme.Primary
	colorBorder   = activeTheme.Border

	colorStatusTodo       = activeTheme.StatusTodo
	colorStatusInProgress = activeTheme.StatusInProgress
	colorStatusInReview   = activeTheme.StatusInReview
	colorStatusDone       = activeTheme.StatusDone

	colorPriorityLow      = activeTheme.PriorityLow
	colorPriorityMedium   = activeTheme.PriorityMedium
	colorPriorityHigh     = activeTheme.PriorityHigh
	colorPriorityCritical = activeTheme.PriorityCritical

	// Rebuild layout styles.
	appHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Background(colorSurface).
		Padding(0, 1)

	appFooterStyle = lipgloss.NewStyle().
		Foreground(colorSubtle).
		Background(colorSurface).
		Padding(0, 1)

	separatorStyle = lipgloss.NewStyle().
		Foreground(colorBorder)

	listTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Padding(0, 1)

	emptyStateStyle = lipgloss.NewStyle().
		Foreground(colorSubtle).
		Italic(true).
		Padding(2, 2)

	// List item styles.
	selectedItemStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary)

	normalItemStyle = lipgloss.NewStyle().
		Foreground(colorText)

	dimStyle = lipgloss.NewStyle().
		Foreground(colorSubtle)

	// Detail view styles.
	detailTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Padding(0, 1)

	detailSectionHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorSubtle).
		Padding(0, 1)

	detailLabelStyle = lipgloss.NewStyle().
		Foreground(colorSubtle).
		Width(12).
		Padding(0, 1)

	detailValueStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Padding(0, 1)

	dividerStyle = lipgloss.NewStyle().
		Foreground(colorBorder)

	labelBadgeStyle = lipgloss.NewStyle().
		Foreground(colorSubtle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)
}


// ── Form field helper (shared by taskedit.go and taskcreate.go) ───────────────

// renderFormField renders a labelled bordered box around content. Used by both
// the edit and create forms so the visual treatment is identical.
func renderFormField(label, content string, focused bool) string {
	borderColor := colorBorder
	if focused {
		borderColor = colorPrimary
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(content)
	return lipgloss.JoinVertical(lipgloss.Left,
		editLabelStyle(focused).Padding(1, 2, 0, 2).Render(label),
		lipgloss.NewStyle().Padding(0, 2).Render(box),
	)
}

// renderConfirmFooter renders a consistent confirmation prompt in the footer bar.
// For destructive actions pass destructive=true to render the question in red.
func renderConfirmFooter(question string, destructive bool) string {
	qColor := colorPrimary
	if destructive {
		qColor = colorPriorityCritical
	}
	label := lipgloss.NewStyle().Foreground(qColor).Bold(true).Render(question)
	yes := lipgloss.NewStyle().Foreground(colorStatusDone).Bold(true).Render("y")
	no := lipgloss.NewStyle().Foreground(colorSubtle).Render("n")
	hint := lipgloss.NewStyle().Foreground(colorSubtle).Render(" yes  /  ")
	hintNo := lipgloss.NewStyle().Foreground(colorSubtle).Render(" cancel")
	return lipgloss.NewStyle().Padding(0, 2).Render(
		label + "   " + yes + hint + no + hintNo,
	)
}


// fitHints joins hints with sep, greedily including as many as fit within
// maxWidth visible characters. If any hints are dropped a "…" is appended.
// sep is the separator between hints (e.g. "   ").
// This keeps the footer to a single line on any terminal width.
func fitHints(hints []string, sep string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	var fitted []string
	used := 0
	sepLen := len(sep)
	for i, h := range hints {
		extra := len(h)
		if i > 0 {
			extra += sepLen
		}
		if used+extra > maxWidth {
			fitted = append(fitted, "…")
			break
		}
		fitted = append(fitted, h)
		used += extra
	}
	return strings.Join(fitted, sep)
}


// renderStatusBadge returns a coloured status label. label is the text to
// display; if empty the canonical status string is used as a fallback.
func renderStatusBadge(s tasks.Status, label string) string {
	if label == "" {
		label = s.String()
	}
	return lipgloss.NewStyle().Foreground(statusColor(s)).Bold(true).Render(label)
}

// renderPriorityBadge returns a coloured priority label string with a visual indicator.
func renderPriorityBadge(p tasks.Priority) string {
	color := priorityColor(p)
	indicator := priorityIndicator(p)
	return lipgloss.NewStyle().Foreground(color).Render(indicator + " " + p.String())
}

func statusColor(s tasks.Status) lipgloss.Color {
	switch s {
	case tasks.StatusTodo:
		return colorStatusTodo
	case tasks.StatusInProgress:
		return colorStatusInProgress
	case tasks.StatusInReview:
		return colorStatusInReview
	case tasks.StatusDone:
		return colorStatusDone
	default:
		return colorSubtle
	}
}

func priorityColor(p tasks.Priority) lipgloss.Color {
	switch p {
	case tasks.PriorityLow:
		return colorPriorityLow
	case tasks.PriorityMedium:
		return colorPriorityMedium
	case tasks.PriorityHigh:
		return colorPriorityHigh
	case tasks.PriorityCritical:
		return colorPriorityCritical
	default:
		return colorSubtle
	}
}

func priorityIndicator(p tasks.Priority) string {
	switch p {
	case tasks.PriorityLow:
		return "↓"
	case tasks.PriorityMedium:
		return "→"
	case tasks.PriorityHigh:
		return "↑"
	case tasks.PriorityCritical:
		return "⚡"
	default:
		return " "
	}
}

func renderHeaderBar(left, right string, width int) string {
	if right == "" {
		return appHeaderStyle.Width(width).Render(left)
	}
	// appHeaderStyle has Padding(0,1) = 1 char each side, so inner width = width-2.
	innerWidth := width - 2
	// right may already contain pre-styled segments (e.g. update badge); don't re-dim it.
	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return appHeaderStyle.Width(width).Render(left + strings.Repeat(" ", gap) + right)
}

func renderFooterBar(help string, width int) string {
	return appFooterStyle.Width(width).Render(help)
}

func renderSeparator(width int) string {
	return separatorStyle.Render(strings.Repeat("─", width))
}
