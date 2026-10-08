package contracttest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ExecutionEnv adds the CR-REQ-013/014/015 stores. Insert* bypass the repositories so the database's own CHECKs are probed.
type ExecutionEnv struct {
	LifecycleEnv
	Approvals        usecase.ApprovalRepository
	GateApprovals    usecase.ApprovalGateReader
	PhaseStarts      usecase.PhaseStartRepository
	Outcomes         usecase.TaskRunOutcomeRepository
	Checks           usecase.RequestCheckRepository
	Scanner          usecase.ExecutingRequestScanner
	Leases           usecase.ReconcileLeases
	Backlog          usecase.BacklogRequestReader
	Processed        usecase.ProcessedEventRepository
	InsertRawOutcome func(tenantID string, overrides map[string]any) error
	InsertRawCheck   func(tenantID string, overrides map[string]any) error
}

// RunExecutionRepositoryContract runs the repository scenarios of phase_starts, task_run_outcomes, request_checks,
// the reconcile scan and lease, and the backlog queries against a real database.
func RunExecutionRepositoryContract(t *testing.T, newEnv func(t *testing.T) ExecutionEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env ExecutionEnv)
	}{
		{"PhaseStarts_TryStart_Race_8Goroutines_OneInserted", phaseStartsRace},
		{"PhaseStarts_TenantIsolation", phaseStartsTenantIsolation},
		{"TaskRunOutcomes_InsertIsIdempotentOnEventID", outcomesIdempotent},
		{"TaskRunOutcomes_ContainerCompletesOnce", outcomesContainerOnce},
		{"TaskRunOutcomes_CountFailed", outcomesCountFailed},
		{"TaskRunOutcomes_LatestFailed_PicksNewest", outcomesLatestFailed},
		{"TaskRunOutcomes_LastEventAt", outcomesLastEventAt},
		{"TaskRunOutcomes_DispatchRetrySince", outcomesDispatchRetry},
		{"TaskRunOutcomes_CheckRejectsUnknownOutcome", outcomesRejectUnknown},
		{"TaskRunOutcomes_TenantIsolation", outcomesTenantIsolation},
		{"RequestChecks_Append_LatestWins", checksLatestWins},
		{"RequestChecks_MetricsRoundTrip", checksMetricsRoundTrip},
		{"RequestChecks_CheckConstraintRejectsUnknownKind", checksRejectUnknownKind},
		{"RequestChecks_TenantIsolation", checksTenantIsolation},
		{"Reconcile_ListQuietExecuting", reconcileScan},
		{"Reconcile_LeaseTwoReplicasOneWinner", reconcileLeaseRace},
		{"Reconcile_LeaseThrottlesByLastRun", reconcileLeaseThrottle},
		{"Backlog_ListReturnedRequests_Keyset_NoDuplicatesNoGaps", backlogKeyset},
		{"Backlog_InsertDuringPaging_StableOrder", backlogInsertDuringPaging},
		{"Backlog_FilterByTypeAndCategory_OnlyBacklogStatus", backlogFilters},
		{"Backlog_ListByStatus_ExecutingAndRequestID", backlogByStatus},
		{"Backlog_TenantIsolation", backlogTenantIsolation},
		{"Backlog_ParentRequestIDs_And_LatestReturnsUseHistory", backlogParentsAndReturns},
		{"GateApprovals_IncludeDecider_AndSubjects", gateApprovalsDecider},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func seedExecuting(t *testing.T, env ExecutionEnv, ctx context.Context, mod func(r *domain.Request)) domain.Request {
	t.Helper()
	return createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status, r.Type, r.Size = domain.RequestStatusExecuting, domain.RequestTypeChangeRequest, domain.RequestSizeM
		if mod != nil {
			mod(r)
		}
	})
}

func outcome(requestID, taskID string, o domain.Outcome, cause string, at time.Time) domain.TaskRunOutcome {
	return domain.TaskRunOutcome{
		ID: uuid.NewString(), RequestID: requestID, TaskID: taskID, Outcome: o, Cause: cause, EventID: uuid.NewString(), OccurredAt: at,
	}
}

func phaseStartsRace(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	phase := uuid.NewString()
	var wg sync.WaitGroup
	var inserted atomic.Int32
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := env.PhaseStarts.TryStart(ctx, domain.PhaseStart{TenantID: tenantID, PhaseTaskID: phase, RequestID: req.ID, StartedBy: uuid.NewString()})
			if err != nil {
				errs <- err
				return
			}
			if ok {
				inserted.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("TryStart: %v", err)
	}
	if inserted.Load() != 1 {
		t.Fatalf("exactly one caller may claim the phase, %d did", inserted.Load())
	}
	starts, err := env.PhaseStarts.ListByRequest(ctx, req.ID)
	if err != nil || len(starts) != 1 || starts[0].PhaseTaskID != phase || starts[0].StartedAt.IsZero() {
		t.Fatalf("got %+v %v", starts, err)
	}
}

func phaseStartsTenantIsolation(t *testing.T, env ExecutionEnv) {
	a, b := newTenant(), newTenant()
	reqA := seedExecuting(t, env, tenOf(a), nil)
	phase := uuid.NewString()
	if ok, err := env.PhaseStarts.TryStart(tenOf(a), domain.PhaseStart{TenantID: a, PhaseTaskID: phase, RequestID: reqA.ID, StartedBy: uuid.NewString()}); err != nil || !ok {
		t.Fatalf("tenant A: %v %v", ok, err)
	}
	starts, err := env.PhaseStarts.ListByRequest(tenOf(b), reqA.ID)
	if err != nil || len(starts) != 0 {
		t.Fatalf("tenant B must not see A's phase starts: %+v %v", starts, err)
	}
	reqB := seedExecuting(t, env, tenOf(b), nil)
	if ok, err := env.PhaseStarts.TryStart(tenOf(b), domain.PhaseStart{TenantID: b, PhaseTaskID: phase, RequestID: reqB.ID, StartedBy: uuid.NewString()}); err != nil || !ok {
		t.Fatalf("the same phase id under another tenant is a different claim: %v %v", ok, err)
	}
}

func outcomesIdempotent(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	o := outcome(req.ID, uuid.NewString(), domain.OutcomeFailed, domain.CauseExecutionFailed, usec())
	if ok, err := env.Outcomes.Insert(ctx, o); err != nil || !ok {
		t.Fatalf("first insert: %v %v", ok, err)
	}
	again := o
	again.ID = uuid.NewString() // a redelivery has a new row id but the same event id
	if ok, err := env.Outcomes.Insert(ctx, again); err != nil || ok {
		t.Fatalf("a repeated event id must be dropped silently: %v %v", ok, err)
	}
	if n, _ := env.Outcomes.CountFailed(ctx, o.TaskID); n != 1 {
		t.Fatalf("a redelivered failure counts once, got %d", n)
	}
}

func outcomesContainerOnce(t *testing.T, env ExecutionEnv) {
	ctx := tenOf(newTenant())
	req := seedExecuting(t, env, ctx, nil)
	phase := uuid.NewString()
	var inserted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := env.Outcomes.Insert(ctx, outcome(req.ID, phase, domain.OutcomePhaseDone, domain.CauseDerived, usec()))
			if err != nil {
				t.Error(err)
			}
			if ok {
				inserted.Add(1)
			}
		}()
	}
	wg.Wait()
	if inserted.Load() != 1 {
		t.Fatalf("a phase completes once however many writers race, %d rows inserted", inserted.Load())
	}
	if ok, _ := env.Outcomes.Exists(ctx, phase, domain.OutcomePhaseDone); !ok {
		t.Fatal("Exists must see phase_done")
	}
	if ok, _ := env.Outcomes.Exists(ctx, phase, domain.OutcomePlanDone); ok {
		t.Fatal("Exists is per outcome")
	}
	// A task may succeed many times (retry after a manual reopen): only containers are limited to one.
	task := uuid.NewString()
	for i := 0; i < 3; i++ {
		if ok, err := env.Outcomes.Insert(ctx, outcome(req.ID, task, domain.OutcomeSucceeded, domain.CauseUserUpdate, usec())); err != nil || !ok {
			t.Fatalf("succeeded #%d: %v %v", i, ok, err)
		}
	}
}

func outcomesCountFailed(t *testing.T, env ExecutionEnv) {
	ctx := tenOf(newTenant())
	req := seedExecuting(t, env, ctx, nil)
	task, other := uuid.NewString(), uuid.NewString()
	now := usec()
	for _, o := range []domain.TaskRunOutcome{
		outcome(req.ID, task, domain.OutcomeFailed, domain.CauseExecutionFailed, now),
		outcome(req.ID, task, domain.OutcomeFailed, domain.CauseRecovery, now.Add(time.Second)),
		outcome(req.ID, task, domain.OutcomeFailed, domain.CauseDispatchError, now.Add(2*time.Second)),
		outcome(req.ID, task, domain.OutcomeStarted, domain.CauseExecuteClaim, now),
		outcome(req.ID, other, domain.OutcomeFailed, domain.CauseExecutionFailed, now),
	} {
		if _, err := env.Outcomes.Insert(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := env.Outcomes.CountFailed(ctx, task); err != nil || n != 2 {
		t.Fatalf("two real failures, the dispatch error does not count: %d %v", n, err)
	}
}

func outcomesLatestFailed(t *testing.T, env ExecutionEnv) {
	ctx := tenOf(newTenant())
	req := seedExecuting(t, env, ctx, nil)
	a, b, none := uuid.NewString(), uuid.NewString(), uuid.NewString()
	now := usec()
	for i, msg := range []string{"first", "second", "third"} {
		o := outcome(req.ID, a, domain.OutcomeFailed, domain.CauseExecutionFailed, now.Add(time.Duration(i)*time.Minute))
		o.ErrorMessage = msg
		if _, err := env.Outcomes.Insert(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	ob := outcome(req.ID, b, domain.OutcomeFailed, domain.CauseRecovery, now)
	ob.ErrorMessage = "b failed"
	_, _ = env.Outcomes.Insert(ctx, ob)
	_, _ = env.Outcomes.Insert(ctx, outcome(req.ID, none, domain.OutcomeSucceeded, domain.CauseUserUpdate, now))
	got, err := env.Outcomes.LatestFailed(ctx, []string{a, b, none})
	if err != nil || len(got) != 2 || got[a].ErrorMessage != "third" || got[b].ErrorMessage != "b failed" {
		t.Fatalf("newest failure per task: %+v %v", got, err)
	}
	if empty, err := env.Outcomes.LatestFailed(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("no ids, no rows: %v %v", empty, err)
	}
}

func outcomesLastEventAt(t *testing.T, env ExecutionEnv) {
	ctx := tenOf(newTenant())
	req := seedExecuting(t, env, ctx, nil)
	if at, err := env.Outcomes.LastEventAt(ctx, req.ID); err != nil || !at.IsZero() {
		t.Fatalf("no events yet: %v %v", at, err)
	}
	base := usec().Add(-time.Hour)
	_, _ = env.Outcomes.Insert(ctx, outcome(req.ID, uuid.NewString(), domain.OutcomeStarted, domain.CauseExecuteClaim, base))
	_, _ = env.Outcomes.Insert(ctx, outcome(req.ID, uuid.NewString(), domain.OutcomeSucceeded, domain.CauseUserUpdate, base.Add(10*time.Minute)))
	at, err := env.Outcomes.LastEventAt(ctx, req.ID)
	if err != nil || !at.Equal(base.Add(10*time.Minute)) {
		t.Fatalf("got %v %v", at, err)
	}
}

func outcomesDispatchRetry(t *testing.T, env ExecutionEnv) {
	ctx := tenOf(newTenant())
	req := seedExecuting(t, env, ctx, nil)
	task := uuid.NewString()
	base := usec().Add(-time.Hour)
	if _, ok, err := env.Outcomes.DispatchRetrySince(ctx, task); err != nil || ok {
		t.Fatalf("no dispatch error yet: %v %v", ok, err)
	}
	for i := 0; i < 3; i++ {
		o := outcome(req.ID, task, domain.OutcomeFailed, domain.CauseDispatchError, base.Add(time.Duration(i)*time.Minute))
		o.ErrorMessage = "TASK_EXECUTE_NO_CONNECTION"
		_, _ = env.Outcomes.Insert(ctx, o)
	}
	since, ok, err := env.Outcomes.DispatchRetrySince(ctx, task)
	if err != nil || !ok || !since.Equal(base) {
		t.Fatalf("the streak began at the first error: %v %v %v", since, ok, err)
	}
	last, ok, err := env.Outcomes.LatestDispatchError(ctx, task)
	if err != nil || !ok || last.ErrorMessage != "TASK_EXECUTE_NO_CONNECTION" || !last.OccurredAt.Equal(base.Add(2*time.Minute)) {
		t.Fatalf("latest dispatch error: %+v %v %v", last, ok, err)
	}
	// A run that started ends the streak; later errors begin a new one.
	_, _ = env.Outcomes.Insert(ctx, outcome(req.ID, task, domain.OutcomeStarted, domain.CauseExecuteClaim, base.Add(10*time.Minute)))
	if _, ok, _ := env.Outcomes.DispatchRetrySince(ctx, task); ok {
		t.Fatal("after a started run the old errors no longer count")
	}
	_, _ = env.Outcomes.Insert(ctx, outcome(req.ID, task, domain.OutcomeFailed, domain.CauseDispatchError, base.Add(20*time.Minute)))
	if since, ok, _ := env.Outcomes.DispatchRetrySince(ctx, task); !ok || !since.Equal(base.Add(20*time.Minute)) {
		t.Fatalf("a new streak: %v %v", since, ok)
	}
}

func outcomesRejectUnknown(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req := seedExecuting(t, env, tenOf(tenantID), nil)
	good := map[string]any{"request_id": req.ID, "outcome": "started"}
	if err := env.InsertRawOutcome(tenantID, good); err != nil {
		t.Fatalf("baseline row must be valid: %v", err)
	}
	if err := env.InsertRawOutcome(tenantID, map[string]any{"request_id": req.ID, "outcome": "exploded"}); err == nil {
		t.Fatal("the outcome CHECK must reject an unknown value")
	}
	if err := env.InsertRawOutcome(tenantID, map[string]any{"request_id": req.ID, "outcome": "phase_done", "once": 2}); err == nil {
		t.Fatal("the once CHECK must reject values other than 1")
	}
}

func outcomesTenantIsolation(t *testing.T, env ExecutionEnv) {
	a, b := newTenant(), newTenant()
	req := seedExecuting(t, env, tenOf(a), nil)
	task := uuid.NewString()
	o := outcome(req.ID, task, domain.OutcomeFailed, domain.CauseExecutionFailed, usec())
	if _, err := env.Outcomes.Insert(tenOf(a), o); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.Outcomes.CountFailed(tenOf(b), task); n != 0 {
		t.Fatalf("tenant B counted A's failures: %d", n)
	}
	if ok, _ := env.Outcomes.Exists(tenOf(b), task, domain.OutcomeFailed); ok {
		t.Fatal("tenant B saw A's outcome")
	}
	if got, _ := env.Outcomes.LatestFailed(tenOf(b), []string{task}); len(got) != 0 {
		t.Fatal("tenant B saw A's latest failure")
	}
}

func recordCheck(t *testing.T, env ExecutionEnv, ctx context.Context, tenantID, requestID string, kind domain.CheckKind, status domain.CheckStatus, metrics string) domain.RequestCheck {
	t.Helper()
	got, err := env.Checks.Append(ctx, domain.RequestCheck{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: requestID, Kind: kind, Status: status, Metrics: json.RawMessage(metrics),
		Summary: "s", Source: domain.CheckSourceManual, RecordedBy: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("append check: %v", err)
	}
	return got
}

func checksLatestWins(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	first := recordCheck(t, env, ctx, tenantID, req.ID, domain.CheckTestsBefore, domain.CheckStatusFailed, `{"total":1}`)
	time.Sleep(5 * time.Millisecond)
	second := recordCheck(t, env, ctx, tenantID, req.ID, domain.CheckTestsBefore, domain.CheckStatusPassed, `{"total":2}`)
	other := recordCheck(t, env, ctx, tenantID, req.ID, domain.CheckTestsAfter, domain.CheckStatusPassed, `{"total":3}`)
	if first.CreatedAt.IsZero() || !second.CreatedAt.After(first.CreatedAt) {
		t.Fatalf("created_at comes from the database clock and orders rows: %v %v", first.CreatedAt, second.CreatedAt)
	}
	latest, ok, err := env.Checks.Latest(ctx, req.ID, domain.CheckTestsBefore)
	if err != nil || !ok || latest.ID != second.ID {
		t.Fatalf("the newest row of the kind wins: %+v %v %v", latest, ok, err)
	}
	if _, ok, _ := env.Checks.Latest(ctx, req.ID, domain.CheckOpsResult); ok {
		t.Fatal("no ops_result recorded")
	}
	all, err := env.Checks.ListByRequest(ctx, req.ID)
	if err != nil || len(all) != 3 || all[0].ID != first.ID || all[1].ID != second.ID || all[2].ID != other.ID {
		t.Fatalf("oldest first: %+v %v", all, err)
	}
	marked := domain.MarkEffective(all)
	if marked[0].Effective || !marked[1].Effective || !marked[2].Effective {
		t.Fatalf("effective flags: %+v", marked)
	}
}

func checksMetricsRoundTrip(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	in := `{"metrics":[{"name":"p95","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":20.5}],"method":"wrk \"quoted\" ünïcode"}`
	got := recordCheck(t, env, ctx, tenantID, req.ID, domain.CheckPerfBaseline, domain.CheckStatusPassed, in)
	var a, b any
	if err := json.Unmarshal([]byte(in), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got.Metrics, &b); err != nil || fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatalf("metrics changed in storage: %s", got.Metrics)
	}
	decoded, err := domain.DecodePerfBaseline(got.Metrics)
	if err != nil || decoded.Metrics[0].TargetChangePercent != 20.5 {
		t.Fatalf("stored JSON must stay decodable: %+v %v", decoded, err)
	}
	empty, err := env.Checks.Append(ctx, domain.RequestCheck{ID: uuid.NewString(), RequestID: req.ID, Kind: domain.CheckOpsResult, Status: domain.CheckStatusPassed, Source: domain.CheckSourceAgent})
	if err != nil || string(empty.Metrics) == "" || empty.Source != domain.CheckSourceAgent || empty.RecordedBy != "" || empty.TaskID != "" {
		t.Fatalf("a record without metrics stores {} and keeps NULL ids empty: %+v %v", empty, err)
	}
}

func checksRejectUnknownKind(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	req := seedExecuting(t, env, tenOf(tenantID), nil)
	base := map[string]any{"request_id": req.ID}
	if err := env.InsertRawCheck(tenantID, base); err != nil {
		t.Fatalf("baseline row must be valid: %v", err)
	}
	for name, ov := range map[string]map[string]any{
		"kind": {"kind": "bogus"}, "status": {"status": "maybe"}, "source": {"source": "oracle"},
	} {
		row := map[string]any{"request_id": req.ID}
		for k, v := range ov {
			row[k] = v
		}
		if err := env.InsertRawCheck(tenantID, row); err == nil {
			t.Errorf("the %s CHECK must reject its bad value", name)
		}
	}
	if err := env.InsertRawCheck(tenantID, map[string]any{"request_id": req.ID, "source": "orca_verified"}); err != nil {
		t.Errorf("orca_verified is a legal source for rows Orca writes itself: %v", err)
	}
}

func checksTenantIsolation(t *testing.T, env ExecutionEnv) {
	a, b := newTenant(), newTenant()
	req := seedExecuting(t, env, tenOf(a), nil)
	recordCheck(t, env, tenOf(a), a, req.ID, domain.CheckOpsResult, domain.CheckStatusPassed, `{}`)
	if rows, err := env.Checks.ListByRequest(tenOf(b), req.ID); err != nil || len(rows) != 0 {
		t.Fatalf("tenant B read A's checks: %+v %v", rows, err)
	}
	if _, ok, _ := env.Checks.Latest(tenOf(b), req.ID, domain.CheckOpsResult); ok {
		t.Fatal("tenant B read A's latest check")
	}
}

func reconcileScan(t *testing.T, env ExecutionEnv) {
	tenantA, tenantB := newTenant(), newTenant()
	old := usec().Add(-2 * time.Hour)
	mkOld := func(tenantID string, mod func(r *domain.Request)) domain.Request {
		return seedExecuting(t, env, tenOf(tenantID), func(r *domain.Request) {
			r.UpdatedAt = old
			if mod != nil {
				mod(r)
			}
		})
	}
	quietA := mkOld(tenantA, nil)
	quietB := mkOld(tenantB, nil) // another tenant: found in the same cross-tenant scan
	fresh := seedExecuting(t, env, tenOf(tenantA), nil)
	notExecuting := mkOld(tenantA, func(r *domain.Request) { r.Status = domain.RequestStatusPlanning })
	busy := mkOld(tenantA, nil)
	dispatchOnly := mkOld(tenantA, nil)
	_, _ = env.Outcomes.Insert(tenOf(tenantA), outcome(busy.ID, uuid.NewString(), domain.OutcomeStarted, domain.CauseExecuteClaim, usec()))
	_, _ = env.Outcomes.Insert(tenOf(tenantA), outcome(dispatchOnly.ID, uuid.NewString(), domain.OutcomeFailed, domain.CauseDispatchError, usec()))

	refs, err := env.Scanner.ListQuietExecuting(context.Background(), 5*time.Minute, 100)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	found := map[string]string{}
	for _, r := range refs {
		found[r.RequestID] = r.TenantID
	}
	for _, want := range []domain.Request{quietA, quietB, dispatchOnly} {
		if found[want.ID] != want.TenantID {
			t.Errorf("request %s (tenant %s) should be picked up, scan returned tenant %q", want.ID, want.TenantID, found[want.ID])
		}
	}
	for name, skip := range map[string]domain.Request{"recently updated": fresh, "not executing": notExecuting, "recent task activity": busy} {
		if _, ok := found[skip.ID]; ok {
			t.Errorf("%s request must not be picked up", name)
		}
	}
	if limited, _ := env.Scanner.ListQuietExecuting(context.Background(), 5*time.Minute, 1); len(limited) != 1 {
		t.Errorf("the limit applies, got %d", len(limited))
	}
}

func reconcileLeaseRace(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := env.Leases.Claim(ctx, req.ID, fmt.Sprintf("replica-%d", i), time.Minute, 0)
			if err != nil {
				t.Errorf("claim: %v", err)
			}
			if ok {
				winners.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("exactly one replica holds the lease, %d claimed it", winners.Load())
	}
	// A different tenant's request with the same id space is a different lease.
	other := newTenant()
	otherReq := seedExecuting(t, env, tenOf(other), nil)
	if ok, err := env.Leases.Claim(tenOf(other), otherReq.ID, "replica-x", time.Minute, 0); err != nil || !ok {
		t.Fatalf("an unrelated request is claimable: %v %v", ok, err)
	}
}

func reconcileLeaseThrottle(t *testing.T, env ExecutionEnv) {
	ctx := tenOf(newTenant())
	req := seedExecuting(t, env, ctx, nil)
	if ok, err := env.Leases.Claim(ctx, req.ID, "a", time.Minute, time.Hour); err != nil || !ok {
		t.Fatalf("first claim: %v %v", ok, err)
	}
	if ok, _ := env.Leases.Claim(ctx, req.ID, "b", time.Minute, time.Hour); ok {
		t.Fatal("a live lease blocks others")
	}
	if err := env.Leases.Release(ctx, req.ID, "b"); err != nil { // not the owner: no effect
		t.Fatal(err)
	}
	if ok, _ := env.Leases.Claim(ctx, req.ID, "b", time.Minute, time.Hour); ok {
		t.Fatal("only the owner can release")
	}
	if err := env.Leases.Release(ctx, req.ID, "a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := env.Leases.Claim(ctx, req.ID, "b", time.Minute, time.Hour); ok {
		t.Fatal("a request reconciled within the quiet window is not claimed again")
	}
	if ok, err := env.Leases.Claim(ctx, req.ID, "b", time.Minute, 0); err != nil || !ok {
		t.Fatalf("without a quiet window it is: %v %v", ok, err)
	}
	// The scan skips a request that was just reconciled.
	other := newTenant()
	oldReq := seedExecuting(t, env, tenOf(other), func(r *domain.Request) { r.UpdatedAt = usec().Add(-3 * time.Hour) })
	scanHas := func() bool {
		refs, _ := env.Scanner.ListQuietExecuting(context.Background(), 5*time.Minute, 500)
		for _, r := range refs {
			if r.RequestID == oldReq.ID {
				return true
			}
		}
		return false
	}
	if !scanHas() {
		t.Fatal("an old request is a candidate")
	}
	if ok, _ := env.Leases.Claim(tenOf(other), oldReq.ID, "a", time.Minute, 5*time.Minute); !ok {
		t.Fatal("claim")
	}
	_ = env.Leases.Release(tenOf(other), oldReq.ID, "a")
	if scanHas() {
		t.Fatal("a request reconciled a moment ago must not come back until the quiet window passes")
	}
}

func seedBacklog(t *testing.T, env ExecutionEnv, ctx context.Context, n int, mod func(i int, r *domain.Request)) []domain.Request {
	t.Helper()
	tenantID := tenantOfCtx(ctx)
	base := usec().Add(-24 * time.Hour)
	var out []domain.Request
	err := env.Tx.InTx(ctx, func(txCtx context.Context) error {
		for i := 0; i < n; i++ {
			r, err := domain.NewRequest(domain.NewRequestInput{TenantID: tenantID, ProjectID: uuid.NewString(), Title: fmt.Sprintf("backlog %d", i), SourceProvider: "manual", ReporterID: uuid.NewString()})
			if err != nil {
				return err
			}
			r.Status, r.Type, r.Size = domain.RequestStatusRequestBacklog, domain.RequestTypeBug, domain.RequestSizeS
			r.ReturnedFromStage, r.ReturnedCategory, r.ReturnReason = domain.ReturnStageTask, domain.ReturnCategoryOther, "r"
			r.UpdatedAt = base.Add(time.Duration(i/20) * time.Second) // many rows share an updated_at
			if mod != nil {
				mod(i, &r)
			}
			if r.Number, err = env.Requests.NextNumber(txCtx); err != nil {
				return err
			}
			if err := env.Requests.Create(txCtx, r); err != nil {
				return err
			}
			out = append(out, r)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed backlog: %v", err)
	}
	return out
}

func tenantOfCtx(ctx context.Context) string {
	id, _ := tenant.TenantID(ctx)
	return id
}

// pageAll walks the backlog with the keyset cursor the way ListBacklogRequests does.
func pageAll(t *testing.T, env ExecutionEnv, ctx context.Context, tenantID string, flt usecase.BacklogRequestFilter, between func(page int)) []string {
	t.Helper()
	var ids []string
	for page := 0; page < 100; page++ {
		rows, err := env.Backlog.ListReturnedRequests(ctx, tenantID, flt)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		more := len(rows) > flt.Limit
		if more {
			rows = rows[:flt.Limit]
		}
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		if !more {
			return ids
		}
		last := rows[len(rows)-1]
		flt.Cursor = &usecase.Cursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
		if between != nil {
			between(page)
		}
	}
	t.Fatal("paging did not terminate")
	return nil
}

func backlogKeyset(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	seeded := seedBacklog(t, env, ctx, 1000, nil)
	ids := pageAll(t, env, ctx, tenantID, usecase.BacklogRequestFilter{Limit: 100}, nil)
	if len(ids) != 1000 {
		t.Fatalf("1000 rows expected, got %d", len(ids))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("row %s returned twice", id)
		}
		seen[id] = true
	}
	for _, r := range seeded {
		if !seen[r.ID] {
			t.Fatalf("row %s was skipped", r.ID)
		}
	}
	// Order is updated_at DESC, id DESC.
	byID := map[string]domain.Request{}
	for _, r := range seeded {
		byID[r.ID] = r
	}
	sorted := sort.SliceIsSorted(ids, func(i, j int) bool {
		a, b := byID[ids[i]], byID[ids[j]]
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID > b.ID
	})
	if !sorted {
		t.Fatal("rows are not in updated_at DESC, id DESC order")
	}
}

func backlogInsertDuringPaging(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	seeded := seedBacklog(t, env, ctx, 250, nil)
	var intruder domain.Request
	ids := pageAll(t, env, ctx, tenantID, usecase.BacklogRequestFilter{Limit: 50}, func(page int) {
		if page == 1 {
			// A request that just landed in the backlog has the newest updated_at: it sorts before every cursor.
			intruder = seedBacklog(t, env, ctx, 1, func(_ int, r *domain.Request) { r.UpdatedAt = usec() })[0]
		}
	})
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("row %s returned twice", id)
		}
		seen[id] = true
	}
	for _, r := range seeded {
		if !seen[r.ID] {
			t.Fatalf("row %s skipped after a concurrent insert", r.ID)
		}
	}
	if seen[intruder.ID] {
		t.Fatal("a row newer than the cursor must not leak into a later page")
	}
}

func backlogFilters(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	proj := uuid.NewString()
	seeded := seedBacklog(t, env, ctx, 6, func(i int, r *domain.Request) {
		if i%2 == 0 {
			r.Type = domain.RequestTypeTask
		}
		if i < 2 {
			r.ReturnedCategory = domain.ReturnCategoryRejected
		}
		if i == 3 {
			r.ProjectID = proj
		}
	})
	executing := seedExecuting(t, env, ctx, nil) // never in the REQUEST view
	got := func(f usecase.BacklogRequestFilter) map[string]bool {
		f.Limit = 50
		out := map[string]bool{}
		for _, id := range pageAll(t, env, ctx, tenantID, f, nil) {
			out[id] = true
		}
		return out
	}
	if all := got(usecase.BacklogRequestFilter{}); len(all) != 6 || all[executing.ID] {
		t.Fatalf("only request_backlog rows: %v", all)
	}
	if tasks := got(usecase.BacklogRequestFilter{Types: []string{"task"}}); len(tasks) != 3 || !tasks[seeded[0].ID] || tasks[seeded[1].ID] {
		t.Fatalf("type filter: %v", tasks)
	}
	if rejected := got(usecase.BacklogRequestFilter{Categories: []string{"rejected"}}); len(rejected) != 2 || !rejected[seeded[1].ID] {
		t.Fatalf("category filter: %v", rejected)
	}
	if both := got(usecase.BacklogRequestFilter{Types: []string{"task"}, Categories: []string{"rejected"}}); len(both) != 1 || !both[seeded[0].ID] {
		t.Fatalf("combined filter: %v", both)
	}
	if byProject := got(usecase.BacklogRequestFilter{ProjectID: proj}); len(byProject) != 1 || !byProject[seeded[3].ID] {
		t.Fatalf("project filter: %v", byProject)
	}
	if none := got(usecase.BacklogRequestFilter{Types: []string{"hotfix"}}); len(none) != 0 {
		t.Fatalf("no match: %v", none)
	}
}

func backlogByStatus(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	running := seedExecuting(t, env, ctx, nil)
	waiting := createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status, r.Type, r.Size = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeBug, domain.RequestSizeS
	})
	done := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusCompleted, domain.RequestTypeBug })
	rows, err := env.Backlog.ListByStatus(ctx, tenantID, []domain.RequestStatus{domain.RequestStatusExecuting, domain.RequestStatusAwaitingPlanApproval}, usecase.BacklogRequestFilter{Limit: 10})
	if err != nil || len(rows) != 2 {
		t.Fatalf("got %d %v", len(rows), err)
	}
	got := map[string]bool{rows[0].ID: true, rows[1].ID: true}
	if !got[running.ID] || !got[waiting.ID] || got[done.ID] {
		t.Fatalf("status filter: %v", got)
	}
	one, err := env.Backlog.ListByStatus(ctx, tenantID, []domain.RequestStatus{domain.RequestStatusExecuting}, usecase.BacklogRequestFilter{RequestID: running.ID, Limit: 10})
	if err != nil || len(one) != 1 || one[0].ID != running.ID || one[0].Title == "" || one[0].Number == 0 || one[0].ReporterID == "" || one[0].Type != domain.RequestTypeChangeRequest {
		t.Fatalf("request_id filter and full columns: %+v %v", one, err)
	}
	if none, _ := env.Backlog.ListByStatus(ctx, tenantID, []domain.RequestStatus{domain.RequestStatusExecuting}, usecase.BacklogRequestFilter{RequestID: waiting.ID, Limit: 10}); len(none) != 0 {
		t.Fatal("request_id and status filters combine")
	}
}

func backlogTenantIsolation(t *testing.T, env ExecutionEnv) {
	a, b := newTenant(), newTenant()
	seedBacklog(t, env, tenOf(a), 5, nil)
	if ids := pageAll(t, env, tenOf(b), b, usecase.BacklogRequestFilter{Limit: 10}, nil); len(ids) != 0 {
		t.Fatalf("tenant B saw %d of A's backlog rows", len(ids))
	}
	mine := seedBacklog(t, env, tenOf(a), 1, nil)[0]
	if parents, _ := env.Backlog.ParentRequestIDs(tenOf(b), b, []string{mine.ID}); len(parents) != 0 {
		t.Fatal("tenant B read A's links")
	}
	if returns, _ := env.Backlog.LatestReturns(tenOf(b), b, []string{mine.ID}); len(returns) != 0 {
		t.Fatal("tenant B read A's return history")
	}
}

func backlogParentsAndReturns(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	kids := seedBacklog(t, env, ctx, 2, nil)
	parentA := seedExecuting(t, env, ctx, nil)
	parentB := seedExecuting(t, env, ctx, nil)
	for _, link := range []struct{ parent, child string }{{parentA.ID, kids[0].ID}, {parentB.ID, kids[0].ID}} {
		l, err := domain.NewRequestLink(link.parent, link.child, domain.LinkReasonRelatesTo, uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
		if err := env.Links.Insert(ctx, l); err != nil {
			t.Fatalf("link: %v", err)
		}
	}
	parents, err := env.Backlog.ParentRequestIDs(ctx, tenantID, []string{kids[0].ID, kids[1].ID})
	sort.Strings(parents[kids[0].ID])
	want := []string{parentA.ID, parentB.ID}
	sort.Strings(want)
	if err != nil || fmt.Sprint(parents[kids[0].ID]) != fmt.Sprint(want) || len(parents[kids[1].ID]) != 0 {
		t.Fatalf("parents: %v %v", parents, err)
	}

	// returned_at comes from the history, not from updated_at (which keeps moving).
	actor := uuid.NewString()
	first, second := usec().Add(-3*time.Hour), usec().Add(-2*time.Hour)
	for i, at := range []time.Time{first, second} {
		if err := env.Returns.Append(ctx, domain.ReturnHistoryEntry{
			RequestID: kids[0].ID, Action: domain.ReturnActionReturned, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther,
			Reason: fmt.Sprint("r", i), ActorID: actor, ActorKind: domain.ActorKindUser, At: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := env.Returns.Append(ctx, domain.ReturnHistoryEntry{RequestID: kids[0].ID, Action: domain.ReturnActionReopened, Reason: "later", ActorKind: domain.ActorKindUser, At: usec()}); err != nil {
		t.Fatal(err)
	}
	returns, err := env.Backlog.LatestReturns(ctx, tenantID, []string{kids[0].ID, kids[1].ID})
	if err != nil || len(returns) != 1 || !returns[kids[0].ID].At.Equal(second) || returns[kids[0].ID].ActorID != actor {
		t.Fatalf("latest 'returned' event only: %+v %v", returns, err)
	}
}

func gateApprovalsDecider(t *testing.T, env ExecutionEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	req := seedExecuting(t, env, ctx, nil)
	now := usec()
	mk := func(st domain.SubjectType, minutes int) domain.Approval {
		return domain.Approval{
			ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, SubjectType: st, SubjectID: uuid.NewString(), Stage: string(req.Status),
			Status: domain.ApprovalStatusPending, RequestedBy: uuid.NewString(), Version: 1, SubjectDigest: "d", SelfApprovalAllowed: true,
			CreatedAt: now.Add(time.Duration(minutes) * time.Minute), UpdatedAt: now.Add(time.Duration(minutes) * time.Minute),
		}
	}
	plan, phase, solution := mk(domain.SubjectPlan, 0), mk(domain.SubjectPhase, 1), mk(domain.SubjectSolution, 2)
	for _, a := range []domain.Approval{plan, phase, solution} {
		if err := env.Approvals.Insert(ctx, a); err != nil {
			t.Fatalf("insert %s: %v", a.SubjectType, err)
		}
	}
	decider := uuid.NewString()
	decided := plan
	if err := decided.Approve(decider, "ok", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ok, err := env.Approvals.UpdateDecision(ctx, decided, 1); err != nil || !ok {
		t.Fatalf("decide: %v %v", ok, err)
	}
	got, err := env.GateApprovals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil || len(got) != 2 {
		t.Fatalf("plan and phase, not the solution approval: %+v %v", got, err)
	}
	for _, a := range got {
		switch a.SubjectType {
		case domain.SubjectPlan:
			if a.Status != domain.ApprovalStatusApproved || a.DecidedBy == nil || *a.DecidedBy != decider || a.DecidedAt == nil {
				t.Fatalf("the approver identity drives who executes: %+v", a)
			}
		case domain.SubjectPhase:
			if a.DecidedBy != nil || a.DecidedAt != nil {
				t.Fatalf("a pending approval has no decider: %+v", a)
			}
		default:
			t.Fatalf("unexpected subject %s", a.SubjectType)
		}
	}
	if none, _ := env.GateApprovals.ListGateApprovals(tenOf(newTenant()), tenantID, []string{req.ID}); len(none) != 0 {
		t.Fatal("a caller in another tenant gets nothing back")
	}
}
