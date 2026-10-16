package contracttest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// The scenarios below cover CR-REQ-038 on a real database: the execution mode pinned on a Request and stored per project, the
// trigger of a Phase start, the outcome listing, and the activity feed that reads eight tables through one query.

func phaseStartTrigger(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	manual, auto, unset, starter := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, s := range []domain.PhaseStart{
		{TenantID: tenantID, PhaseTaskID: manual, RequestID: req.ID, StartedBy: starter, Trigger: domain.PhaseStartManual},
		{TenantID: tenantID, PhaseTaskID: auto, RequestID: req.ID, StartedBy: starter, Trigger: domain.PhaseStartAuto},
		{TenantID: tenantID, PhaseTaskID: unset, RequestID: req.ID, StartedBy: starter},
	} {
		if ok, err := env.PhaseStarts.TryStart(ctx, s); err != nil || !ok {
			t.Fatalf("TryStart: %v %v", ok, err)
		}
	}
	got := map[string]domain.PhaseStartTrigger{}
	starts, err := env.PhaseStarts.ListByRequest(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range starts {
		got[s.PhaseTaskID] = s.Trigger
	}
	if got[manual] != domain.PhaseStartManual || got[auto] != domain.PhaseStartAuto || got[unset] != domain.PhaseStartManual {
		t.Fatalf("triggers = %v: an unset trigger is stored as manual", got)
	}
}

func outcomesListByRequest(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req, other := seedExecuting(t, env, ctx, nil), seedExecuting(t, env, ctx, nil)
	task := uuid.NewString()
	now := usec()
	order := []domain.TaskRunOutcome{
		outcome(req.ID, task, domain.OutcomeStarted, domain.CauseExecuteClaim, now),
		outcome(req.ID, task, domain.OutcomeFailed, domain.CauseExecutionFailed, now.Add(time.Minute)),
		outcome(req.ID, task, domain.OutcomeStarted, domain.CauseExecuteClaim, now.Add(2*time.Minute)),
	}
	order[1].ErrorMessage = "execution failed: retryable TIMED_OUT"
	order[0].ContainerID = uuid.NewString()
	for _, o := range append([]domain.TaskRunOutcome{order[2], order[0], order[1]}, outcome(other.ID, uuid.NewString(), domain.OutcomeStarted, domain.CauseExecuteClaim, now)) {
		if _, err := env.Outcomes.Insert(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	got, err := env.Outcomes.ListByRequest(ctx, req.ID)
	if err != nil || len(got) != 3 {
		t.Fatalf("the Request's own outcomes only: %d %v", len(got), err)
	}
	for i, o := range got {
		if o.ID != order[i].ID {
			t.Fatalf("outcome %d = %s, want %s: oldest first", i, o.ID, order[i].ID)
		}
	}
	if got[0].ContainerID != order[0].ContainerID || got[1].ErrorMessage != "execution failed: retryable TIMED_OUT" || got[1].Outcome != domain.OutcomeFailed {
		t.Fatalf("fields round-trip: %+v", got)
	}
	if mine, _ := env.Outcomes.ListByRequest(tenOf(newTenant()), req.ID); len(mine) != 0 {
		t.Fatalf("another tenant must read nothing: %d", len(mine))
	}
	if none, err := env.Outcomes.ListByRequest(ctx, uuid.NewString()); err != nil || len(none) != 0 {
		t.Fatalf("an unknown request has no outcomes: %d %v", len(none), err)
	}
}

func pinExecutionMode(t *testing.T, env ExecutionEnv) {
	pinner, ok := env.Requests.(usecase.ExecutionModePinner)
	if !ok {
		t.Fatal("the request repository must pin execution modes")
	}
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	if got, _ := env.Requests.Get(ctx, req.ID); got.ExecutionMode != "" {
		t.Fatalf("a new Request has no mode yet: %q", got.ExecutionMode)
	}
	if got, err := pinner.PinExecutionModeIfUnset(ctx, req.ID, domain.ExecutionContinuous); err != nil || got != domain.ExecutionContinuous {
		t.Fatalf("first pin: %q %v", got, err)
	}
	if got, err := pinner.PinExecutionModeIfUnset(ctx, req.ID, domain.ExecutionStepByStep); err != nil || got != domain.ExecutionContinuous {
		t.Fatalf("the first mode wins: %q %v", got, err)
	}
	read, err := env.Requests.Get(ctx, req.ID)
	if err != nil || read.ExecutionMode != domain.ExecutionContinuous || read.Version != req.Version {
		t.Fatalf("read after pin: mode=%q version=%d (was %d): pinning is versionless %v", read.ExecutionMode, read.Version, req.Version, err)
	}
	if _, err := pinner.PinExecutionModeIfUnset(ctx, uuid.NewString(), domain.ExecutionContinuous); err == nil {
		t.Fatal("an unknown request is not found")
	}
	if _, err := pinner.PinExecutionModeIfUnset(tenOf(newTenant()), req.ID, domain.ExecutionStepByStep); err == nil {
		t.Fatal("another tenant cannot pin or read it")
	}
	if got, _ := env.Requests.Get(ctx, req.ID); got.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("a refused pin changed nothing: %q", got.ExecutionMode)
	}
}

// activityRaw inserts a row into a table that has no repository method in this package; ExecutionEnv.InsertRaw knows the dialect.
func activityRaw(t *testing.T, env ExecutionEnv, table string, row map[string]any) {
	t.Helper()
	if err := env.InsertRaw(table, row); err != nil {
		t.Fatalf("insert into %s: %v", table, err)
	}
}

// seedActivity writes one line from every source and returns the Request and the time of the earliest line.
func seedActivity(t *testing.T, env ExecutionEnv, tenantID string) (domain.Request, string, time.Time) {
	t.Helper()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	t0 := usec().Add(-2 * time.Hour)
	at := func(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }
	task := uuid.NewString()

	activityRaw(t, env, "request_revisions", map[string]any{
		"id": uuid.NewString(), "tenant_id": tenantID, "request_id": req.ID, "revision": 1, "cause": "created", "snapshot": "{}", "digest": "d",
		"actor_id": "reporter-1", "actor_kind": "user", "created_at": at(0),
	})
	if err := env.History.Append(ctx, domain.RequestTypeChange{RequestID: req.ID, ToType: domain.RequestTypeChangeRequest, ActorID: uuid.NewString(), ActorKind: domain.ActorKindAgent, Reason: "new capability", At: at(1)}); err != nil {
		t.Fatal(err)
	}
	activityRaw(t, env, "solutions", map[string]any{
		"id": uuid.NewString(), "tenant_id": tenantID, "request_id": req.ID, "options": "[]", "kind": "solution", "status": "approved", "seq": 3, "created_at": at(2),
	})
	activityRaw(t, env, "clarifications", map[string]any{
		"id": uuid.NewString(), "tenant_id": tenantID, "request_id": req.ID, "seq": 1, "source": "solution_open_question", "source_ref": "", "status": "answered",
		"resume_status": "analyzing", "round": 1, "asked_request_revision": 1, "due_at": at(600), "created_by": "agent", "created_at": at(3), "answered_at": at(4),
		"cancel_reason": "",
	})
	a := domain.Approval{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, SubjectType: domain.SubjectPlan, SubjectID: uuid.NewString(), Stage: "awaiting_plan_approval",
		Status: domain.ApprovalStatusPending, RequestedBy: "system", Version: 1, SubjectDigest: "d", SelfApprovalAllowed: true, CreatedAt: at(5), UpdatedAt: at(5),
	}
	if err := env.Approvals.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	decided := a
	if err := decided.Approve("approver-1", "looks right", at(6)); err != nil {
		t.Fatal(err)
	}
	if ok, err := env.Approvals.UpdateDecision(ctx, decided, 1); err != nil || !ok {
		t.Fatalf("decide: %v %v", ok, err)
	}
	phase := uuid.NewString()
	if _, err := env.PhaseStarts.TryStart(ctx, domain.PhaseStart{TenantID: tenantID, PhaseTaskID: phase, RequestID: req.ID, StartedBy: starterOf(tenantID), Trigger: domain.PhaseStartAuto}); err != nil {
		t.Fatal(err)
	}
	for i, o := range []domain.TaskRunOutcome{
		outcome(req.ID, task, domain.OutcomeStarted, domain.CauseExecuteClaim, at(10)),
		outcome(req.ID, task, domain.OutcomeFailed, domain.CauseExecutionFailed, at(11)),
		outcome(req.ID, task, domain.OutcomeFailed, domain.CauseDispatchError, at(12)),
		outcome(req.ID, task, domain.OutcomeStarted, domain.CauseExecuteClaim, at(13)),
		outcome(req.ID, task, domain.OutcomeSucceeded, domain.CauseExecutionCompleted, at(14)),
		outcome(req.ID, task, domain.OutcomeSucceeded, "user_update", at(15)),
	} {
		o.ErrorMessage = map[int]string{1: "execution failed: retryable TIMED_OUT", 2: "TASK_EXECUTE_NO_CONNECTION"}[i]
		if _, err := env.Outcomes.Insert(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	if err := env.Returns.Append(ctx, domain.ReturnHistoryEntry{
		RequestID: req.ID, Action: domain.ReturnActionReturned, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther,
		Reason: "task failed twice", ActorKind: domain.ActorKindSystem, At: at(20),
	}); err != nil {
		t.Fatal(err)
	}
	return req, task, t0
}

var activityStarters = map[string]string{}

// starterOf is the user (a uuid: phase_starts.started_by is one) that starts the Phase of a tenant's seeded activity.
func starterOf(tenantID string) string {
	if id, ok := activityStarters[tenantID]; ok {
		return id
	}
	id := uuid.NewString()
	activityStarters[tenantID] = id
	return id
}

func activityAllSources(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req, task, _ := seedActivity(t, env, tenantID)
	ctx := tenOf(tenantID)
	rows, err := env.Activity.ListActivity(ctx, req.ID, domain.ActivityQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, r := range rows {
		kinds = append(kinds, r.Kind)
	}
	// phase_starts.started_at is the database clock, so the Phase start is the newest line.
	want := []string{
		domain.ActivityPhaseStarted, domain.ActivityRequestReturned, domain.ActivityTaskAccepted, domain.ActivityTaskSucceeded, domain.ActivityTaskStarted,
		domain.ActivityTaskFailed, domain.ActivityTaskFailed, domain.ActivityTaskStarted, domain.ActivityApprovalDecided, domain.ActivityApprovalRequested,
		domain.ActivityClarifyAnswered, domain.ActivityClarifyAsked, domain.ActivityAnalysisGenerated, domain.ActivityRequestClassified, domain.ActivityRequestCreated,
	}
	if fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Fatalf("kinds newest first:\n got %v\nwant %v", kinds, want)
	}
	byKind := func(kind string) []domain.ActivityRow {
		var out []domain.ActivityRow
		for _, r := range rows {
			if r.Kind == kind {
				out = append(out, r)
			}
		}
		return out
	}
	// The attempt number counts the runs that used the retry budget, up to and including the line; a dispatch error that never
	// reached an agent has none.
	attempts := map[string][]int64{}
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].SubjectID == task {
			attempts[rows[i].Kind] = append(attempts[rows[i].Kind], rows[i].N)
		}
	}
	if fmt.Sprint(attempts[domain.ActivityTaskStarted]) != "[1 2]" || fmt.Sprint(attempts[domain.ActivityTaskFailed]) != "[1 0]" ||
		fmt.Sprint(attempts[domain.ActivityTaskSucceeded]) != "[2]" || fmt.Sprint(attempts[domain.ActivityTaskAccepted]) != "[2]" {
		t.Fatalf("attempt numbers: %v", attempts)
	}
	if r := byKind(domain.ActivityRequestClassified)[0]; r.B != "change_request" || r.C != "new capability" || r.ActorKind != "ai" {
		t.Fatalf("classification: %+v", r)
	}
	if r := byKind(domain.ActivityAnalysisGenerated)[0]; r.A != "solution" || r.B != "approved" || r.N != 3 {
		t.Fatalf("solution: %+v", r)
	}
	if r := byKind(domain.ActivityClarifyAsked)[0]; r.A != "solution_open_question" || r.B != "answered" || r.C != "analyzing" || r.N != 1 {
		t.Fatalf("clarification asked: %+v", r)
	}
	if r := byKind(domain.ActivityApprovalRequested)[0]; r.SubjectType != "plan" || r.A != "awaiting_plan_approval" || r.ActorKind != "system" {
		t.Fatalf("approval requested: %+v", r)
	}
	if r := byKind(domain.ActivityApprovalDecided)[0]; r.A != "approved" || r.C != "looks right" || r.ActorID != "approver-1" {
		t.Fatalf("approval decided: %+v", r)
	}
	if r := byKind(domain.ActivityPhaseStarted)[0]; r.A != "auto" || r.ActorKind != "system" || r.SubjectType != "phase" || r.ActorID != starterOf(tenantID) {
		t.Fatalf("phase started: %+v", r)
	}
	if r := byKind(domain.ActivityRequestReturned)[0]; r.A != "returned" || r.B != "task" || r.C != "other" || r.D != "task failed twice" {
		t.Fatalf("returned: %+v", r)
	}
	for _, r := range rows {
		if r.ID == "" || r.OccurredAt.IsZero() || r.ActorKind == "" {
			t.Fatalf("every line carries an id, a time and an actor kind: %+v", r)
		}
	}
	ids := map[string]bool{}
	for _, r := range rows {
		if ids[r.ID] {
			t.Fatalf("line ids are unique across sources: %s", r.ID)
		}
		ids[r.ID] = true
	}
}

func activityFiltersAndKeyset(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req, task, t0 := seedActivity(t, env, tenantID)
	ctx := tenOf(tenantID)
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
	if got := read(domain.ActivityQuery{Kinds: []string{domain.ActivityTaskFailed, domain.ActivityRequestReturned}}); len(got) != 3 {
		t.Fatalf("kinds filter: %d rows, want 2 failures and the return", len(got))
	}
	if got := read(domain.ActivityQuery{SubjectID: task}); len(got) != 6 {
		t.Fatalf("subject filter: %d rows, want the task's 6 lines", len(got))
	}
	if got := read(domain.ActivityQuery{ActorKind: "agent"}); len(got) != 6 {
		t.Fatalf("actor filter: %d rows, want the six task lines", len(got))
	}
	if got := read(domain.ActivityQuery{ActorKind: "ai"}); len(got) != 1 || got[0].Kind != domain.ActivityRequestClassified {
		t.Fatalf("the AI classifier is stored as agent but filters as ai: %+v", got)
	}
	since := t0.Add(10 * time.Minute)
	if got := read(domain.ActivityQuery{Since: &since}); len(got) != 8 {
		t.Fatalf("since filter: %d rows", len(got))
	}
	// Keyset paging in pages of 4 reads every line once, in order, with no repeats or gaps.
	all := read(domain.ActivityQuery{})
	var paged []domain.ActivityRow
	var before *domain.ActivityCursor
	for i := 0; i < 10; i++ {
		page := read(domain.ActivityQuery{Limit: 4, Before: before})
		paged = append(paged, page...)
		if len(page) < 4 {
			break
		}
		last := page[len(page)-1]
		before = &domain.ActivityCursor{At: last.OccurredAt, ID: last.ID}
	}
	if len(paged) != len(all) {
		t.Fatalf("paged %d rows, all %d", len(paged), len(all))
	}
	for i := range all {
		if paged[i].ID != all[i].ID {
			t.Fatalf("row %d differs between paging and one read: %s vs %s", i, paged[i].ID, all[i].ID)
		}
	}
	if got := read(domain.ActivityQuery{Limit: 2}); len(got) != 2 {
		t.Fatalf("limit: %d", len(got))
	}
}

func activityTenantIsolation(t *testing.T, env ExecutionEnv) {
	owner, other := newTenant(), newTenant()
	req, _, _ := seedActivity(t, env, owner)
	if rows, err := env.Activity.ListActivity(tenOf(other), req.ID, domain.ActivityQuery{Limit: 100}); err != nil || len(rows) != 0 {
		t.Fatalf("another tenant sees %d rows (%v)", len(rows), err)
	}
	if rows, err := env.Activity.ListActivity(tenOf(owner), uuid.NewString(), domain.ActivityQuery{Limit: 100}); err != nil || len(rows) != 0 {
		t.Fatalf("an unknown request has no activity: %d %v", len(rows), err)
	}
	if rows, err := env.Activity.ListActivity(tenOf(owner), "not-a-uuid", domain.ActivityQuery{Limit: 100}); err != nil || len(rows) != 0 {
		t.Fatalf("a malformed id reads nothing and does not fail: %d %v", len(rows), err)
	}
}

var _ = context.Background
