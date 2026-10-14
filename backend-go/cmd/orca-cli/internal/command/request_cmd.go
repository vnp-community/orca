package command

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func newRequestCmd(e flowEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "request", Short: "create and drive Requests (the same steps as the UI)"}
	cmd.AddCommand(newRequestCreateCmd(e), newRequestListCmd(e), newRequestShowCmd(e), newRequestClassifyCmd(e),
		newRequestConfirmTypeCmd(e), newRequestCancelCmd(e), newRequestReopenCmd(e), newRequestWaitCmd(e), newRequestCleanupCmd(e))
	return cmd
}

func newRequestCreateCmd(e flowEnv) *cobra.Command {
	var project, title, body, bodyFile, clientID string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a Request in a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readBodyArg(body, bodyFile)
			if err != nil {
				return e.usage("%v", err)
			}
			if project == "" || strings.TrimSpace(title) == "" {
				return e.usage("--project and --title are required")
			}
			payload := apiclient.Document{"projectId": project, "title": title, "body": text}
			withOptional(payload, "clientRequestId", clientID)
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.CreateRequest(ctx, payload)
			}, renderRequest)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id")
	cmd.Flags().StringVar(&title, "title", "", "Request title")
	cmd.Flags().StringVar(&body, "body", "", "Request body (what to do)")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "read the body from a file (- for standard input)")
	cmd.Flags().StringVar(&clientID, "client-request-id", "", "idempotency key: the same key returns the same Request")
	return cmd
}

func newRequestListCmd(e flowEnv) *cobra.Command {
	var project, status, kind, pageToken string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list Requests (newest first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			for k, v := range map[string]string{"projectId": project, "status": status, "type": kind, "pageToken": pageToken} {
				if v != "" {
					q.Set(k, v)
				}
			}
			if limit > 0 {
				q.Set("pageSize", strconv.Itoa(limit))
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ListRequests(ctx, q)
			}, renderRequestList)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "only this project")
	cmd.Flags().StringVar(&status, "status", "", "comma-separated statuses, e.g. awaiting_analysis_approval,executing")
	cmd.Flags().StringVar(&kind, "type", "", "comma-separated types, e.g. change_request,bug")
	cmd.Flags().IntVar(&limit, "limit", 0, "page size (the service caps it)")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "continue a previous listing")
	return cmd
}

func newRequestShowCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "show <request-id>", Short: "show one Request", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				doc, err := cli.GetRequest(ctx, args[0])
				if err != nil {
					return nil, err
				}
				// What the agent runs of this Request did is part of knowing where it stands; a failed read must not hide the Request.
				if solutions, serr := cli.ListSolutions(ctx, args[0], nil); serr == nil {
					doc["analysisRuns"] = solutions["runs"]
				}
				if plan := doc.Str("request", "planTaskId"); plan != "" {
					if wb, werr := cli.GetPlanWritebacks(ctx, plan); werr == nil {
						doc["writebacks"] = wb
					}
				}
				return doc, nil
			}, renderRequestShow)
		},
	}
}

// renderRequestShow is renderRequest plus the latest analysis and plan runs with what their agent calls reported, what the
// agents said they wrote back to the design knowledge, and, once the Request is completed, what is left for a person.
func renderRequestShow(doc apiclient.Document) string {
	out := renderRequest(doc)
	if section := renderRuns(doc); section != "" {
		out += "\n" + section
	}
	if section := renderWritebacks(doc.Doc("writebacks")); section != "" {
		out += "\n" + section
	}
	if doc.Str("request", "status") == "completed" {
		out += "\n" + renderHandover(doc)
	}
	return out
}

func renderRuns(doc apiclient.Document) string {
	runs := doc.Docs("analysisRuns")
	if len(runs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("runs:")
	for i, r := range runs {
		if i == 3 {
			fmt.Fprintf(&b, "\n  ... %d older run(s)", len(runs)-3)
			break
		}
		fmt.Fprintf(&b, "\n  %s  %s  %s", r.Str("id"), firstOr(r.Str("kind"), "run"), r.Str("status"))
		if e := r.Str("engine"); e != "" {
			fmt.Fprintf(&b, "  engine %s", e)
		}
		if line := renderRunStats(r.Doc("stats"), 0); line != "" {
			fmt.Fprintf(&b, "\n      agent: %s", line)
		}
	}
	return b.String()
}

func newRequestClassifyCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "classify <request-id>", Short: "ask the AI to classify the Request (type, size, urgency)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.ClassifyRequest(ctx, args[0])
			}, renderRequest)
		},
	}
}

func newRequestConfirmTypeCmd(e flowEnv) *cobra.Command {
	var kind, size, urgency, reason string
	var version int64
	cmd := &cobra.Command{
		Use: "confirm-type <request-id>", Short: "confirm (or set) the type; the Request then moves on to analysis", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if kind == "" {
				return e.usage("--type is required")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				v, err := versionFor(ctx, cli, args[0], version)
				if err != nil {
					return nil, err
				}
				body := apiclient.Document{"type": kind, "expectedVersion": v}
				withOptional(body, "size", size)
				withOptional(body, "urgency", urgency)
				withOptional(body, "reason", reason)
				return cli.ConfirmRequestType(ctx, args[0], body)
			}, renderRequest)
		},
	}
	cmd.Flags().StringVar(&kind, "type", "", "question, bug, change_request, hotfix ...")
	cmd.Flags().StringVar(&size, "size", "", "S, M, L ...")
	cmd.Flags().StringVar(&urgency, "urgency", "", "urgency (hotfix needs urgent and a reason)")
	cmd.Flags().StringVar(&reason, "reason", "", "why (required for urgent)")
	cmd.Flags().Int64Var(&version, "expected-version", 0, "the Request version you saw (default: the current one)")
	return cmd
}

func newRequestCancelCmd(e flowEnv) *cobra.Command {
	var reason string
	var version int64
	cmd := &cobra.Command{
		Use: "cancel <request-id>", Short: "cancel the Request", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				v, err := versionFor(ctx, cli, args[0], version)
				if err != nil {
					return nil, err
				}
				return cli.CancelRequest(ctx, args[0], apiclient.Document{"reason": reason, "expectedVersion": v})
			}, renderRequest)
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why it is cancelled")
	cmd.Flags().Int64Var(&version, "expected-version", 0, "the Request version you saw (default: the current one)")
	return cmd
}

func newRequestReopenCmd(e flowEnv) *cobra.Command {
	var note string
	var version int64
	cmd := &cobra.Command{
		Use: "reopen <request-id>", Short: "reopen a Request from the backlog; it restarts at type confirmation", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				v, err := versionFor(ctx, cli, args[0], version)
				if err != nil {
					return nil, err
				}
				return cli.ReopenRequest(ctx, args[0], apiclient.Document{"note": note, "expectedVersion": v})
			}, renderRequest)
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "why it is reopened")
	cmd.Flags().Int64Var(&version, "expected-version", 0, "the Request version you saw (default: the current one)")
	return cmd
}

func newRequestWaitCmd(e flowEnv) *cobra.Command {
	var until string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use: "wait <request-id>", Short: "wait until the Request reaches a status (completed and cancelled always end the wait)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targets := map[string]bool{}
			for _, s := range strings.Split(until, ",") {
				if s = strings.TrimSpace(s); s != "" {
					targets[s] = true
				}
			}
			if len(targets) == 0 {
				return e.usage("--until is required, e.g. --until awaiting_analysis_approval,awaiting_plan_approval")
			}
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return defaultWaiter().requestStatus(ctx, cli, args[0], targets, timeout)
			}, renderRequest)
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "comma-separated statuses to wait for")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "give up after this long (exit 1, WAIT_TIMEOUT)")
	return cmd
}

// newRequestCleanupCmd removes the worktree of a Request's OpenSpec proposal. The branch and the stored change stay, so the
// plan can still be read with `orca plan bundle`.
func newRequestCleanupCmd(e flowEnv) *cobra.Command {
	return &cobra.Command{
		Use: "cleanup <request-id>", Short: "remove the worktree of the Request's OpenSpec proposal (the branch and the stored change stay)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.run(cmd, func(ctx context.Context, cli *apiclient.Client) (apiclient.Document, error) {
				return cli.CleanupProposal(ctx, args[0])
			}, renderProposalCleanup)
		},
	}
}

func renderProposalCleanup(doc apiclient.Document) string {
	branch := firstOr(doc.Str("branch"), "(no proposal branch)")
	switch {
	case doc.Bool("removed") && doc.Bool("abandoned"):
		return "removed the proposal worktree and marked the change abandoned; branch " + branch + " is kept"
	case doc.Bool("removed"):
		return "removed the proposal worktree; branch " + branch + " is kept"
	}
	return "no proposal worktree to remove (" + branch + ")"
}
