package cmd

import (
	"fmt"
	"os"

	"github.com/ben-fourie/flow-cli/internal/config"
	igit "github.com/ben-fourie/flow-cli/internal/git"
	"github.com/ben-fourie/flow-cli/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with task / calendar providers",
}

var authJiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Connect your Jira account",
	Long: `Interactive wizard to connect flow to your Jira Cloud account.

Your API token is stored securely in the OS keychain and never written to disk.
To generate a token visit: https://id.atlassian.com/manage-api-tokens`,
	RunE: runAuthJira,
}

func init() {
	authCmd.AddCommand(authJiraCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthJira(_ *cobra.Command, _ []string) error {
	repoRoot, _ := igit.RepoRoot()

	cfg, err := config.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		cfg = config.Default
	}

	model := ui.NewJiraAuthModel(cfg, repoRoot)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running auth: %w", err)
	}

	return nil
}
