package command

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newDecisionCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "decision", Short: "the decisions recorded for a Request, and a person's confirmation of a high risk one"}
	cmd.AddCommand(newDecisionListCmd(e), newDecisionConfirmCmd(e))
	return cmd
}

func newDecisionListCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "list <request-id>", Short: "list the decisions of a Request", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListDecisions(ctx, args[0])
			}, renderDecisions)
		},
	}
}

func renderDecisions(doc apiclient.Document) string {
	var b strings.Builder
	for _, d := range doc.Docs("decisions") {
		b.WriteString(d.Str("id") + "  " + d.Str("subjectKind") + "  " + d.Str("status") + "  risk " + firstOr(d.Str("riskLevel"), "-") + "  chose " + firstOr(d.Str("chosenOptionId"), "-"))
		if t := d.Str("chosenOptionTitle"); t != "" {
			b.WriteString(": " + t)
		}
		if by := d.Str("confirmedBy"); by != "" {
			b.WriteString("  confirmed by " + by)
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "no decisions"
	}
	return strings.TrimRight(b.String(), "\n")
}

func newDecisionConfirmCmd(e flowEnv) *cobra.Command {
	var text string
	var version int64
	cmd := &cobra.Command{
		Use: "confirm <decision-id> --text <the chosen option's title>", Args: cobra.ExactArgs(1),
		Short: "confirm a high risk choice by typing the title of the option that was chosen (a person's act; the Solution gate stays closed until then)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(text) == "" {
				return e.usage("--text is required: type the title of the chosen option exactly as `orca decision list` shows it")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ConfirmDecision(ctx, args[0], apiclient.Document{"confirmationText": text, "expectedVersion": version})
			}, nil)
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "the chosen option's title (case and surrounding space do not matter)")
	cmd.Flags().Int64Var(&version, "expected-version", 0, "the decision version you saw (0 skips the check)")
	return cmd
}
