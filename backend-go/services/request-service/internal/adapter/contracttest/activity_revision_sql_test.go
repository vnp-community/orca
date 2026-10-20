package contracttest

import (
	"strings"
	"testing"
)

const revisionUnionOK = `
SELECT 'analysis.generated', id, 'x' FROM solutions WHERE tenant_id = ?
UNION ALL
-- the feedback text is never selected
SELECT 'analysis.revision_requested', CONCAT(id, ':revision'), started_at, 'user', '' FROM analysis_runs WHERE tenant_id = ? AND kind <> 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'
UNION ALL
SELECT 'plan.revision_requested', CONCAT(id, ':plan-revision'), started_at, 'user', '' FROM analysis_runs WHERE tenant_id = ? AND kind = 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'
UNION ALL
SELECT 'request.returned', id FROM request_return_history WHERE tenant_id = ?`

func TestActivityUnionFeedbackProblem(t *testing.T) {
	if p := activityUnionFeedbackProblem(revisionUnionOK); p != "" {
		t.Fatalf("a safe union is accepted, got %q", p)
	}
	cases := map[string]string{
		"selected in the revision branch": strings.Replace(revisionUnionOK, "started_at, 'user', ''", "started_at, feedback, ''", 1),
		"selected in the plan branch":     strings.Replace(revisionUnionOK, "':plan-revision'), started_at, 'user', ''", "':plan-revision'), started_at, 'user', feedback", 1),
		"plan branch missing":             strings.ReplaceAll(revisionUnionOK, "plan.revision_requested", "plan.other"),
		"analysis branch not kind-fenced": strings.Replace(revisionUnionOK, "AND kind <> 'plan' ", "", 1),
		"plan branch not kind-fenced":     strings.Replace(revisionUnionOK, "AND kind = 'plan' ", "", 1),
		"selected through the d column":   strings.Replace(revisionUnionOK, "'user', ''", "'user', LEFT(feedback, 10)", 1),
		"revision branch missing":         strings.ReplaceAll(revisionUnionOK, "analysis.revision_requested", "analysis.other"),
		"no non-empty filter":             strings.Replace(revisionUnionOK, "AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'", "", 1),
		"another branch names feedback":   strings.Replace(revisionUnionOK, "FROM request_return_history", ", feedback FROM request_return_history", 1),
	}
	for name, union := range cases {
		if activityUnionFeedbackProblem(union) == "" {
			t.Errorf("%s: must be refused", name)
		}
	}
}
