package command

import (
	"context"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newSolutionCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "solution", Short: "the AI's analysis of a Request: options to choose from"}
	cmd.AddCommand(newSolutionListCmd(e), newSolutionGenerateCmd(e), newSolutionChooseCmd(e))
	return cmd
}

func newSolutionListCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "list <request-id>", Short: "list the Solutions and analysis runs of a Request (newest first)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListSolutions(ctx, args[0], url.Values{})
			}, renderSolutions)
		},
	}
}

func newSolutionGenerateCmd(e flowEnv) *cobra.Command {
	var feedback, mode string
	var wait bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use: "generate <request-id>", Short: "start (or restart, with --feedback) the analysis", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := apiclient.Document{}
			withOptional(body, "feedback", feedback)
			withOptional(body, "analysisMode", mode)
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				started, err := cli.GenerateSolution(ctx, args[0], body)
				if err != nil || !wait {
					return started, err
				}
				return defaultWaiter().analysisRun(ctx, cli, args[0], started.Str("runId"), timeout)
			}, renderSolutions)
		},
	}
	cmd.Flags().StringVar(&feedback, "feedback", "", "what the next analysis should do differently")
	cmd.Flags().StringVar(&mode, "mode", "", "complete or agent_readonly")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for the analysis and print the Solutions")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "with --wait: give up after this long")
	return cmd
}

func newSolutionChooseCmd(e flowEnv) *cobra.Command {
	var option, rationale, comment string
	cmd := &cobra.Command{
		Use: "choose <request-id> <solution-id>", Short: "choose an option; then approve the solution gate with `orca approval approve`", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if option == "" {
				return e.usage("--option is required")
			}
			body := apiclient.Document{"optionId": option}
			withOptional(body, "rationale", rationale)
			withOptional(body, "comment", comment)
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ChooseSolutionOption(ctx, args[0], args[1], body)
			}, renderChosenOption)
		},
	}
	cmd.Flags().StringVar(&option, "option", "", "option id")
	cmd.Flags().StringVar(&rationale, "rationale", "", "why this option (required when it is not the recommended one)")
	cmd.Flags().StringVar(&comment, "comment", "", "a note for the approver")
	return cmd
}
