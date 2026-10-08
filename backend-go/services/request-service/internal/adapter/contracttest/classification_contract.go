package contracttest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type scriptedClassifier struct {
	mu    sync.Mutex
	calls int
	out   func(call int) (domain.ClassificationProposal, error)
}

func (s *scriptedClassifier) Classify(context.Context, usecase.ClassificationInput) (domain.ClassificationProposal, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()
	return s.out(n)
}

func proposalOf(t domain.RequestType) func(int) (domain.ClassificationProposal, error) {
	return func(int) (domain.ClassificationProposal, error) {
		return domain.ClassificationProposal{Type: t, Size: domain.RequestSizeM, Urgency: domain.UrgencyNormal, Confidence: 0.85, Reason: "looks like it"}, nil
	}
}

type failingApprovals struct{}

func (failingApprovals) RequestTypeApproval(context.Context, string) error { return nil }
func (failingApprovals) Approve(context.Context, string, string) error {
	return errors.New("approval backend down")
}

type classificationKit struct {
	env     IntakeEnv
	ai      *scriptedClassifier
	propose *usecase.ProposeRequestClassification
	confirm *usecase.ConfirmRequestType
	change  *usecase.ChangeRequestType
	history *usecase.ListRequestTypeHistory
}

func newClassificationKit(env IntakeEnv, ai *scriptedClassifier) *classificationKit {
	tr := realTransitioner(env)
	return &classificationKit{
		env: env, ai: ai,
		propose: usecase.NewProposeRequestClassification(env.Requests, env.History, env.Processed, ai, tr, usecase.NoopApprovalRecorder{}, env.Runs, env.Tx, env.Outbox),
		confirm: usecase.NewConfirmRequestType(env.Requests, env.History, tr, usecase.NoopApprovalRecorder{}, env.Tx, env.Outbox),
		change:  usecase.NewChangeRequestType(env.Requests, env.History, tr, usecase.NoopApprovalCanceller{}, usecase.NoopExecutionGuard{}, env.Tx, env.Outbox),
		history: usecase.NewListRequestTypeHistory(env.Requests, env.History),
	}
}

// RunClassificationContract runs the CR-REQ-005 scenarios on the dialect's real database.
func RunClassificationContract(t *testing.T, newEnv func(t *testing.T) IntakeEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env IntakeEnv)
	}{
		{"AIProposalWritesAllFields", clsProposalWritesAllFields},
		{"AIFailureStillAwaitingConfirmation", clsFailureStillAwaiting},
		{"RedeliverySameEventOnce", clsRedeliveryOnce},
		{"ConfirmConcurrent", clsConfirmConcurrent},
		{"ConfirmIdempotent", clsConfirmIdempotent},
		{"HistoryOrderAIUserChange", clsHistoryOrder},
		{"ChangeKeepsSolutions", clsChangeKeepsSolutions},
		{"AttemptsLimitIncludesFailures", clsAttemptsLimit},
		{"ApprovalFailureRollsBackConfirm", clsApprovalFailureRollsBack},
		{"TenantIsolation", clsTenantIsolation},
		{"ProcessedEvents", clsProcessedEvents},
		{"RunLifecycle", clsRunLifecycle},
		{"RunRecoveryAndClaimLimit", clsRunRecovery},
		{"RunnerEndToEnd", clsRunnerEndToEnd},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) { sc.fn(t, env) })
	}
}

func seedRequest(t *testing.T, env IntakeEnv, ctx context.Context, status domain.RequestStatus, mod func(r *domain.Request)) domain.Request {
	t.Helper()
	return createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status = status
		if mod != nil {
			mod(r)
		}
	})
}

func clsProposalWritesAllFields(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	k := newClassificationKit(env, &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)})
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	if err := k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID, EventID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.Type != domain.RequestTypeBug || got.Size != domain.RequestSizeM || got.Confidence == nil || *got.Confidence != 0.85 ||
		got.TypeSource != domain.TypeSourceAI || got.ClassificationAttempts != 1 || got.Status != domain.RequestStatusAwaitingTypeConfirmation || got.ClassificationReason != "looks like it" {
		t.Fatalf("%+v", got)
	}
	h, _ := env.History.List(ctx, r.ID)
	if len(h) != 1 || h[0].ActorKind != domain.ActorKindAgent || h[0].ToType != domain.RequestTypeBug {
		t.Fatalf("%+v", h)
	}
	n := 0
	for _, s := range env.OutboxSubjects(t, tenantID) {
		if s == domain.SubjectRequestClassified {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d classified events", n)
	}
}

func clsFailureStillAwaiting(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	k := newClassificationKit(env, &scriptedClassifier{out: func(int) (domain.ClassificationProposal, error) {
		return domain.ClassificationProposal{}, domain.ErrProposalInvalid
	}})
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	if err := k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.Status != domain.RequestStatusAwaitingTypeConfirmation || got.Type != "" || got.ClassificationAttempts != 1 || got.ClassificationReason != "invalid classifier output" {
		t.Fatalf("%+v", got)
	}
}

func clsRedeliveryOnce(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	ai := &scriptedClassifier{out: proposalOf(domain.RequestTypeTask)}
	k := newClassificationKit(env, ai)
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	ev := uuid.NewString()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Concurrent deliveries may lose the CAS; the point is that only one result is ever recorded.
			_ = retryDeadlock(func() error { return k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID, EventID: ev}) })
		}()
	}
	wg.Wait()
	_ = k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID, EventID: ev})
	got, _ := env.Requests.Get(ctx, r.ID)
	h, _ := env.History.List(ctx, r.ID)
	if got.ClassificationAttempts != 1 || len(h) != 1 {
		t.Fatalf("attempts=%d history=%d", got.ClassificationAttempts, len(h))
	}
}

func awaitingWithProposal(t *testing.T, env IntakeEnv, ctx context.Context, typ domain.RequestType) domain.Request {
	return seedRequest(t, env, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) {
		c := 0.8
		r.Type, r.TypeSource, r.Confidence, r.Size = typ, domain.TypeSourceAI, &c, domain.RequestSizeM
	})
}

func userActor(ctx context.Context) (string, domain.ActorKind) {
	id, _ := tenant.UserID(ctx)
	return id, domain.ActorKindUser
}

func clsConfirmConcurrent(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	k := newClassificationKit(env, &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)})
	r := awaitingWithProposal(t, env, ctx, domain.RequestTypeBug)
	actor, kind := userActor(ctx)
	types := []string{"task", "question"}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, typ := range types {
		wg.Add(1)
		go func(i int, typ string) {
			defer wg.Done()
			<-start
			errs[i] = retryDeadlock(func() error {
				_, err := k.confirm.Execute(ctx, usecase.ConfirmInput{RequestID: r.ID, Type: typ, ActorID: actor, ActorKind: kind})
				return err
			})
		}(i, typ)
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errCode(err) == "REQUEST_VERSION_CONFLICT" || errCode(err) == "REQUEST_TRANSITION_NOT_ALLOWED":
		default:
			t.Fatalf("unexpected loser error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d winners (errs: %v)", wins, errs)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	wantStatus := map[domain.RequestType]domain.RequestStatus{domain.RequestTypeTask: domain.RequestStatusPlanning, domain.RequestTypeQuestion: domain.RequestStatusAnalyzing}[got.Type]
	if wantStatus == "" || got.Status != wantStatus {
		t.Fatalf("%+v", got)
	}
}

func clsConfirmIdempotent(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	k := newClassificationKit(env, &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)})
	r := awaitingWithProposal(t, env, ctx, domain.RequestTypeBug)
	actor, kind := userActor(ctx)
	in := usecase.ConfirmInput{RequestID: r.ID, Type: "bug", Size: "M", ActorID: actor, ActorKind: kind}
	if _, err := k.confirm.Execute(ctx, in); err != nil {
		t.Fatal(err)
	}
	events := len(env.OutboxSubjects(t, tenantID))
	out, err := k.confirm.Execute(ctx, in)
	if err != nil || out.Status != domain.RequestStatusAnalyzing || len(env.OutboxSubjects(t, tenantID)) != events {
		t.Fatalf("second confirm must be a silent success: %+v %v", out, err)
	}
	if h, _ := env.History.List(ctx, r.ID); len(h) != 0 {
		t.Fatalf("accepting the AI proposal must not write history: %+v", h)
	}
}

func clsHistoryOrder(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	k := newClassificationKit(env, &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)})
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	if err := k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	actor, kind := userActor(ctx)
	time.Sleep(5 * time.Millisecond)
	if _, err := k.confirm.Execute(ctx, usecase.ConfirmInput{RequestID: r.ID, Type: "task", Reason: "smaller than it looks", ActorID: actor, ActorKind: kind}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := k.change.Execute(ctx, usecase.ChangeInput{RequestID: r.ID, NewType: "change_request", Reason: "scope grew", ActorID: actor, ActorKind: kind}); err != nil {
		t.Fatal(err)
	}
	h, err := k.history.Execute(ctx, r.ID)
	if err != nil || len(h) != 3 {
		t.Fatalf("%+v %v", h, err)
	}
	want := []struct {
		to   domain.RequestType
		kind domain.ActorKind
	}{{domain.RequestTypeBug, domain.ActorKindAgent}, {domain.RequestTypeTask, domain.ActorKindUser}, {domain.RequestTypeChangeRequest, domain.ActorKindUser}}
	for i, w := range want {
		if h[i].ToType != w.to || h[i].ActorKind != w.kind {
			t.Fatalf("row %d = %+v", i, h[i])
		}
	}
	if h[1].FromType != domain.RequestTypeBug || h[2].FromType != domain.RequestTypeTask {
		t.Fatalf("from chain broken: %+v", h)
	}
}

func clsChangeKeepsSolutions(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	k := newClassificationKit(env, &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)})
	r := seedRequest(t, env, ctx, domain.RequestStatusAnalyzing, func(r *domain.Request) { r.Type, r.TypeSource = domain.RequestTypeBug, domain.TypeSourceHuman })
	now := time.Now().UTC().Truncate(time.Microsecond)
	sol := domain.Solution{ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, OptionsJSON: []byte(`[{"title":"A"}]`), Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := env.Solutions.Insert(ctx, sol); err != nil {
		t.Fatal(err)
	}
	actor, kind := userActor(ctx)
	out, err := k.change.Execute(ctx, usecase.ChangeInput{RequestID: r.ID, NewType: "change_request", Reason: "bigger", ActorID: actor, ActorKind: kind})
	if err != nil || out.Status != domain.RequestStatusAwaitingTypeConfirmation {
		t.Fatalf("%+v %v", out, err)
	}
	list, _ := env.Solutions.ListByRequestID(ctx, r.ID)
	if len(list) != 1 || list[0].ID != sol.ID || list[0].Version != 1 {
		t.Fatalf("solutions changed: %+v", list)
	}
	subjects := env.OutboxSubjects(t, tenantID)
	if subjects[len(subjects)-1] != domain.SubjectRequestTypeChanged {
		t.Fatalf("subjects = %v", subjects)
	}
}

func clsAttemptsLimit(t *testing.T, env IntakeEnv) {
	ctx := userCtx(newTenant())
	ai := &scriptedClassifier{out: func(int) (domain.ClassificationProposal, error) {
		return domain.ClassificationProposal{}, usecase.ErrNoDevServer
	}}
	k := newClassificationKit(env, ai)
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	for i := 0; i < domain.MaxClassificationAttempts; i++ {
		if err := k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID, Manual: true}); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	err := k.propose.Execute(ctx, usecase.ProposeInput{RequestID: r.ID, Manual: true})
	requireCode(t, err, "REQUEST_CLASSIFICATION_LIMIT")
	if ai.calls != domain.MaxClassificationAttempts {
		t.Fatalf("AI called %d times", ai.calls)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.ClassificationAttempts != domain.MaxClassificationAttempts {
		t.Fatalf("attempts = %d", got.ClassificationAttempts)
	}
}

func clsApprovalFailureRollsBack(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	confirm := usecase.NewConfirmRequestType(env.Requests, env.History, realTransitioner(env), failingApprovals{}, env.Tx, env.Outbox)
	r := awaitingWithProposal(t, env, ctx, domain.RequestTypeBug)
	actor, kind := userActor(ctx)
	if _, err := confirm.Execute(ctx, usecase.ConfirmInput{RequestID: r.ID, Type: "task", ActorID: actor, ActorKind: kind}); err == nil {
		t.Fatal("want error")
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	h, _ := env.History.List(ctx, r.ID)
	if got.Type != domain.RequestTypeBug || got.Status != domain.RequestStatusAwaitingTypeConfirmation || len(h) != 0 || len(env.OutboxSubjects(t, tenantID)) != 0 {
		t.Fatalf("confirmation must roll back as a whole: %+v history=%d", got, len(h))
	}
}

func clsTenantIsolation(t *testing.T, env IntakeEnv) {
	ownerCtx := userCtx(newTenant())
	k := newClassificationKit(env, &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)})
	r := awaitingWithProposal(t, env, ownerCtx, domain.RequestTypeBug)
	other := userCtx(newTenant())
	actor, kind := userActor(other)
	_, err := k.confirm.Execute(other, usecase.ConfirmInput{RequestID: r.ID, Type: "task", ActorID: actor, ActorKind: kind})
	requireCode(t, err, "REQUEST_NOT_FOUND")
	_, err = k.history.Execute(other, r.ID)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	err = k.propose.Execute(other, usecase.ProposeInput{RequestID: r.ID, Manual: true})
	requireCode(t, err, "REQUEST_NOT_FOUND")
}

func clsProcessedEvents(t *testing.T, env IntakeEnv) {
	a, b := userCtx(newTenant()), userCtx(newTenant())
	ev := uuid.NewString()
	if already, err := env.Processed.MarkProcessed(a, ev, "s"); err != nil || already {
		t.Fatalf("first: %v %v", already, err)
	}
	if already, err := env.Processed.MarkProcessed(a, ev, "s"); err != nil || !already {
		t.Fatalf("second: %v %v", already, err)
	}
	if already, err := env.Processed.MarkProcessed(b, ev, "s"); err != nil || already {
		t.Fatalf("same event id in another tenant is a first delivery: %v %v", already, err)
	}
	// The marker joins the caller's transaction.
	rolled := uuid.NewString()
	boom := errors.New("rollback")
	err := env.Tx.InTx(a, func(txCtx context.Context) error {
		if _, err := env.Processed.MarkProcessed(txCtx, rolled, "s"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if already, _ := env.Processed.MarkProcessed(a, rolled, "s"); already {
		t.Fatal("marker survived a rolled-back transaction")
	}
	n, err := env.Processed.Prune(context.Background(), time.Now().Add(time.Hour))
	if err != nil || n < 3 {
		t.Fatalf("prune removed %d rows, err %v", n, err)
	}
	if already, _ := env.Processed.MarkProcessed(a, ev, "s"); already {
		t.Fatal("pruned marker still blocks")
	}
}

func newRun(tenantID, requestID, eventID, owner string, lease time.Time) domain.ClassificationRun {
	return domain.ClassificationRun{ID: uuid.NewString(), TenantID: tenantID, RequestID: requestID, Trigger: "start_classification", SourceEventID: eventID,
		ActorID: uuid.NewString(), Status: domain.ClassificationRunRunning, Claims: 1, LeaseOwner: owner, LeaseExpiresAt: lease.UTC().Truncate(time.Microsecond), StartedAt: time.Now().UTC().Truncate(time.Microsecond)}
}

func clsRunLifecycle(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	ev := uuid.NewString()
	first := newRun(tenantID, r.ID, ev, "a", time.Now().Add(time.Minute))
	if _, started, err := env.Runs.Start(ctx, first); err != nil || !started {
		t.Fatalf("first start: %v %v", started, err)
	}
	// Same event again: the existing run comes back.
	dup, started, err := env.Runs.Start(ctx, newRun(tenantID, r.ID, ev, "b", time.Now().Add(time.Minute)))
	if err != nil || started || dup.ID != first.ID {
		t.Fatalf("event dedupe: %+v %v %v", dup, started, err)
	}
	// Another event while the first run is live: still one live run per request.
	live, started, err := env.Runs.Start(ctx, newRun(tenantID, r.ID, uuid.NewString(), "b", time.Now().Add(time.Minute)))
	if err != nil || started || live.ID != first.ID {
		t.Fatalf("one live run: %+v %v %v", live, started, err)
	}
	if ok, _ := env.Runs.Renew(ctx, first.ID, "intruder", time.Now().Add(time.Hour)); ok {
		t.Fatal("only the lease owner may renew")
	}
	if ok, err := env.Runs.Renew(ctx, first.ID, "a", time.Now().Add(time.Hour)); err != nil || !ok {
		t.Fatalf("owner renew: %v %v", ok, err)
	}
	if err := env.Runs.Finish(ctx, first.ID, domain.ClassificationRunSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	fin, err := env.Runs.Get(ctx, first.ID)
	if err != nil || fin.Status != domain.ClassificationRunSucceeded || fin.FinishedAt == nil {
		t.Fatalf("%+v %v", fin, err)
	}
	if ok, _ := env.Runs.Renew(ctx, first.ID, "a", time.Now().Add(time.Hour)); ok {
		t.Fatal("finished run cannot be renewed")
	}
	// Redelivery of the finished run's event still dedupes; a new event can start a new run.
	again, started, _ := env.Runs.Start(ctx, newRun(tenantID, r.ID, ev, "c", time.Now().Add(time.Minute)))
	if started || again.ID != first.ID {
		t.Fatalf("finished event must still dedupe: %+v %v", again, started)
	}
	next, started, err := env.Runs.Start(ctx, newRun(tenantID, r.ID, uuid.NewString(), "c", time.Now().Add(time.Minute)))
	if err != nil || !started || next.ID == first.ID {
		t.Fatalf("slot must be free after Finish: %+v %v %v", next, started, err)
	}
	_, err = env.Runs.Get(userCtx(newTenant()), next.ID)
	requireCode(t, err, "REQUEST_NOT_FOUND")
}

func clsRunRecovery(t *testing.T, env IntakeEnv) {
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := userCtx(tenantA), userCtx(tenantB)
	ra := seedRequest(t, env, ctxA, domain.RequestStatusClassifying, nil)
	rb := seedRequest(t, env, ctxB, domain.RequestStatusClassifying, nil)
	exhausted := seedRequest(t, env, ctxA, domain.RequestStatusClassifying, nil)
	fresh := seedRequest(t, env, ctxA, domain.RequestStatusClassifying, nil)

	mk := func(ctx context.Context, tid, reqID string, claims int, expired bool) domain.ClassificationRun {
		run := newRun(tid, reqID, uuid.NewString(), "dead", time.Now().Add(time.Hour))
		run.Claims = claims
		if _, ok, err := env.Runs.Start(ctx, run); err != nil || !ok {
			t.Fatalf("start: %v %v", ok, err)
		}
		if expired {
			env.SetRunLease(t, run.ID, time.Now().Add(-time.Minute))
		}
		return run
	}
	runA := mk(ctxA, tenantA, ra.ID, 1, true)
	runB := mk(ctxB, tenantB, rb.ID, 2, true)
	runEx := mk(ctxA, tenantA, exhausted.ID, domain.MaxClassificationRunClaims, true)
	runFresh := mk(ctxA, tenantA, fresh.ID, 1, false)

	claimed, err := env.Runs.ClaimExpired(context.Background(), "me", time.Now().Add(time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.ClassificationRun{}
	for _, c := range claimed {
		byID[c.ID] = c
	}
	if len(byID) != 2 || byID[runA.ID].TenantID != tenantA || byID[runB.ID].TenantID != tenantB {
		t.Fatalf("claimed %v, want the expired runs of both tenants", claimed)
	}
	if byID[runA.ID].Claims != 2 || byID[runB.ID].Claims != 3 || byID[runA.ID].LeaseOwner != "me" {
		t.Fatalf("claim bookkeeping wrong: %+v", byID)
	}
	if got, _ := env.Runs.Get(ctxA, runEx.ID); got.Status != domain.ClassificationRunFailed || got.ErrorCode != "LEASE_EXPIRED" {
		t.Fatalf("exhausted run must be failed: %+v", got)
	}
	if got, _ := env.Runs.Get(ctxA, runFresh.ID); got.Status != domain.ClassificationRunRunning || got.LeaseOwner != "dead" {
		t.Fatalf("live lease must be untouched: %+v", got)
	}
	again, _ := env.Runs.ClaimExpired(context.Background(), "me", time.Now().Add(time.Minute), 10)
	if len(again) != 0 {
		t.Fatalf("just-claimed runs have a fresh lease: %v", again)
	}
}

func clsRunnerEndToEnd(t *testing.T, env IntakeEnv) {
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	ai := &scriptedClassifier{out: proposalOf(domain.RequestTypeBug)}
	k := newClassificationKit(env, ai)
	runner := usecase.NewClassificationRunner(env.Requests, env.Runs, k.propose, env.Tx, "e2e", time.Minute)
	defer runner.Close()
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	res, err := runner.Enqueue(ctx, usecase.EnqueueClassificationInput{RequestID: r.ID, EventID: uuid.NewString(), Trigger: "start_classification"})
	if err != nil || !res.Started {
		t.Fatalf("%+v %v", res, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		run, err := env.Runs.Get(ctx, res.Run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == domain.ClassificationRunSucceeded {
			break
		}
		if run.Status == domain.ClassificationRunFailed || time.Now().After(deadline) {
			t.Fatalf("run = %+v", run)
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.Status != domain.RequestStatusAwaitingTypeConfirmation || got.Type != domain.RequestTypeBug || ai.calls != 1 {
		t.Fatalf("%+v calls=%d", got, ai.calls)
	}
}

// SeedClassifyingRunForRLS creates a classifying request with a live run and returns the run id;
// adapter tests use it to probe row-level security below the repositories.
func SeedClassifyingRunForRLS(t *testing.T, env IntakeEnv, ctx context.Context) string {
	t.Helper()
	tenantID, _ := tenant.TenantID(ctx)
	r := seedRequest(t, env, ctx, domain.RequestStatusClassifying, nil)
	run := newRun(tenantID, r.ID, uuid.NewString(), "rls", time.Now().Add(time.Minute))
	if _, ok, err := env.Runs.Start(ctx, run); err != nil || !ok {
		t.Fatalf("start run: %v %v", ok, err)
	}
	return run.ID
}
