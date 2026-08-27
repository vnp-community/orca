// Package command builds orca-cli's cobra command tree and the
// process-exit-code contract main.go relies on.
package command

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/config"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
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
	root.AddCommand(newAgentCmd(clientFactory, &jsonOutput, exitCode))
	root.AddCommand(newSnapshotCmd(clientFactory, &jsonOutput, exitCode))
	root.AddCommand(newDaemonCmd(clientFactory, &jsonOutput, exitCode))
	root.AddCommand(newServeCmd(&jsonOutput, exitCode))
	return root, exitCode
}

// newComposeSupervisor builds a ComposeSupervisor from the --compose-file/
// --pid-file flag overrides (empty means "use the localdaemon package's
// default resolution" — env override or repo-cwd-relative compose file,
// XDG data-dir pidfile).
func newComposeSupervisor(composeFile, pidFile string) (*localdaemon.ComposeSupervisor, error) {
	if composeFile == "" {
		composeFile = localdaemon.DefaultComposeFile()
	}
	if pidFile == "" {
		var err error
		pidFile, err = localdaemon.DefaultPidFile()
		if err != nil {
			return nil, err
		}
	}
	return &localdaemon.ComposeSupervisor{ComposeFile: composeFile, PidFile: pidFile}, nil
}

func newDaemonCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	cmd := &cobra.Command{Use: "daemon", Short: "inspect or control the Orca daemon (remote health check, or a local docker-compose stack)"}
	cmd.AddCommand(newDaemonStatusCmd(clientFactory, jsonOutput, exitCode))
	cmd.AddCommand(newDaemonStopCmd(jsonOutput, exitCode))
	return cmd
}

func newDaemonStopCmd(jsonOutput *bool, exitCode *int) *cobra.Command {
	var local bool
	var composeFile, pidFile string
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop the local docker-compose stack (--local); refused in remote/GitOps mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := ModeRemote
			var sup *localdaemon.ComposeSupervisor
			if local {
				mode = ModeLocal
				var err error
				sup, err = newComposeSupervisor(composeFile, pidFile)
				if err != nil {
					*exitCode = output.ReportError(err, *jsonOutput)
					return nil
				}
			}

			if err := RunDaemonStop(cmd.Context(), mode, sup); err != nil {
				*exitCode = output.ReportError(err, *jsonOutput)
				return nil
			}
			*exitCode = output.Report(map[string]string{"status": "stopped"}, nil, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "stop this host's local docker-compose stack")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "path to docker-compose.yml (--local only; default: $ORCA_COMPOSE_FILE or ./docker-compose.yml)")
	cmd.Flags().StringVar(&pidFile, "pid-file", "", "path to the supervisor pidfile (--local only; default: XDG data dir/orca/daemon.pid)")
	return cmd
}

func newServeCmd(jsonOutput *bool, exitCode *int) *cobra.Command {
	var local, daemon bool
	var composeFile, pidFile string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the local docker-compose stack (--local required; --daemon backgrounds it)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemon && local {
				// Re-exec detached (localdaemon.Daemonize), stripping
				// --daemon so the child runs RunServe directly in the
				// foreground of its own detached session instead of
				// re-daemonizing forever. See serve.go's doc comment:
				// daemonizing is a main.go/root.go-level concern, RunServe
				// itself stays synchronous and testable.
				childArgs := stripFlag(os.Args[1:], "--daemon")
				if err := localdaemon.Daemonize(childArgs); err != nil {
					*exitCode = output.ReportError(err, *jsonOutput)
					return nil
				}
				fmt.Println("orca serve started in the background")
				*exitCode = output.ExitOK
				return nil
			}

			sup, err := newComposeSupervisor(composeFile, pidFile)
			if err != nil {
				*exitCode = output.ReportError(err, *jsonOutput)
				return nil
			}
			if err := RunServe(cmd.Context(), local, sup); err != nil {
				*exitCode = output.ReportError(err, *jsonOutput)
				return nil
			}
			*exitCode = output.Report(map[string]string{"status": "started"}, nil, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "start this host's docker-compose stack (required)")
	cmd.Flags().BoolVar(&daemon, "daemon", false, "background the process after starting")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "path to docker-compose.yml (default: $ORCA_COMPOSE_FILE or ./docker-compose.yml)")
	cmd.Flags().StringVar(&pidFile, "pid-file", "", "path to the supervisor pidfile (default: XDG data dir/orca/daemon.pid)")
	return cmd
}

// stripFlag removes every exact-match occurrence of flag (a boolean flag
// with no attached value, e.g. "--daemon") from args before re-exec'ing
// the detached child — the child must not re-daemonize itself.
func stripFlag(args []string, flag string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == flag {
			continue
		}
		out = append(out, a)
	}
	return out
}

func newDaemonStatusCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	var local bool
	var composeFile, pidFile string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "report daemon health — remote api-gateway healthz/readyz by default, or this host's compose stack with --local",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := ModeRemote
			var cli *apiclient.Client
			var sup *localdaemon.ComposeSupervisor
			// The two modes must never bleed into each other — see
			// daemon_status.go's doc comment: only one of cli/sup is ever
			// non-nil below, matching RunDaemonStatus's nil-safety
			// contract.
			if local {
				mode = ModeLocal
				var err error
				sup, err = newComposeSupervisor(composeFile, pidFile)
				if err != nil {
					*exitCode = output.ReportError(err, *jsonOutput)
					return nil
				}
			} else {
				var err error
				cli, err = clientFactory()
				if err != nil {
					*exitCode = output.ReportError(err, *jsonOutput)
					return nil
				}
			}

			result, err := RunDaemonStatus(cmd.Context(), mode, cli, sup)
			if err != nil {
				*exitCode = output.ReportError(err, *jsonOutput)
				return nil
			}
			*exitCode = output.Report(result, nil, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "check this host's local docker-compose stack instead of a remote api-gateway")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "path to docker-compose.yml (--local only; default: $ORCA_COMPOSE_FILE or ./docker-compose.yml)")
	cmd.Flags().StringVar(&pidFile, "pid-file", "", "path to the supervisor pidfile (--local only; default: XDG data dir/orca/daemon.pid)")
	return cmd
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

func newAgentCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "interact with a worktree's running agent"}
	cmd.AddCommand(newAgentStatusCmd(clientFactory, jsonOutput, exitCode))
	cmd.AddCommand(newAgentWaitCmd(clientFactory, jsonOutput, exitCode))
	cmd.AddCommand(newAgentSendCmd(clientFactory, jsonOutput, exitCode))
	return cmd
}

func newAgentStatusCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	var worktreeID string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "report whether an agent is running in a worktree, and whether it's ready for input",
		RunE: func(cmd *cobra.Command, args []string) error {
			*exitCode = ExecuteAgentStatus(cmd.Context(), clientFactory, worktreeID, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().StringVar(&worktreeID, "worktree", "", "worktree ID (required)")
	return cmd
}

func newAgentWaitCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	var worktreeID string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "block until the worktree's agent exits or --timeout elapses (BR-CLI-05: a timeout exits 2)",
		RunE: func(cmd *cobra.Command, args []string) error {
			*exitCode = ExecuteAgentWait(cmd.Context(), clientFactory, worktreeID, timeout, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().StringVar(&worktreeID, "worktree", "", "worktree ID (required)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "how long to wait before giving up (e.g. 30s, 5m)")
	return cmd
}

func newAgentSendCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	var worktreeID, text string
	cmd := &cobra.Command{
		Use:   "send",
		Short: "send text input to the worktree's running agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved := text
			if resolved == "" {
				// No --text: fall back to stdin, matching every other
				// `orca ... send`-shaped CLI's convention (e.g. `kubectl
				// apply -f -`) so this composes in a shell pipeline.
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					*exitCode = output.ReportError(err, *jsonOutput)
					return nil
				}
				resolved = string(data)
			}
			*exitCode = ExecuteAgentSend(cmd.Context(), clientFactory, worktreeID, resolved, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().StringVar(&worktreeID, "worktree", "", "worktree ID (required)")
	cmd.Flags().StringVar(&text, "text", "", "text to send (default: read from stdin)")
	return cmd
}

func newSnapshotCmd(clientFactory func() (*apiclient.Client, error), jsonOutput *bool, exitCode *int) *cobra.Command {
	var worktreeID, outputPath string
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "capture a worktree's agent scrollback as a flat text file (BR-CLI-06)",
		RunE: func(cmd *cobra.Command, args []string) error {
			*exitCode = ExecuteSnapshot(cmd.Context(), clientFactory, worktreeID, outputPath, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().StringVar(&worktreeID, "worktree", "", "worktree ID (required)")
	cmd.Flags().StringVar(&outputPath, "output", "", "file to write the snapshot to (default: stdout)")
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
