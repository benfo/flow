// Package cmd defines the Cobra command tree for the flow CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/benfo/flow/internal/config"
	igit "github.com/benfo/flow/internal/git"
	"github.com/benfo/flow/internal/keychain"
	"github.com/benfo/flow/internal/providers"
	"github.com/benfo/flow/internal/providers/jira"
	"github.com/benfo/flow/internal/tasks"
	"github.com/benfo/flow/internal/ui"
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
	repoRoot, _ := igit.RepoRoot()

	cfg, err := config.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		cfg = config.Default
	}

	kr := keychain.New()

	registry := providers.NewRegistry()
	registry.Register("mock", func(c config.Config, k *keychain.Keychain) (tasks.Provider, bool, error) {
		return tasks.NewMockProvider(), true, nil
	})
	registry.Register("jira", jira.New)

	ps, buildErr := registry.Build(cfg, kr)
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", buildErr)
	}

	// Fall back to mock when no real providers are configured.
	if len(ps) == 0 {
		ps = append(ps, tasks.NewMockProvider())
	}

	provider := providers.NewComposite(ps)

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
