package cmd

import (
	"fmt"
	"os"

	"github.com/ben-fourie/flow-cli/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure branch naming preferences",
	Long: `Interactive wizard for configuring how flow generates Git branch names.

Settings are saved to ~/.config/flow-cli/config.yaml by default, or to
.flow.yaml in the repository root if you choose per-repo scope.`,
	RunE: runSetup,
}

func init() {
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
