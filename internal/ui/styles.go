// Package ui contains all Bubble Tea models, views, and Lip Gloss styles
// for the flow dashboard.
package ui

import (
	"github.com/ben-fourie/flow-cli/internal/tasks"
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

	listTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)
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

// ── Badge renderers ───────────────────────────────────────────────────────────

// renderStatusBadge returns a coloured status label string.
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
