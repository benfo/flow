// Package ui contains all Bubble Tea models, views, and Lip Gloss styles
// for the flow dashboard.
package ui

import (
	"strings"

	"github.com/benfo/flow/internal/tasks"
	"github.com/charmbracelet/lipgloss"
)

// ── Colour palette ────────────────────────────────────────────────────────────
// Inspired by the Tokyo Night theme for consistency across modern terminals.

const (
	colorSurface  = lipgloss.Color("#24283b")
	colorText     = lipgloss.Color("#c0caf5")
	colorSubtle   = lipgloss.Color("#565f89")
	colorPrimary  = lipgloss.Color("#7aa2f7")
	colorBorder   = lipgloss.Color("#3b4261")

	colorStatusTodo       = lipgloss.Color("#565f89")
	colorStatusInProgress = lipgloss.Color("#7aa2f7")
	colorStatusInReview   = lipgloss.Color("#e0af68")
	colorStatusDone       = lipgloss.Color("#9ece6a")

	colorPriorityLow      = lipgloss.Color("#565f89")
	colorPriorityMedium   = lipgloss.Color("#e0af68")
	colorPriorityHigh     = lipgloss.Color("#ff9e64")
	colorPriorityCritical = lipgloss.Color("#f7768e")
)

// ── Layout styles ─────────────────────────────────────────────────────────────

var (
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
)

// ── List item styles ──────────────────────────────────────────────────────────

var (
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(colorText)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)
)

// ── Detail view styles ────────────────────────────────────────────────────────

var (
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
)

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

// renderDiscardConfirm renders the inline "discard changes?" confirmation bar
// shown when the user presses esc on a dirty form.
func renderDiscardConfirm() string {
	label := lipgloss.NewStyle().Foreground(colorPriorityCritical).Bold(true).Render("Discard changes?")
	yes := lipgloss.NewStyle().Foreground(colorStatusDone).Bold(true).Render("y")
	no := lipgloss.NewStyle().Foreground(colorSubtle).Render("n")
	hint := lipgloss.NewStyle().Foreground(colorSubtle).Render(" yes  /  ")
	hintNo := lipgloss.NewStyle().Foreground(colorSubtle).Render(" keep editing")
	return lipgloss.NewStyle().Padding(0, 2).Render(
		label + "   " + yes + hint + no + hintNo,
	)
}

// renderDeleteConfirm renders the inline "delete task?" confirmation bar
// shown when the user presses D on the detail view.
func renderDeleteConfirm(taskID string) string {
	label := lipgloss.NewStyle().Foreground(colorPriorityCritical).Bold(true).Render("Delete " + taskID + "?")
	yes := lipgloss.NewStyle().Foreground(colorPriorityCritical).Bold(true).Render("y")
	no := lipgloss.NewStyle().Foreground(colorSubtle).Render("n")
	hint := lipgloss.NewStyle().Foreground(colorSubtle).Render(" yes  /  ")
	hintNo := lipgloss.NewStyle().Foreground(colorSubtle).Render(" cancel")
	return lipgloss.NewStyle().Padding(0, 2).Render(
		label + "   " + yes + hint + no + hintNo,
	)
}

// renderBranchPrompt renders an inline "question? y / n" bar used in the branch view.
func renderBranchPrompt(question string) string {
	label := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(question)
	yes := lipgloss.NewStyle().Foreground(colorStatusDone).Bold(true).Render("y")
	no := lipgloss.NewStyle().Foreground(colorSubtle).Render("n")
	hint := lipgloss.NewStyle().Foreground(colorSubtle).Render(" yes  /  ")
	hintNo := lipgloss.NewStyle().Foreground(colorSubtle).Render(" no")
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


func renderStatusBadge(s tasks.Status) string {
	color := statusColor(s)
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(s.String())
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

func renderHeaderBar(title string, width int) string {
	return appHeaderStyle.Width(width).Render(title)
}

func renderFooterBar(help string, width int) string {
	return appFooterStyle.Width(width).Render(help)
}

func renderSeparator(width int) string {
	return separatorStyle.Render(strings.Repeat("─", width))
}
