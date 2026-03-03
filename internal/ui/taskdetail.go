package ui

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/tasks"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Browser helper ────────────────────────────────────────────────────────────

// openURL returns a tea.Cmd that opens the given URL in the system's default
// browser using the platform-appropriate command. The result is fire-and-forget;
// errors are silently ignored because there is no meaningful recovery path in
// a TUI context.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			// Covers Linux and other Unix-like systems.
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}

// ── Detail renderer ───────────────────────────────────────────────────────────

// renderTaskDetail builds the full string content for the detail viewport.
// width is used to draw horizontal dividers that span the available terminal width.
// activeBranch is shown in the metadata when non-empty.
func renderTaskDetail(t tasks.Task, width int, activeBranch string) string {
	var sb strings.Builder

	divider := dividerStyle.Render(strings.Repeat("─", max(0, width-4)))

	// Title
	sb.WriteString(detailTitleStyle.Render(t.Title))
	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n\n")

	// Metadata fields
	writeField(&sb, "ID", t.ID)
	writeField(&sb, "Status", renderStatusBadge(t.Status))
	writeField(&sb, "Priority", renderPriorityBadge(t.Priority))
	writeField(&sb, "Project", t.Project)
	writeField(&sb, "Assignee", t.Assignee)

	if activeBranch != "" {
		branchVal := lipgloss.NewStyle().Foreground(colorSubtle).Render("⎇  " + activeBranch)
		writeField(&sb, "Branch", branchVal)
	}

	if t.URL != "" {
		writeField(&sb, "URL", dimStyle.Render(t.URL))
	}

	if len(t.Labels) > 0 {
		badges := make([]string, len(t.Labels))
		for i, l := range t.Labels {
			badges[i] = labelBadgeStyle.Render(l)
		}
		writeField(&sb, "Labels", lipgloss.JoinHorizontal(lipgloss.Top, badges...))
	}

	sb.WriteString("\n")
	sb.WriteString(divider)
	sb.WriteString("\n\n")

	// Description
	sb.WriteString(detailSectionHeaderStyle.Render("Description"))
	sb.WriteString("\n\n")

	// Wrap description text to fit the available width with a small margin.
	descWidth := max(0, width-6)
	sb.WriteString(lipgloss.NewStyle().Width(descWidth).Padding(0, 1).Foreground(colorText).Render(t.Description))
	sb.WriteString("\n")

	return sb.String()
}

// writeField appends a single label/value pair to the string builder.
func writeField(sb *strings.Builder, label, value string) {
	sb.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Left,
		detailLabelStyle.Render(label),
		detailValueStyle.Render(value),
	))
	sb.WriteString("\n")
}

// max returns the larger of two ints. Replaces the built-in for Go < 1.21.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
