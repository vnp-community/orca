package contracttest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// OutboxRow is one outbox_events row read back for assertions.
type OutboxRow struct {
	Subject string
	Payload []byte
}

// LifecycleEnv adds what the CR-REQ-003/006 scenarios need beyond the repository Env.
// OutboxRows and InsertRawRequest are dialect specific, so each adapter's test file supplies them.
type LifecycleEnv struct {
	Env
	TxScope usecase.TxScope
	Outbox  usecase.OutboxWriter
	Returns usecase.ReturnHistoryRepository
	// OutboxRows lists the tenant's outbox rows in insertion order.
	OutboxRows func(t *testing.T, tenantID string) []OutboxRow
	// InsertRawRequest bypasses the use cases to prove the DB itself rejects invalid rows.
	InsertRawRequest func(tenantID string, overrides map[string]any) error
}

type noopCanceller struct{}

func (noopCanceller) CancelPending(context.Context, string, string) error { return nil }

type toggleGuard struct {
	mu     sync.Mutex
	active bool
}

func (g *toggleGuard) HasActiveExecution(context.Context, string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active, nil
}

type failingOutbox struct {
	usecase.OutboxWriter
	fail bool
}

func (f *failingOutbox) InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error {
	if f.fail {
		return errors.New("forced outbox failure")
	}
	return f.OutboxWriter.InsertOutboxEvent(ctx, ev)
}

type lifecycleUseCases struct {
	transition *usecase.TransitionRequest
	ret        *usecase.ReturnRequestToBacklog
	reopen     *usecase.ReopenRequest
	cancel     *usecase.CancelRequest
	spawn      *usecase.SpawnChildRequest
	guard      *toggleGuard
	outbox     *failingOutbox
}

func buildLifecycle(env LifecycleEnv) lifecycleUseCases {
	out := &failingOutbox{OutboxWriter: env.Outbox}
	guard := &toggleGuard{}
	tr := usecase.NewTransitionRequest(env.Requests, env.TxScope, out)
	creator := usecase.NewIdempotentChildCreator(env.Requests, env.Idempotency, out)
	return lifecycleUseCases{
		transition: tr,
		ret:        usecase.NewReturnRequestToBacklog(env.Requests, tr, env.Returns, noopCanceller{}, guard, env.Tx, out),
		reopen:     usecase.NewReopenRequest(env.Requests, tr, env.Returns, nil, env.Tx),
		cancel:     usecase.NewCancelRequest(env.Requests, tr, env.Returns, noopCanceller{}, guard, env.Tx),
		spawn:      usecase.NewSpawnChildRequest(env.Requests, env.Links, creator, env.Tx),
		guard:      guard,
		outbox:     out,
	}
}

// RunTransitionContract covers CR-REQ-003 section 4 on a real database.
func RunTransitionContract(t *testing.T, newEnv func(t *testing.T) LifecycleEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env LifecycleEnv)
	}{
		{"TenTransitionsConcurrent", tenTransitionsConcurrent},
		{"IdempotentRedeliveryNoExtraOutbox", idempotentRedeliveryNoExtraOutbox},
		{"OutboxFailureRollsBackStatus", outboxFailureRollsBackStatus},
		{"NestedJoinsOuterTransaction", nestedJoinsOuterTransaction},
		{"BacklogCheckHeldAndReopenClears", backlogCheckHeldAndReopenClears},
		{"FullHappyPathPerType", fullHappyPathPerType},
		{"TransitionTenantIsolation", transitionTenantIsolation},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

// RunLifecycleExitContract covers CR-REQ-006 section 4 on a real database.
func RunLifecycleExitContract(t *testing.T, newEnv func(t *testing.T) LifecycleEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env LifecycleEnv)
	}{
		{"ReturnFromEveryStatus", returnFromEveryStatus},
		{"ReturnTwiceNoExtraRows", returnTwiceNoExtraRows},
		{"BacklogCheckHeldEverywhere", backlogCheckHeldEverywhere},
		{"ReopenResetsColumnsAndEmitsStatusChanged", reopenResetsColumns},
		{"CancelFromBacklogAndAnalyzing", cancelFromBacklogAndAnalyzing},
		{"CancelCompletedRejectedAndTwiceOK", cancelCompletedRejectedAndTwiceOK},
		{"ActiveExecutionBlocksReturnAndCancel", activeExecutionBlocksReturnAndCancel},
		{"Spawn12ConcurrentSameKey", spawn12ConcurrentSameKey},
		{"SpawnChildLimit50", spawnChildLimit50},
		{"SpawnDepthSix", spawnDepthSix},
		{"SpawnRollbackNoOrphanLink", spawnRollbackNoOrphanLink},
		{"TenantIsolationLinksAndHistory", tenantIsolationLinksAndHistory},
		{"ReturnHistoryAppendListOrder", returnHistoryAppendListOrder},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func statusPtr(s domain.RequestStatus) *domain.RequestStatus { return &s }

func isLoss(err error) bool {
	if err == nil {
		return false
	}
	c := errCode(err)
	return c == "REQUEST_STATE_STALE" || c == "REQUEST_VERSION_CONFLICT" || c == "REQUEST_TRANSITION_NOT_ALLOWED" ||
		containsAny(err.Error(), "1213", "deadlock detected", "could not serialize")
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		for i := 0; i+len(x) <= len(s); i++ {
			if s[i:i+len(x)] == x {
				return true
			}
		}
	}
	return false
}

func countSubject(rows []OutboxRow, subject string) int {
	n := 0
	for _, r := range rows {
		if r.Subject == subject {
			n++
		}
	}
	return n
}

func mustGet(t *testing.T, env LifecycleEnv, ctx context.Context, id string) domain.Request {
	t.Helper()
	r, err := env.Requests.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func tenOf(tenantID string) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

func tenTransitionsConcurrent(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status, r.Type = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeBug
	})
	type outcome struct {
		trig    domain.Trigger
		applied bool
		err     error
	}
	results := make(chan outcome, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		trig := domain.TriggerTypeConfirmed
		if i%2 == 1 {
			trig = domain.TriggerCancel
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := uc.transition.Execute(ctx, usecase.TransitionInput{
				RequestID: r.ID, Trigger: trig, ExpectedFrom: statusPtr(domain.RequestStatusAwaitingTypeConfirmation),
				ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser, Reason: "race",
			})
			results <- outcome{trig, res.Applied, err}
		}()
	}
	wg.Wait()
	close(results)
	applied := 0
	for o := range results {
		if o.applied {
			applied++
		} else if o.err != nil && !isLoss(o.err) {
			t.Errorf("%s: unexpected error %v", o.trig, o.err)
		}
	}
	if applied != 1 {
		t.Fatalf("%d transitions applied, want exactly 1", applied)
	}
	got := mustGet(t, env, ctx, r.ID)
	if got.Status != domain.RequestStatusAnalyzing && got.Status != domain.RequestStatusCancelled {
		t.Fatalf("final status %s", got.Status)
	}
	if n := countSubject(env.OutboxRows(t, tenantID), "orca.request.request.status_changed"); n != 1 {
		t.Fatalf("%d status_changed rows, want 1", n)
	}
}

func idempotentRedeliveryNoExtraOutbox(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	in := usecase.TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady, ExpectedFrom: statusPtr(domain.RequestStatusClassifying), ActorKind: domain.ActorKindAgent}
	first, err := uc.transition.Execute(ctx, in)
	if err != nil || !first.Applied {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := uc.transition.Execute(ctx, in)
	if err != nil || second.Applied {
		t.Fatalf("second: %+v %v", second, err)
	}
	if n := len(env.OutboxRows(t, tenantID)); n != 1 {
		t.Fatalf("%d outbox rows after redelivery, want 1", n)
	}
	if got := mustGet(t, env, ctx, r.ID); got.Version != 2 {
		t.Fatalf("version %d, want 2", got.Version)
	}
}

func outboxFailureRollsBackStatus(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	uc.outbox.fail = true
	_, err := uc.transition.Execute(ctx, usecase.TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady})
	uc.outbox.fail = false
	if err == nil {
		t.Fatal("want outbox error")
	}
	if got := mustGet(t, env, ctx, r.ID); got.Status != domain.RequestStatusClassifying || got.Version != 1 {
		t.Fatalf("status leaked past rollback: %+v", got)
	}
}

func nestedJoinsOuterTransaction(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	if env.TxScope.InTransaction(ctx) {
		t.Fatal("InTransaction true outside InTx")
	}
	sentinel := errors.New("outer fails")
	err := env.Tx.InTx(ctx, func(txCtx context.Context) error {
		if !env.TxScope.InTransaction(txCtx) {
			return errors.New("InTransaction false inside InTx")
		}
		if _, err := uc.transition.Execute(txCtx, usecase.TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v", err)
	}
	if got := mustGet(t, env, ctx, r.ID); got.Status != domain.RequestStatusClassifying {
		t.Fatalf("nested transition survived the outer rollback: %s", got.Status)
	}
	if n := len(env.OutboxRows(t, tenantID)); n != 0 {
		t.Fatalf("%d outbox rows survived the rollback", n)
	}
}

func backlogCheckHeldAndReopenClears(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusPlanning, domain.RequestTypeBug })
	if _, err := uc.transition.Execute(ctx, usecase.TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerReturnToBacklog, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryInfeasible, Reason: "cannot",
	}); err != nil {
		t.Fatal(err)
	}
	got := mustGet(t, env, ctx, r.ID)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStagePlan || got.ReturnedCategory != domain.ReturnCategoryInfeasible || got.ReturnReason != "cannot" {
		t.Fatalf("backlog row %+v", got)
	}
	if _, err := uc.transition.Execute(ctx, usecase.TransitionInput{RequestID: r.ID, Trigger: domain.TriggerReopen}); err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, env, ctx, r.ID)
	if got.Status != domain.RequestStatusClassifying || got.ReturnedFromStage != "" || got.ReturnedCategory != "" || got.ReturnReason != "" {
		t.Fatalf("reopened row %+v", got)
	}
}

func fullHappyPathPerType(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	for _, typ := range domain.AllRequestTypes() {
		t.Run(string(typ), func(t *testing.T) {
			tenantID := newTenant()
			ctx := tenOf(tenantID)
			flow, err := domain.FlowFor(typ)
			if err != nil {
				t.Fatal(err)
			}
			steps, err := domain.HappyPathSteps(flow, domain.RequestSizeL)
			if err != nil {
				t.Fatal(err)
			}
			r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Size = domain.RequestSizeL })
			for i, step := range steps {
				if step.Trigger == domain.TriggerTypeConfirmed {
					// Classification (CR-REQ-005) sets the type; here the test plays that role.
					cur := mustGet(t, env, ctx, r.ID)
					cur.Type = typ
					if _, err := env.Requests.Update(ctx, cur, cur.Version); err != nil {
						t.Fatal(err)
					}
				}
				res, err := uc.transition.Execute(ctx, usecase.TransitionInput{RequestID: r.ID, Trigger: step.Trigger, ActorKind: domain.ActorKindSystem})
				if err != nil || res.Request.Status != step.To {
					t.Fatalf("step %d %s: status %s err %v, want %s", i, step.Trigger, res.Request.Status, err, step.To)
				}
			}
			if got := mustGet(t, env, ctx, r.ID); got.Status != domain.RequestStatusCompleted {
				t.Fatalf("final status %s", got.Status)
			}
			rows := env.OutboxRows(t, tenantID)
			if n := countSubject(rows, "orca.request.request.status_changed"); n != len(steps) {
				t.Fatalf("%d status_changed rows, want %d", n, len(steps))
			}
			if n := countSubject(rows, "orca.request.request.completed"); n != 1 {
				t.Fatalf("%d completed rows, want 1", n)
			}
		})
	}
}

func transitionTenantIsolation(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctxA, ctxB := tenOf(newTenant()), tenOf(newTenant())
	r := createRequest(t, env.Env, ctxA, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	_, err := uc.transition.Execute(ctxB, usecase.TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady})
	requireCode(t, err, "REQUEST_NOT_FOUND")
	if got := mustGet(t, env, ctxA, r.ID); got.Status != domain.RequestStatusClassifying {
		t.Fatalf("other tenant changed status to %s", got.Status)
	}
}

// ---- CR-REQ-006

var returnableStatuses = []domain.RequestStatus{
	domain.RequestStatusClassifying, domain.RequestStatusAwaitingTypeConfirmation, domain.RequestStatusAnalyzing,
	domain.RequestStatusAwaitingAnalysisApproval, domain.RequestStatusPlanning, domain.RequestStatusAwaitingPlanApproval, domain.RequestStatusExecuting,
}

func stageOf(st domain.RequestStatus) domain.ReturnStage {
	stages, _ := domain.StageForStatus(st, domain.FlowDefinition{}, "")
	return stages[0]
}

func returnFromEveryStatus(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	for _, st := range returnableStatuses {
		t.Run(string(st), func(t *testing.T) {
			tenantID := newTenant()
			ctx := tenOf(tenantID)
			r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = st, domain.RequestTypeTask })
			actor := uuid.NewString()
			got, err := uc.ret.Execute(ctx, usecase.ReturnInput{
				RequestID: r.ID, Stage: stageOf(st), Category: domain.ReturnCategoryMissingInfo, Reason: "need info", ActorID: actor, ActorKind: domain.ActorKindUser,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != stageOf(st) || got.ReturnedCategory != domain.ReturnCategoryMissingInfo {
				t.Fatalf("row %+v", got)
			}
			hist, err := env.Returns.List(ctx, r.ID)
			if err != nil || len(hist) != 1 || hist[0].Action != domain.ReturnActionReturned || hist[0].ActorID != actor || hist[0].Category != domain.ReturnCategoryMissingInfo {
				t.Fatalf("history %+v %v", hist, err)
			}
			rows := env.OutboxRows(t, tenantID)
			if len(rows) != 2 || countSubject(rows, "orca.request.request.returned") != 1 || countSubject(rows, "orca.request.request.status_changed") != 1 {
				t.Fatalf("outbox %v", rows)
			}
		})
	}
}

func returnTwiceNoExtraRows(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusPlanning, domain.RequestTypeBug })
	in := usecase.ReturnInput{RequestID: r.ID, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryOther, Reason: "r", ActorKind: domain.ActorKindUser}
	for i := 0; i < 2; i++ {
		if _, err := uc.ret.Execute(ctx, in); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	hist, _ := env.Returns.List(ctx, r.ID)
	if len(hist) != 1 || len(env.OutboxRows(t, tenantID)) != 2 {
		t.Fatalf("history=%d outbox=%d after two identical returns", len(hist), len(env.OutboxRows(t, tenantID)))
	}
}

func backlogCheckHeldEverywhere(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	// Every path into and out of the backlog keeps (status = backlog) = (category present); the DB would reject otherwise.
	for _, st := range returnableStatuses {
		r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = st, domain.RequestTypeTask })
		if _, err := uc.ret.Execute(ctx, usecase.ReturnInput{RequestID: r.ID, Stage: stageOf(st), Category: domain.ReturnCategoryOther, Reason: "r"}); err != nil {
			t.Fatalf("%s: %v", st, err)
		}
		if _, err := uc.cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "done"}); err != nil {
			t.Fatalf("%s cancel from backlog: %v", st, err)
		}
	}
	other := newTenant()
	for name, ov := range map[string]map[string]any{
		"backlog without category":  {"status": "request_backlog", "returned_from_stage": "plan"},
		"category without backlog":  {"returned_category": "other"},
		"unknown category":          {"status": "request_backlog", "returned_from_stage": "plan", "returned_category": "bogus"},
		"backlog with only a stage": {"status": "request_backlog", "returned_from_stage": "task"},
	} {
		if err := env.InsertRawRequest(other, ov); err == nil {
			t.Errorf("%s was accepted by the database", name)
		}
	}
	if err := env.InsertRawRequest(other, map[string]any{"status": "request_backlog", "returned_from_stage": "plan", "returned_category": "other"}); err != nil {
		t.Errorf("valid backlog row rejected: %v", err)
	}
}

func reopenResetsColumns(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status, r.Type = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeBug
	})
	if _, err := uc.ret.Execute(ctx, usecase.ReturnInput{RequestID: r.ID, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryRejected, Reason: "no"}); err != nil {
		t.Fatal(err)
	}
	got, err := uc.reopen.Execute(ctx, usecase.ReopenInput{RequestID: r.ID, Note: "try again", ActorID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.RequestStatusClassifying || got.ReturnedFromStage != "" || got.ReturnedCategory != "" || got.ReturnReason != "" || got.Type != domain.RequestTypeBug {
		t.Fatalf("row %+v", got)
	}
	hist, _ := env.Returns.List(ctx, r.ID)
	if len(hist) != 2 || hist[1].Action != domain.ReturnActionReopened || hist[1].Reason != "try again" {
		t.Fatalf("history %+v", hist)
	}
	rows := env.OutboxRows(t, tenantID)
	last := rows[len(rows)-1]
	var p map[string]any // JSONB re-spaces the document, so compare decoded values
	if err := json.Unmarshal(last.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if last.Subject != "orca.request.request.status_changed" || p["trigger"] != "reopen" || p["to"] != "classifying" {
		t.Fatalf("last outbox row %s %s", last.Subject, last.Payload)
	}
}

func cancelFromBacklogAndAnalyzing(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	a := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusAnalyzing, domain.RequestTypeBug })
	b := createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status, r.ReturnedFromStage, r.ReturnedCategory, r.ReturnReason = domain.RequestStatusRequestBacklog, domain.ReturnStageAnalysis, domain.ReturnCategoryOther, "x"
	})
	for _, r := range []domain.Request{a, b} {
		res, err := uc.cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "dup", ActorID: uuid.NewString()})
		if err != nil || !res.Applied || res.Request.Status != domain.RequestStatusCancelled || res.Request.ReturnedFromStage != "" {
			t.Fatalf("%s: %+v %v", r.Status, res, err)
		}
	}
}

func cancelCompletedRejectedAndTwiceOK(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	done := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusCompleted })
	_, err := uc.cancel.Execute(ctx, usecase.CancelInput{RequestID: done.ID, Reason: "x"})
	requireCode(t, err, "REQUEST_CANCEL_NOT_ALLOWED")
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusPlanning, domain.RequestTypeBug })
	if _, err := uc.cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	before := len(env.OutboxRows(t, tenantID))
	res, err := uc.cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "x"})
	if err != nil || res.Applied {
		t.Fatalf("second cancel: %+v %v", res, err)
	}
	if after := len(env.OutboxRows(t, tenantID)); after != before {
		t.Fatalf("second cancel wrote %d outbox rows", after-before)
	}
}

func activeExecutionBlocksReturnAndCancel(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusExecuting, domain.RequestTypeTask })
	uc.guard.active = true
	_, err := uc.ret.Execute(ctx, usecase.ReturnInput{RequestID: r.ID, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther, Reason: "r"})
	requireCode(t, err, "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION")
	_, err = uc.cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "r"})
	requireCode(t, err, "REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION")
	if got := mustGet(t, env, ctx, r.ID); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("status %s", got.Status)
	}
}

func newSpikeParent(t *testing.T, env LifecycleEnv, ctx context.Context) domain.Request {
	return createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Status, r.Type = domain.RequestStatusCompleted, domain.RequestTypeSpike })
}

func spawnInput(parent domain.Request, clientID string) usecase.SpawnInput {
	return usecase.SpawnInput{
		ParentRequestID: parent.ID, LinkReason: domain.LinkReasonSpawnedBySpike, Title: "child", TypeHint: domain.RequestTypeTask,
		ClientRequestID: clientID, ActorID: uuid.NewString(), Provider: domain.SourceProviderManual,
	}
}

func spawn12ConcurrentSameKey(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	parent := newSpikeParent(t, env, ctx)
	in := spawnInput(parent, "same-key") // same actor too: the idempotency key includes the actor
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	created := make(chan bool, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var res usecase.SpawnResult
			err := retryDeadlock(func() error {
				var err error
				res, err = uc.spawn.Execute(ctx, in)
				return err
			})
			if err != nil {
				errs <- err
				return
			}
			ids <- res.Child.ID
			created <- res.Created
		}()
	}
	wg.Wait()
	close(ids)
	close(created)
	close(errs)
	for err := range errs {
		t.Errorf("spawn: %v", err)
	}
	distinct, createdCount := map[string]bool{}, 0
	for id := range ids {
		distinct[id] = true
	}
	for c := range created {
		if c {
			createdCount++
		}
	}
	if len(distinct) != 1 || createdCount != 1 {
		t.Fatalf("distinct children=%d created=%d, want 1 and 1", len(distinct), createdCount)
	}
	links, err := env.Links.ListChildren(ctx, parent.ID)
	if err != nil || len(links) != 1 || links[0].Reason != domain.LinkReasonSpawnedBySpike {
		t.Fatalf("links %+v %v", links, err)
	}
	if n := countSubject(env.OutboxRows(t, tenantID), "orca.request.request.created"); n != 1 {
		t.Fatalf("%d created events, want 1", n)
	}
}

func spawnChildLimit50(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	parent := newSpikeParent(t, env, ctx)
	for i := 0; i < domain.MaxChildrenPerParent; i++ {
		child := createRequest(t, env.Env, ctx, nil)
		if err := env.Links.Insert(ctx, domain.RequestLink{ParentRequestID: parent.ID, ChildRequestID: child.ID, Reason: domain.LinkReasonSpawnedBySpike}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := uc.spawn.Execute(ctx, spawnInput(parent, "one-too-many"))
	requireCode(t, err, "REQUEST_CHILD_LIMIT")
}

func spawnDepthSix(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctx := tenOf(newTenant())
	cur := createRequest(t, env.Env, ctx, func(r *domain.Request) { r.Type, r.Status = domain.RequestTypeBug, domain.RequestStatusAnalyzing })
	for level := 2; level <= 6; level++ {
		in := usecase.SpawnInput{
			ParentRequestID: cur.ID, LinkReason: domain.LinkReasonEscalation, Title: fmt.Sprintf("level %d", level),
			ClientRequestID: fmt.Sprintf("lvl-%d", level), ActorID: uuid.NewString(), Provider: domain.SourceProviderManual,
		}
		res, err := uc.spawn.Execute(ctx, in)
		if level <= 5 {
			if err != nil {
				t.Fatalf("level %d: %v", level, err)
			}
			// The escalated child must itself be a valid parent: put it in a non-cancelled state.
			cur = res.Child
			continue
		}
		requireCode(t, err, "REQUEST_CHILD_DEPTH_EXCEEDED")
	}
}

func spawnRollbackNoOrphanLink(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	parent := newSpikeParent(t, env, ctx)
	uc.outbox.fail = true
	_, err := uc.spawn.Execute(ctx, spawnInput(parent, "will-fail"))
	uc.outbox.fail = false
	if err == nil {
		t.Fatal("want forced outbox failure")
	}
	if links, _ := env.Links.ListChildren(ctx, parent.ID); len(links) != 0 {
		t.Fatalf("orphan links: %+v", links)
	}
	// The claim rolled back too, so the same key now succeeds and the number sequence has no hole.
	res, err := uc.spawn.Execute(ctx, spawnInput(parent, "will-fail"))
	if err != nil || !res.Created {
		t.Fatalf("retry: %+v %v", res, err)
	}
	if res.Child.Number != 2 {
		t.Fatalf("child number %d, want 2 (parent is 1, the failed attempt must not burn a number)", res.Child.Number)
	}
}

func tenantIsolationLinksAndHistory(t *testing.T, env LifecycleEnv) {
	uc := buildLifecycle(env)
	ctxA, ctxB := tenOf(newTenant()), tenOf(newTenant())
	parent := newSpikeParent(t, env, ctxA)
	res, err := uc.spawn.Execute(ctxA, spawnInput(parent, "iso"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.cancel.Execute(ctxA, usecase.CancelInput{RequestID: res.Child.ID, Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	if links, _ := env.Links.ListChildren(ctxB, parent.ID); len(links) != 0 {
		t.Fatalf("tenant B sees links %+v", links)
	}
	if hist, _ := env.Returns.List(ctxB, res.Child.ID); len(hist) != 0 {
		t.Fatalf("tenant B sees history %+v", hist)
	}
	if _, err := uc.spawn.Execute(ctxB, spawnInput(parent, "iso-b")); err == nil {
		t.Fatal("tenant B spawned a child under tenant A's parent")
	}
}

func returnHistoryAppendListOrder(t *testing.T, env LifecycleEnv) {
	ctx := tenOf(newTenant())
	r := createRequest(t, env.Env, ctx, nil)
	base := time.Now().UTC().Truncate(time.Microsecond)
	entries := []domain.ReturnHistoryEntry{
		{RequestID: r.ID, Action: domain.ReturnActionCancelled, Reason: "third", ActorKind: domain.ActorKindUser, At: base.Add(2 * time.Second)},
		{RequestID: r.ID, Action: domain.ReturnActionReturned, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryBlockedDependency, Reason: "first", ActorID: uuid.NewString(), ActorKind: domain.ActorKindSystem, At: base},
		{RequestID: r.ID, Action: domain.ReturnActionReopened, Reason: "second", ActorKind: domain.ActorKindUser, At: base.Add(time.Second)},
	}
	for _, e := range entries {
		if err := env.Returns.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := env.Returns.List(ctx, r.ID)
	if err != nil || len(got) != 3 {
		t.Fatalf("list %+v %v", got, err)
	}
	for i, want := range []string{"first", "second", "third"} {
		if got[i].Reason != want {
			t.Errorf("position %d = %q, want %q", i, got[i].Reason, want)
		}
	}
	if got[0].Stage != domain.ReturnStagePlan || got[0].Category != domain.ReturnCategoryBlockedDependency || got[0].ActorKind != domain.ActorKindSystem || got[1].ActorID != "" || got[1].Stage != "" {
		t.Errorf("column round trip: %+v", got)
	}
	if !got[0].At.Equal(base) {
		t.Errorf("at %v != %v", got[0].At, base)
	}
}
