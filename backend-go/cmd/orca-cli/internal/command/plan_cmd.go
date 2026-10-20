package command

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newPlanCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "the execution plan of a Request: generate it, read its tree, start a Phase"}
	cmd.AddCommand(newPlanGenerateCmd(e), newPlanProposalCmd(e), newPlanCommitCmd(e), newPlanBundleCmd(e), newPlanRevisionsCmd(e), newPlanResultCmd(e), newPlanTreeCmd(e), newPlanStartPhaseCmd(e))
	return cmd
}

// newPlanGenerateCmd does what the UI's "Generate plan" does: propose, wait for the run, commit what it proposed.
// The Plan still needs its approval gate (`orca approval approve`) before any Phase can start.
func newPlanGenerateCmd(e flowEnv) *cobra.Command {
	var feedback string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use: "generate <request-id>", Short: "propose a plan, wait for the AI, and commit it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				body := apiclient.Document{"mode": "propose"}
				withOptional(body, "feedback", feedback)
				proposed, err := cli.GeneratePlan(ctx, args[0], body)
				if err != nil || proposed.Bool("alreadyExists") {
					return proposed, err
				}
				ready := proposed
				if proposed["proposal"] == nil {
					if ready, err = defaultWaiter().planProposal(ctx, cli, args[0], proposed.Str("runId"), timeout); err != nil {
						return nil, err
					}
				}
				commit := apiclient.Document{"mode": "commit", "proposal": ready["proposal"]}
				withOptional(commit, "rawAiResponse", ready.Str("rawAiResponse"))
				return cli.GeneratePlan(ctx, args[0], commit)
			}, nil)
		},
	}
	cmd.Flags().StringVar(&feedback, "feedback", "", "what the plan should do differently")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "give up on the run after this long (default 4m, 64m when an agent authors an OpenSpec change)")
	return cmd
}

// newPlanCommitCmd commits the plan a finished run proposed. `plan generate` does this itself, but its sign-in token can run out
// during a long wait (about ten minutes) while the run goes on, and running generate again would pay for a second run.
func newPlanCommitCmd(e flowEnv) *cobra.Command {
	var run string
	cmd := &cobra.Command{
		Use: "commit <request-id>", Short: "commit the plan a finished run proposed (when `plan generate` was cut short by an expired sign-in)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				ready, err := cli.GetPlanProposal(ctx, args[0], run)
				if err != nil {
					return nil, err
				}
				if ready.Str("status") != "succeeded" || ready["proposal"] == nil {
					return nil, &apiclient.CLIError{StatusCode: 409, Code: "REQUEST_PLAN_PROPOSAL_NOT_READY",
						Message: "the plan run is " + ready.Str("status") + ", not succeeded: wait for it with `orca plan proposal " + args[0] + "`"}
				}
				commit := apiclient.Document{"mode": "commit", "proposal": ready["proposal"]}
				withOptional(commit, "rawAiResponse", ready.Str("rawAiResponse"))
				return cli.GeneratePlan(ctx, args[0], commit)
			}, nil)
		},
	}
	cmd.Flags().StringVar(&run, "run", "", "run id (default: the latest)")
	return cmd
}

func newPlanProposalCmd(e flowEnv) *cobra.Command {
	var run string
	cmd := &cobra.Command{
		Use: "proposal <request-id>", Short: "read a plan proposal run (its status and the proposed plan)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetPlanProposal(ctx, args[0], run)
			}, renderPlanProposal)
		},
	}
	cmd.Flags().StringVar(&run, "run", "", "run id (default: the latest)")
	return cmd
}

// newPlanBundleCmd reads the OpenSpec change a plan was authored as: the file list, or one file's text. With --revision it
// reads an older version, which is read-only: a change to the plan is asked for with `orca plan generate --feedback`.
func newPlanBundleCmd(e flowEnv) *cobra.Command {
	var file, run string
	var revision int
	cmd := &cobra.Command{
		Use: "bundle <request-id>", Short: "the OpenSpec change behind a plan: its files, or one file with --file (read it before you approve the plan)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if revision < 0 {
				return e.usage("--revision must be a version number (see `orca plan revisions`)")
			}
			if file != "" {
				return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
					return cli.GetPlanBundleFileRevision(ctx, args[0], file, revision)
				}, renderBundleFile)
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetPlanProposalRevision(ctx, args[0], run, revision)
			}, renderPlanBundle)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "print this file of the change (for example tasks.md or specs/<capability>/spec.md)")
	cmd.Flags().StringVar(&run, "run", "", "run id (default: the latest)")
	cmd.Flags().IntVar(&revision, "revision", 0, "read this version of the plan (default: the current one; see `orca plan revisions`)")
	return cmd
}

// newPlanRevisionsCmd lists the versions of a plan, newest first, with the first words of what was asked for each.
func newPlanRevisionsCmd(e flowEnv) *cobra.Command {
	var pageToken string
	var pageSize int
	cmd := &cobra.Command{
		Use: "revisions <request-id>", Short: "the versions of a plan, newest first, with what was asked to get each one", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListPlanRevisions(ctx, args[0], pageToken, pageSize)
			}, renderPlanRevisions)
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "versions per page (default 20, at most 100)")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "continue from the token a previous page printed")
	return cmd
}

func newPlanTreeCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "tree <request-id>", Short: "the committed plan as a task tree (phases, tasks, their status)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				req, err := cli.GetRequest(ctx, args[0])
				if err != nil {
					return nil, err
				}
				plan := req.Str("request", "planTaskId")
				if plan == "" {
					return nil, &apiclient.CLIError{StatusCode: 404, Code: "REQUEST_PLAN_NOT_FOUND", Message: "this Request has no committed plan yet (status " + req.Str("request", "status") + ")"}
				}
				return cli.GetTaskSubtree(ctx, plan)
			}, renderPlanTree)
		},
	}
}

func newPlanStartPhaseCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "start-phase <request-id> <phase-task-id>", Short: "start a Phase whose gate is approved; its tasks go to the agents", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.StartPhase(ctx, args[0], args[1])
			}, nil)
		},
	}
}

func newPlanResultCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "result <task-id>", Short: "how a task's newest run ended: checks, files, and what the agent reports reading in the knowledge graph", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetTaskExecution(ctx, args[0])
			}, renderTaskResult)
		},
	}
}
