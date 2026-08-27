// Package command builds orca-cli's cobra command tree and the
// process-exit-code contract main.go relies on.
package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/config"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

// defaultAPIURL is used only when neither ORCA_API_URL nor a saved
// credentials file supplies one — see config.ResolveAPIURL.
const defaultAPIURL = "http://localhost:8080"

// DefaultClientFactory resolves an apiclient.Client from on-disk
// credentials (~/.config/orca/credentials.json), honoring the
// ORCA_API_URL/ORCA_API_TOKEN env overrides config.Resolve* implements.
// This is the real, non-test seam every command's RunE uses; tests supply
// their own factory pointing at an httptest server instead.
func DefaultClientFactory() (*apiclient.Client, error) {
	creds, err := config.Load()
	if err != nil {
		return nil, err
	}
	apiURL := config.ResolveAPIURL(creds, defaultAPIURL)
	token := config.ResolveToken(creds)
	return apiclient.New(apiURL, token), nil
}

// NewRootCmd builds orca-cli's full command tree. clientFactory constructs
// the apiclient.Client every leaf command uses to reach api-gateway — a
// seam tests override to point at an httptest server instead of resolving
// real on-disk credentials via config.Load(). The returned *int is filled
// in with the BR-CLI-02/03 exit code once Execute() runs a leaf command's
// RunE (cobra itself has no exit-code concept beyond nil/non-nil error).
func NewRootCmd(clientFactory func() (*apiclient.Client, error)) (*cobra.Command, *int) {
	exitCode := new(int)
	var jsonOutput bool

	root := &cobra.Command{
		Use:   "orca",
		Short: "orca-cli — command-line client for the Orca platform",
		// Cobra's default usage/error printing duplicates what
		// output.ReportError already writes for every real command error —
		// silence both so a failed leaf command's stderr is exactly one
		// line (or one JSON object in --json mode), not two.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "emit machine-readable JSON output")

	root.AddCommand(newWorktreeCmd(clientFactory, &jsonOutput, exitCode))
	return root, exitCode
}

func newWorktreeCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	cmd := &cobra.Command{Use: "worktree", Short: "manage worktrees"}
	cmd.AddCommand(newWorktreeCreateCmd(clientFactory, jsonOutput, exitCode))
	return cmd
}

func newWorktreeCreateCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	var opts WorktreeCreateOptions
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a new worktree (and best-effort spawn an agent in it)",
		RunE: func(cmd *cobra.Command, args []string) error {
			*exitCode = ExecuteWorktreeCreate(cmd.Context(), clientFactory, opts, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.ProjectID, "project-id", "", "project ID (required)")
	cmd.Flags().StringVar(&opts.RepoID, "repo-id", "", "repo ID (required)")
	cmd.Flags().StringVar(&opts.Name, "name", "", "new branch/worktree name (required)")
	cmd.Flags().StringVar(&opts.Base, "base", "", "base ref to branch from")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "agent type to best-effort spawn in the new worktree")
	cmd.Flags().StringVar(&opts.Prompt, "prompt", "", "initial prompt to send the spawned agent")
	cmd.Flags().StringVar(&opts.IdempotencyKeyOverride, "idempotency-key", "", "override the auto-derived idempotency key (BR-CLI-01)")
	return cmd
}

// Run parses real os.Args against orca-cli's full command tree (real
// on-disk credentials via DefaultClientFactory) and returns the process
// exit code main.go should pass to os.Exit.
func Run() int {
	root, exitCode := NewRootCmd(DefaultClientFactory)
	*exitCode = output.ExitOK
	if err := root.Execute(); err != nil {
		// cobra failed before any RunE ran (unknown flag/command, etc.) —
		// that's a usage error by definition.
		fmt.Fprintln(os.Stderr, err)
		return output.ExitUsageError
	}
	return *exitCode
}
