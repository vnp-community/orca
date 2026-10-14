package command

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
)

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// renderRequest summarises the Request in a gateway {request: {...}} document.
func renderRequest(doc apiclient.Document) string {
	r := doc.Doc("request")
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s  %s\n", r.Int("number"), r.Str("id"), r.Str("title"))
	fmt.Fprintf(&b, "status: %s   type: %s   size: %s   urgency: %s   version: %d", r.Str("status"), orDash(r.Str("type")),
		orDash(r.Str("size")), orDash(r.Str("urgency")), r.Int("version"))
	if plan := r.Str("planTaskId"); plan != "" {
		fmt.Fprintf(&b, "\nplan task: %s", plan)
	}
	if reason := r.Str("classificationReason"); reason != "" {
		fmt.Fprintf(&b, "\nclassification: %s", reason)
	}
	if runID := doc.Str("runId"); runID != "" {
		fmt.Fprintf(&b, "\nrun: %s", runID)
	}
	return b.String()
}

// renderRequestList prints one line per Request.
func renderRequestList(doc apiclient.Document) string {
	rows := doc.Docs("requests")
	if len(rows) == 0 {
		return "no requests"
	}
	lines := make([]string, 0, len(rows)+1)
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("#%-5d %-36s %-26s %-16s %s", r.Int("number"), r.Str("id"), r.Str("status"), orDash(r.Str("type")), r.Str("title")))
	}
	if next := doc.Str("nextPageToken"); next != "" {
		lines = append(lines, "more: --page-token "+next)
	}
	return strings.Join(lines, "\n")
}

// renderApprovals prints one line per approval gate; the digest is what a decider commits to.
func renderApprovals(doc apiclient.Document) string {
	rows := doc.Docs("approvals")
	if len(rows) == 0 {
		return "no approvals"
	}
	lines := make([]string, 0, len(rows))
	for _, a := range rows {
		lines = append(lines, fmt.Sprintf("%-36s %-9s %-9s v%-3d subject %s  %s", a.Str("id"), a.Str("subjectType"), a.Str("status"), a.Int("version"),
			orDash(a.Str("subjectId")), a.Str("requestTitle")))
	}
	return strings.Join(lines, "\n")
}

// renderApproval shows the single gate with its version and digest.
func renderApproval(doc apiclient.Document) string {
	a := doc.Doc("approval")
	if a == nil {
		a = apiclient.Document(doc)
	}
	return fmt.Sprintf("%s  %s  %s\nstatus: %s   stage: %s   version: %d\ndigest: %s", a.Str("id"), a.Str("subjectType"), a.Str("requestTitle"),
		a.Str("status"), a.Str("stage"), a.Int("version"), a.Str("subjectDigest"))
}

// renderSolutions prints each Solution with its options (the recommended one marked *) and the analysis runs.
func renderSolutions(doc apiclient.Document) string {
	var b strings.Builder
	for _, s := range doc.Docs("solutions") {
		fmt.Fprintf(&b, "solution %s  status: %s\n", s.Str("id"), s.Str("status"))
		for _, o := range s.Docs("options", "options") {
			mark := " "
			if o.Str("recommended") == "true" {
				mark = "*"
			}
			fmt.Fprintf(&b, "  %s option %s  %s\n", mark, o.Str("id"), o.Str("title"))
			if sum := o.Str("summary"); sum != "" {
				fmt.Fprintf(&b, "      %s\n", sum)
			}
		}
		if reason := s.Str("options", "recommendation", "reason"); reason != "" {
			fmt.Fprintf(&b, "  why recommended: %s\n", reason)
		}
	}
	for _, r := range doc.Docs("runs") {
		fmt.Fprintf(&b, "run %s  %s  %s %s\n", r.Str("id"), r.Str("kind"), r.Str("status"), r.Str("errorCode"))
	}
	if b.Len() == 0 {
		return "no solutions"
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderChosenOption says what the choice did and, for a high-risk option, why approval will still be refused.
func renderChosenOption(doc apiclient.Document) string {
	line := fmt.Sprintf("solution %s: option %d %s; approval digest %s", doc.Str("solution", "id"), doc.Int("solution", "chosenOption"),
		doc.Str("decisionStatus"), doc.Str("approvalDigest"))
	if doc.Bool("requiresConfirmation") {
		line += "\nwarning: this option is high risk and must be confirmed by typing its title; the gateway has no confirmation " +
			"channel yet (TASK-FE-E2E-028), so approval is refused. Choose the recommended option or confirm in the UI."
	}
	return line
}
