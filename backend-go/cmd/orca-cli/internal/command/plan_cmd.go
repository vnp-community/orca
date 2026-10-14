package command

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newPlanCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "the execution plan of a Request: generate it, read its tree, start a Phase"}
	cmd.AddCommand(newPlanGenerateCmd(e), newPlanProposalCmd(e), newPlanTreeCmd(e), newPlanStartPhaseCmd(e))
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
	cmd.Flags().DurationVar(&timeout, "timeout", 4*time.Minute, "give up on the AI run after this long")
	return cmd
}

func newPlanProposalCmd(e flowEnv) *cobra.Command {
	var run string
	cmd := &cobra.Command{
		Use: "proposal <request-id>", Short: "read a plan proposal run (its status and the proposed plan)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetPlanProposal(ctx, args[0], run)
			}, nil)
		},
	}
	cmd.Flags().StringVar(&run, "run", "", "run id (default: the latest)")
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
