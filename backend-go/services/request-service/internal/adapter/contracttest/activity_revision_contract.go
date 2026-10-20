package contracttest

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Scenarios for the activity line analysis.revision_requested (TASK-REQ-041-03): one line per analysis run that carries
// reviewer feedback, with the feedback itself never read into the feed.

// revisionFeedbackMarker is written into every revision run so a leak anywhere in a row shows up as a substring.
const revisionFeedbackMarker = "REVISION-FEEDBACK-MUST-NOT-LEAK"

var (
	sqlLineComment      = regexp.MustCompile(`(?m)--[^\n]*`)
	feedbackPredicateRE = regexp.MustCompile(`(?i)feedback\s+IS\s+NOT\s+NULL\s+AND\s+feedback\s*<>\s*''\s+AND\s+feedback\s+NOT\s+LIKE\s+'clarification:%'`)
)

// AssertActivityUnionOmitsFeedback reads the text of a dialect's activity UNION: the revision branch must filter on the
// feedback column and no other part of the statement may name it, so the text cannot reach a selected column (C9).
func AssertActivityUnionOmitsFeedback(t *testing.T, union string) {
	t.Helper()
	if problem := activityUnionFeedbackProblem(union); problem != "" {
		t.Fatal(problem)
	}
}

// activityUnionFeedbackProblem is "" when the union is safe; split from the assertion so the check itself can be tested.
// The analysis and the plan branch share the run table, so each must also keep to its own kind.
func activityUnionFeedbackProblem(union string) string {
	code := sqlLineComment.ReplaceAllString(union, "")
	for _, b := range []struct{ kind, kindPredicate string }{
		{domain.ActivityAnalysisRevisionRequested, "kind <> 'plan'"},
		{domain.ActivityPlanRevisionRequested, "kind = 'plan'"},
	} {
		if problem := revisionBranchProblem(code, b.kind, b.kindPredicate); problem != "" {
			return problem
		}
	}
	if rest := feedbackPredicateRE.ReplaceAllString(code, ""); strings.Contains(strings.ToLower(rest), "feedback") {
		return fmt.Sprintf("the activity union names the feedback column outside the revision filters: %q", rest)
	}
	return ""
}

func revisionBranchProblem(code, kind, kindPredicate string) string {
	start := strings.Index(code, "'"+kind+"'")
	if start < 0 {
		return "the activity union has no " + kind + " branch"
	}
	branch := code[start:]
	if end := strings.Index(branch, "UNION ALL"); end >= 0 {
		branch = branch[:end]
	}
	from := strings.Index(strings.ToUpper(branch), " FROM ")
	if from < 0 {
		return fmt.Sprintf("the %s branch has no FROM: %q", kind, branch)
	}
	if strings.Contains(strings.ToLower(branch[:from]), "feedback") {
		return fmt.Sprintf("the %s branch selects feedback: %q", kind, branch[:from])
	}
	if !feedbackPredicateRE.MatchString(branch[from:]) {
		return fmt.Sprintf("the %s branch must keep only runs with non-empty feedback: %q", kind, branch[from:])
	}
	if !strings.Contains(branch[from:], kindPredicate) {
		return fmt.Sprintf("the %s branch must filter on %s: %q", kind, kindPredicate, branch[from:])
	}
	return ""
}

// revisionRun is the analysis_runs row of one round; a nil feedback is the first, unrevised analysis.
func revisionRun(tenantID, requestID, solutionID, actorID string, feedback any, at time.Time) map[string]any {
	row := map[string]any{
		"id": uuid.NewString(), "tenant_id": tenantID, "request_id": requestID, "kind": "solution", "mode": "complete", "status": "succeeded",
		"started_at": at, "feedback": feedback,
	}
	if solutionID != "" {
		row["solution_id"] = solutionID
	}
	if actorID != "" {
		row["actor_id"] = actorID
	}
	return row
}

func activityRevisionRequested(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req, _, t0 := seedActivity(t, env, tenantID)
	ctx := tenOf(tenantID)
	at := func(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }
	alice, bob := uuid.NewString(), uuid.NewString()
	sol1, sol2, sol3 := uuid.NewString(), uuid.NewString(), uuid.NewString()

	// The first analysis and runs with a null or empty feedback are not revisions.
	initial := revisionRun(tenantID, req.ID, sol1, alice, nil, at(2))
	blank := revisionRun(tenantID, req.ID, sol1, alice, "", at(3))
	// A run the system started after a clarification carries a marker, not a reviewer's ask.
	afterClarification := revisionRun(tenantID, req.ID, sol1, "", domain.ClarificationFeedbackPrefix+uuid.NewString(), at(4))
	// Rounds 2 and 3 start in the same instant, so the keyset tie-break is part of the scenario.
	r1 := revisionRun(tenantID, req.ID, sol1, alice, revisionFeedbackMarker+" one", at(7))
	r2 := revisionRun(tenantID, req.ID, sol2, bob, revisionFeedbackMarker+" two", at(9))
	r3 := revisionRun(tenantID, req.ID, sol3, alice, revisionFeedbackMarker+" three", at(9))
	// Another Request of the tenant and the same Request id under another tenant must not add lines.
	other, _, _ := seedActivity(t, env, tenantID)
	foreign := revisionRun(tenantID, other.ID, uuid.NewString(), alice, revisionFeedbackMarker+" other request", at(8))
	crossTenant := revisionRun(newTenant(), req.ID, uuid.NewString(), alice, revisionFeedbackMarker+" other tenant", at(8))
	for _, r := range []map[string]any{initial, blank, afterClarification, r1, r2, r3, foreign, crossTenant} {
		activityRaw(t, env, "analysis_runs", r)
	}

	read := func(q domain.ActivityQuery) []domain.ActivityRow {
		t.Helper()
		if q.Limit == 0 {
			q.Limit = 100
		}
		rows, err := env.Activity.ListActivity(ctx, req.ID, q)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	revisions := read(domain.ActivityQuery{Kinds: []string{domain.ActivityAnalysisRevisionRequested}})
	if len(revisions) != 3 {
		t.Fatalf("three revision rounds make exactly three lines, got %d: %+v", len(revisions), revisions)
	}
	want := map[string]map[string]any{r1["id"].(string): r1, r2["id"].(string): r2, r3["id"].(string): r3}
	for _, row := range revisions {
		run, ok := want[strings.TrimSuffix(row.ID, ":revision")]
		if !ok || !strings.HasSuffix(row.ID, ":revision") {
			t.Fatalf("line id is the run id plus :revision, got %q", row.ID)
		}
		started := run["started_at"].(time.Time)
		if !row.OccurredAt.Equal(started) || row.ActorKind != "user" || row.ActorID != run["actor_id"] || row.SubjectType != "solution" || row.SubjectID != run["solution_id"] {
			t.Fatalf("line of run %v: %+v", run["id"], row)
		}
		if row.A != "" || row.B != "" || row.C != "" || row.D != "" || row.N != 0 {
			t.Fatalf("the generic columns stay empty: %+v", row)
		}
		delete(want, strings.TrimSuffix(row.ID, ":revision"))
	}
	if !revisions[0].OccurredAt.Equal(revisions[1].OccurredAt) || !revisions[1].OccurredAt.After(revisions[2].OccurredAt) || revisions[2].ID != r1["id"].(string)+":revision" {
		t.Fatalf("newest first, the two simultaneous rounds before the oldest: %+v", revisions)
	}

	// Nothing in the feed carries the feedback: not in a row, not in the entry a screen receives.
	all := read(domain.ActivityQuery{})
	for _, row := range all {
		entry := domain.BuildActivityEntry(row, nil, 2)
		if strings.Contains(fmt.Sprintf("%+v", row), revisionFeedbackMarker) || strings.Contains(fmt.Sprintf("%+v", entry), revisionFeedbackMarker) {
			t.Fatalf("feedback leaked into the feed: %+v", row)
		}
		if row.Kind == domain.ActivityAnalysisRevisionRequested && (entry.Summary != "Requested changes to the analysis" || entry.Severity != domain.ActivityInfo) {
			t.Fatalf("a revision line has a fixed summary and info severity: %+v", entry)
		}
	}

	// Keyset paging reads the same lines once each, with and without the kind filter, even across the tie.
	pageAll := func(q domain.ActivityQuery, size int) []string {
		var ids []string
		var before *domain.ActivityCursor
		for i := 0; i < 40; i++ {
			q.Limit, q.Before = size, before
			page := read(q)
			for _, r := range page {
				ids = append(ids, r.ID)
			}
			if len(page) < size {
				break
			}
			last := page[len(page)-1]
			before = &domain.ActivityCursor{At: last.OccurredAt, ID: last.ID}
		}
		return ids
	}
	onlyRevisions := domain.ActivityQuery{Kinds: []string{domain.ActivityAnalysisRevisionRequested}}
	if got, one := pageAll(onlyRevisions, 1), pageAll(onlyRevisions, 2); fmt.Sprint(got) != fmt.Sprint(one) || len(got) != 3 || got[0] != revisions[0].ID || got[1] != revisions[1].ID || got[2] != revisions[2].ID {
		t.Fatalf("paging the revision lines by 1 and by 2 must agree with one read: %v %v", got, one)
	}
	var oneRead []string
	for _, r := range all {
		oneRead = append(oneRead, r.ID)
	}
	if got := pageAll(domain.ActivityQuery{}, 4); fmt.Sprint(got) != fmt.Sprint(oneRead) {
		t.Fatalf("paging the whole feed by 4 must agree with one read:\n got %v\nwant %v", got, oneRead)
	}
	if len(all) != 15+3 {
		t.Fatalf("the seeded feed plus three revision lines, got %d", len(all))
	}

	// The other filters reach the new lines too.
	if got := read(domain.ActivityQuery{SubjectID: sol2}); len(got) != 1 || got[0].ID != r2["id"].(string)+":revision" {
		t.Fatalf("subject filter on the solution: %+v", got)
	}
	if got := read(domain.ActivityQuery{Kinds: []string{domain.ActivityAnalysisRevisionRequested, domain.ActivityAnalysisGenerated}}); len(got) != 4 {
		t.Fatalf("kinds filter mixing the new kind with another: %d rows", len(got))
	}
	since := at(8)
	if got := read(domain.ActivityQuery{Kinds: onlyRevisions.Kinds, Since: &since}); len(got) != 2 {
		t.Fatalf("since filter: %d rows, want the two simultaneous rounds", len(got))
	}
	if got := read(domain.ActivityQuery{Kinds: onlyRevisions.Kinds, ActorKind: "agent"}); len(got) != 0 {
		t.Fatalf("revision lines are made by a user, not an agent: %d", len(got))
	}
}

// A run stores no solution or actor until its worker fills them in; the line must still read, with empty ids.
func activityRevisionWithoutSolutionOrActor(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req := seedExecuting(t, env, tenOf(tenantID), nil)
	run := revisionRun(tenantID, req.ID, "", "", revisionFeedbackMarker, usec().Add(-time.Hour))
	activityRaw(t, env, "analysis_runs", run)
	rows, err := env.Activity.ListActivity(tenOf(tenantID), req.ID, domain.ActivityQuery{Kinds: []string{domain.ActivityAnalysisRevisionRequested}, Limit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatalf("one line: %d %v", len(rows), err)
	}
	if r := rows[0]; r.ActorID != "" || r.SubjectID != "" || r.ActorKind != "user" || r.SubjectType != "solution" || r.ID != run["id"].(string)+":revision" {
		t.Fatalf("empty ids, not an error: %+v", r)
	}
}

// A Plan run keeps its feedback in the same column as the analysis runs: it must read as a plan line, never as an analysis one.
func activityPlanRevisionRequested(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req := seedExecuting(t, env, tenOf(tenantID), nil)
	ctx := tenOf(tenantID)
	t0 := usec().Add(-3 * time.Hour)
	alice := uuid.NewString()
	planRun := func(feedback any, actor string, at time.Time) map[string]any {
		row := revisionRun(tenantID, req.ID, "", actor, feedback, at)
		row["kind"] = "plan"
		return row
	}
	analysis := revisionRun(tenantID, req.ID, uuid.NewString(), alice, revisionFeedbackMarker+" analysis", t0)
	first := planRun(nil, alice, t0.Add(time.Minute))
	blank := planRun("", alice, t0.Add(2*time.Minute))
	system := planRun(domain.ClarificationFeedbackPrefix+uuid.NewString(), "", t0.Add(3*time.Minute))
	revised := planRun(revisionFeedbackMarker+" plan", alice, t0.Add(4*time.Minute))
	noActor := planRun(revisionFeedbackMarker+" plan again", "", t0.Add(5*time.Minute))
	otherTenant := planRun(revisionFeedbackMarker+" foreign", alice, t0.Add(6*time.Minute))
	otherTenant["tenant_id"] = newTenant()
	for _, r := range []map[string]any{analysis, first, blank, system, revised, noActor, otherTenant} {
		activityRaw(t, env, "analysis_runs", r)
	}

	read := func(kinds ...string) []domain.ActivityRow {
		t.Helper()
		rows, err := env.Activity.ListActivity(ctx, req.ID, domain.ActivityQuery{Kinds: kinds, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	plans := read(domain.ActivityPlanRevisionRequested)
	if len(plans) != 2 || plans[0].ID != noActor["id"].(string)+":plan-revision" || plans[1].ID != revised["id"].(string)+":plan-revision" {
		t.Fatalf("two Plan rounds with feedback make two plan lines, newest first: %+v", plans)
	}
	for _, row := range plans {
		if row.ActorKind != "user" || row.SubjectType != "plan" || row.SubjectID != "" || row.A != "" || row.B != "" || row.C != "" || row.D != "" || row.N != 0 {
			t.Fatalf("a plan line has fixed columns: %+v", row)
		}
	}
	if plans[1].ActorID != alice || plans[0].ActorID != "" {
		t.Fatalf("the actor is the run's: %+v", plans)
	}
	analyses := read(domain.ActivityAnalysisRevisionRequested)
	if len(analyses) != 1 || analyses[0].ID != analysis["id"].(string)+":revision" || analyses[0].SubjectType != "solution" {
		t.Fatalf("the Plan runs must not appear as analysis lines: %+v", analyses)
	}
	for _, row := range read() {
		entry := domain.BuildActivityEntry(row, nil, 2)
		if strings.Contains(fmt.Sprintf("%+v", row), revisionFeedbackMarker) || strings.Contains(fmt.Sprintf("%+v", entry), revisionFeedbackMarker) {
			t.Fatalf("feedback leaked into the feed: %+v", row)
		}
		if row.Kind == domain.ActivityPlanRevisionRequested && (entry.Summary != "Requested changes to the plan" || entry.Severity != domain.ActivityInfo) {
			t.Fatalf("a plan line has a fixed summary and info severity: %+v", entry)
		}
	}
	if both := read(domain.ActivityPlanRevisionRequested, domain.ActivityAnalysisRevisionRequested); len(both) != 3 {
		t.Fatalf("both kinds in one filter: %d", len(both))
	}
}
