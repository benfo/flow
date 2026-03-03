// Package cmd defines the Cobra command tree for the flow CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/ben-fourie/flow-cli/internal/config"
	igit "github.com/ben-fourie/flow-cli/internal/git"
	"github.com/ben-fourie/flow-cli/internal/tasks"
	"github.com/ben-fourie/flow-cli/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "flow",
	Short: "A developer dashboard for your terminal",
	Long: `flow is a full-screen terminal dashboard that surfaces your tasks
and upcoming calendar events so you can stay focused without leaving the terminal.`,
	RunE: runDashboard,
}

// Execute is the main entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDashboard(_ *cobra.Command, _ []string) error {
	// Resolve the repo root (empty string if not inside a repo — that's fine).
	repoRoot, _ := igit.RepoRoot()

	cfg, err := config.Load(repoRoot)
	if err != nil {
		// Config errors are non-fatal; warn and continue with defaults.
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		cfg = config.Default
	}

	provider := tasks.NewMockProvider()

	model, err := ui.New(provider, cfg)
	if err != nil {
		return fmt.Errorf("initialising dashboard: %w", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running dashboard: %w", err)
	}

	return nil
}
