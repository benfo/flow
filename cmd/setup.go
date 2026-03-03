package cmd

import (
	"fmt"
	"os"

	"github.com/benfo/flow/internal/config"
	igit "github.com/benfo/flow/internal/git"
	"github.com/benfo/flow/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure flow preferences",
	Long: `Interactive wizards for configuring flow.

Run without a subcommand to configure Git branch naming.
Use 'flow setup jira' to update Jira filter settings.`,
	RunE: runSetup,
}

var setupJiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Configure Jira filter settings (projects, scope)",
	Long: `Update which Jira projects appear in the dashboard.

Requires Jira to already be authenticated. Run 'flow auth jira' first if needed.
Filter settings can be saved globally or per-repository.`,
	RunE: runSetupJira,
}

func init() {
	setupCmd.AddCommand(setupJiraCmd)
	rootCmd.AddCommand(setupCmd)
}

func runSetup(_ *cobra.Command, _ []string) error {
	model := ui.NewSetupModel()

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running setup: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Branch naming configured. Run 'flow' to open the dashboard.")
	return nil
}

func runSetupJira(_ *cobra.Command, _ []string) error {
	repoRoot, _ := igit.RepoRoot()

	cfg, err := config.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		cfg = config.Default
	}

	model, hint := ui.NewJiraConfigModel(cfg, repoRoot)
	if hint != "" {
		fmt.Fprintln(os.Stderr, hint)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running setup jira: %w", err)
	}

	return nil
}

