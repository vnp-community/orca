package command

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newApprovalCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "approval", Short: "approval gates (solution, plan, phase): list, inspect, approve, reject"}
	cmd.AddCommand(newApprovalListCmd(e), newApprovalPendingCmd(e), newApprovalShowCmd(e),
		newApprovalDecideCmd(e, "approve"), newApprovalDecideCmd(e, "reject"))
	return cmd
}

func newApprovalListCmd(e flowEnv) *cobra.Command {
	var subject, status string
	cmd := &cobra.Command{
		Use: "list <request-id>", Short: "list the approval gates of a Request", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			if subject != "" {
				q.Set("subjectType", subject)
			}
			if status != "" {
				q.Set("status", status)
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListRequestApprovals(ctx, args[0], q)
			}, renderApprovals)
		},
	}
	cmd.Flags().StringVar(&subject, "subject", "", "solution, plan, phase ...")
	cmd.Flags().StringVar(&status, "status", "", "pending, approved, rejected ...")
	return cmd
}

func newApprovalPendingCmd(e flowEnv) *cobra.Command {
	var subject string
	cmd := &cobra.Command{
		Use: "pending", Short: "gates waiting for you across all Requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			if subject != "" {
				q.Set("subjectType", subject)
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListPendingApprovals(ctx, q)
			}, renderApprovals)
		},
	}
	cmd.Flags().StringVar(&subject, "subject", "", "solution, plan, phase ...")
	return cmd
}

func newApprovalShowCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "show <approval-id>", Short: "show a gate with its version and digest", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.GetApproval(ctx, args[0])
			}, renderApproval)
		},
	}
}

// newApprovalDecideCmd serves approve and reject. The service decides only against the version and digest the person
// saw; without --version/--digest the CLI reads them just before deciding and says so on stderr.
func newApprovalDecideCmd(e flowEnv, decision string) *cobra.Command {
	var comment, digest, executionMode string
	var version int64
	short := map[string]string{"approve": "approve a gate", "reject": "reject a gate (a comment is required)"}[decision]
	cmd := &cobra.Command{
		Use: decision + " <approval-id>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if decision == "reject" && comment == "" {
				return e.usage("--comment is required to reject")
			}
			if (version == 0) != (digest == "") {
				return e.usage("give --version and --digest together, or neither")
			}
			mode, err := decideExecutionMode(decision, executionMode)
			if err != nil {
				return e.usage("%s", err.Error())
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				v, d := version, digest
				if v == 0 {
					gate, err := cli.GetApproval(ctx, args[0])
					if err != nil {
						return nil, err
					}
					a := gate.Doc("approval")
					v, d = a.Int("version"), a.Str("subjectDigest")
					fmt.Fprintf(os.Stderr, "%s %s gate %s at version %d, digest %s\n", decision, a.Str("subjectType"), args[0], v, d)
				}
				body := apiclient.Document{"version": v, "digest": d, "comment": comment}
				if mode != "" {
					body["executionMode"] = mode
				}
				return cli.DecideApproval(ctx, args[0], decision, body)
			}, nil)
		},
	}
	cmd.Flags().StringVar(&comment, "comment", "", "a note recorded with the decision")
	cmd.Flags().Int64Var(&version, "version", 0, "the gate version you saw")
	cmd.Flags().StringVar(&digest, "digest", "", "the gate digest you saw")
	cmd.Flags().StringVar(&executionMode, "execution-mode", "", `approve the plan and run every Phase without asking again: "continuous"`)
	if decision == "reject" {
		_ = cmd.Flags().MarkHidden("execution-mode") // present only to refuse it with a clear message
	}
	return cmd
}

// decideExecutionMode checks the flag locally so a typo is a usage error; the service stays the source of truth for the rest.
func decideExecutionMode(decision, raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case mode == "":
		return "", nil
	case decision != "approve":
		return "", errors.New("--execution-mode applies to approve only")
	case mode != "continuous":
		return "", fmt.Errorf("--execution-mode accepts only continuous, got %q", raw)
	}
	return mode, nil
}
