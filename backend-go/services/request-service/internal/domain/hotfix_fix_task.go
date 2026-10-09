package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/apperrors"
)

const hotfixTitlePrefix = "Hotfix: "

// HotfixFixProposal is the single fix task of a hotfix, built from its diagnosis. A hotfix has no Plan stage, so
// this is the only place its task content is decided; the same validation as a committed Plan runs on it.
func HotfixFixProposal(req Request, diag DiagnosisDocument, maxTitleRunes int) PlanProposal {
	title := truncateRunes(hotfixTitlePrefix+strings.TrimSpace(req.Title), maxTitleRunes)
	summary := strings.TrimSpace(diag.Summary)
	if summary == "" {
		summary = strings.TrimSpace(diag.RootCause.Statement)
	}
	return PlanProposal{
		Title:   title,
		Summary: truncateRunes(summary, planProposalSummaryN),
		Tasks: []TaskProposal{{
			Title:       title,
			Description: truncateRunes(hotfixFixDescription(diag), maxPlanTextRunes),
			TaskType:    "bug",
		}},
	}
}

// hotfixFixDescription is what the agent reads as the task description: the scope rule first, then the diagnosis.
func hotfixFixDescription(d DiagnosisDocument) string {
	var b strings.Builder
	b.WriteString("Apply the smallest safe change that fixes the production issue below. Do not refactor or widen the scope.\n\n")
	fmt.Fprintf(&b, "Root cause (confidence %.2f): %s\n", d.RootCause.Confidence, strings.TrimSpace(d.RootCause.Statement))
	fmt.Fprintf(&b, "Impact: severity %s", d.Impact.Severity)
	if d.Impact.UserFacing {
		b.WriteString(", user facing")
	}
	if d.Impact.DataRisk {
		b.WriteString(", data at risk")
	}
	if len(d.Impact.AffectedComponents) > 0 {
		b.WriteString("; components: " + strings.Join(d.Impact.AffectedComponents, ", "))
	}
	b.WriteString("\n")
	if len(d.FixDirections) > 0 {
		b.WriteString("\nFix directions:\n")
		for _, f := range d.FixDirections {
			fmt.Fprintf(&b, "- %s", strings.TrimSpace(f.Summary))
			if f.Risk != "" {
				fmt.Fprintf(&b, " (risk: %s)", f.Risk)
			}
			b.WriteString("\n")
		}
	}
	if steps := d.Reproduction.Steps; len(steps) > 0 {
		b.WriteString("\nReproduction:\n")
		for i, s := range steps {
			fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(s))
		}
	}
	return strings.TrimSpace(b.String())
}

func truncateRunes(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// ErrHotfixFixTaskUnavailable means task-service could not store or look up the fix task; the analysis can be retried.
func ErrHotfixFixTaskUnavailable(cause error) error {
	return apperrors.New(apperrors.KindUnavailable, "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE", "task-service could not store the hotfix fix task", cause)
}
