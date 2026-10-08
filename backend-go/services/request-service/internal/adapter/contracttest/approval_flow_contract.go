package contracttest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const probeSubject = "test.approval.probe"

// probeArtifacts stands in for the Solution/Plan owner: Decided writes a probe outbox row in the approval
// transaction, so rollback and atomicity are observable.
type probeArtifacts struct {
	out        usecase.OutboxWriter
	failAfter  bool
	mu         sync.Mutex
	failActive bool
}

func (p *probeArtifacts) setFail(v bool) { p.mu.Lock(); p.failActive = v; p.mu.Unlock() }
func (p *probeArtifacts) failing() bool  { p.mu.Lock(); defer p.mu.Unlock(); return p.failActive }

func (p *probeArtifacts) Describe(_ context.Context, req domain.Request, st domain.SubjectType, hinted string) (string, string, error) {
	id := hinted
	if id == "" {
		id = "subject-" + req.ID
	}
	return id, domain.SubjectDigest(st, id, "v1"), nil
}

func (p *probeArtifacts) Decided(ctx context.Context, a domain.Approval, approved bool) error {
	ev, err := usecase.NewOutboxEvent(ctx, probeSubject, map[string]any{"approval_id": a.ID, "approved": approved})
	if err != nil {
		return err
	}
	if err := p.out.InsertOutboxEvent(ctx, ev); err != nil {
		return err
	}
	if p.failing() {
		return errors.New("forced artifact failure after the probe write")
	}
	return nil
}

func (p *probeArtifacts) Closed(context.Context, domain.Approval, string) error { return nil }

type rigTeams struct {
	mu      sync.Mutex
	teamsOf map[string][]string
	members map[string][]string
}

func (r *rigTeams) TeamsForUser(_ context.Context, u string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.teamsOf[u], nil
}
func (r *rigTeams) MembersOfTeam(_ context.Context, id string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.members[id], nil
}

type rigAdmins struct {
	mu     sync.Mutex
	admins []string
}

func (r *rigAdmins) ListAdmins(context.Context, string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.admins, nil
}

type approvalRig struct {
	env      ApprovalEnv
	art      *probeArtifacts
	teams    *rigTeams
	admins   *rigAdmins
	registry *usecase.SubjectHandlerRegistry
	open     *usecase.OpenApproval
	decide   *usecase.DecideApproval
	cancel   *usecase.CancelApproval
	canceler *usecase.CancelPendingApprovalsForRequest
	expire   *usecase.ExpireApprovals
	remind   *usecase.RemindPendingApprovals
	extend   *usecase.ExtendApproval
	pending  *usecase.ListPendingApprovalsForUser
	ret      *usecase.ReturnRequestToBacklog
	recorder *usecase.RequestTypeApprovalRecorder
	typeH    *usecase.RequestTypeApprovalHandler
}

func newApprovalRig(env ApprovalEnv) *approvalRig {
	r := &approvalRig{env: env, art: &probeArtifacts{out: env.Outbox}, teams: &rigTeams{teamsOf: map[string][]string{}, members: map[string][]string{}}, admins: &rigAdmins{}}
	r.registry = usecase.NewSubjectHandlerRegistry()
	tr := usecase.NewTransitionRequest(env.Requests, env.TxScope, env.Outbox)
	approvals := env.Approvals
	r.canceler = &usecase.CancelPendingApprovalsForRequest{Repo: approvals, Tx: env.Tx, Locker: env.Locker, Registry: r.registry, Outbox: env.Outbox}
	r.ret = usecase.NewReturnRequestToBacklog(env.Requests, tr, env.Returns, &usecase.PendingApprovalCanceller{Inner: r.canceler}, usecase.NoActiveExecutionGuard{}, env.Tx, env.Outbox)
	for _, st := range domain.AllSubjectTypes {
		if st != domain.SubjectRequestType {
			r.registry.Register(st, &usecase.TransitionSubjectHandler{Artifacts: r.art, Transition: tr, Returner: r.ret, Requests: env.Locker})
		}
	}
	r.typeH = &usecase.RequestTypeApprovalHandler{Returner: r.ret, Requests: env.Locker}
	r.registry.Register(domain.SubjectRequestType, r.typeH)
	r.open = &usecase.OpenApproval{Repo: approvals, Tx: env.TxScope, Requests: env.Requests, Locker: env.Locker, Registry: r.registry, Resolver: &usecase.ResolveApproverPolicy{Repo: env.Policies},
		ApproverRepo: env.Approvers, Outbox: env.Outbox, Teams: r.teams, Admins: r.admins}
	r.expire = &usecase.ExpireApprovals{Repo: approvals, Tx: env.Tx, Locker: env.Locker, Registry: r.registry, Returner: r.ret, Outbox: env.Outbox}
	r.decide = &usecase.DecideApproval{Repo: approvals, Tx: env.Tx, Locker: env.Locker, Registry: r.registry, Outbox: env.Outbox, Expirer: r.expire,
		Authorizer: &usecase.AuthorizeApprovalDecision{ApproverRepo: env.Approvers, Teams: r.teams}}
	r.cancel = &usecase.CancelApproval{Repo: approvals, Tx: env.Tx, Locker: env.Locker, Registry: r.registry, Outbox: env.Outbox}
	r.remind = &usecase.RemindPendingApprovals{Repo: approvals, Requests: env.Requests, Tx: env.Tx, Outbox: env.Outbox}
	r.extend = &usecase.ExtendApproval{Repo: approvals, Tx: env.Tx, Locker: env.Locker}
	r.pending = &usecase.ListPendingApprovalsForUser{Repo: approvals, Teams: r.teams}
	r.recorder = &usecase.RequestTypeApprovalRecorder{Open: r.open, Repo: approvals, Tx: env.Tx, Locker: env.Locker, Outbox: env.Outbox}
	return r
}

func asUser(tenantID, user, role string) context.Context {
	return tenant.WithRole(tenant.WithUserID(tenOf(tenantID), user), role)
}

func (r *approvalRig) planRequest(t *testing.T, tenantID string) domain.Request {
	t.Helper()
	return createRequest(t, r.env.Env, tenOf(tenantID), func(rq *domain.Request) {
		rq.Status, rq.Type, rq.Size = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeChangeRequest, domain.RequestSizeM
	})
}

func (r *approvalRig) setPolicy(t *testing.T, tenantID string, mod func(p *domain.ApprovalPolicy)) {
	t.Helper()
	if _, err := r.env.Policies.Upsert(tenOf(tenantID), policyOf(tenantID, mod), 0); err != nil {
		t.Fatal(err)
	}
}

func rowsOf(env ApprovalEnv, t *testing.T, tenantID, subject string) []OutboxRow {
	t.Helper()
	var out []OutboxRow
	for _, r := range env.OutboxRows(t, tenantID) {
		if r.Subject == subject {
			out = append(out, r)
		}
	}
	return out
}

// RunApprovalFlowContract drives the approval use cases over a real database: TASK-REQ-009-06 (atomicity,
// races, tenant isolation) and the end-to-end half of TASK-REQ-010-07 (policy, team, expiry, reminders).
func RunApprovalFlowContract(t *testing.T, newEnv func(t *testing.T) ApprovalEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, rig *approvalRig)
	}{
		{"OpenThenApproveMovesRequestAndWritesAtomically", flowOpenApprove},
		{"HandlerFailureRollsBackApprovalRequestAndProbe", flowHandlerFailureRollsBack},
		{"RejectReturnsRequestToBacklogWithComment", flowReject},
		{"CancelPendingRacesApproveExactlyOneWins", flowCancelRacesApprove},
		{"ConcurrentApprovesExactlyOneWins", flowConcurrentApproves},
		{"ExpireSweepsEveryTenantWithoutAmbientTenant", flowExpireAcrossTenants},
		{"ExpireRacesApproveNoUnexpectedErrors", flowExpireRacesApprove},
		{"ReminderFiresOnceEvenWithTwoSweepers", flowReminderOnce},
		{"TeamPolicySnapshotDecidesWhoMayApprove", flowTeamPolicy},
		{"PolicyChangeAfterOpenDoesNotChangeApprovers", flowSnapshotImmuneToPolicyEdit},
		{"PreDeployNeedsAnotherAdmin", flowPreDeployNoEligible},
		{"CrossTenantCannotDecideOrRead", flowCrossTenant},
		{"RequestTypeApprovalConfirmsThroughRealConfirmRequestType", flowRequestTypeConfirm},
		{"RequestTypeDirectConfirmSettlesApproval", flowRequestTypeDirectConfirm},
		{"PendingForUserMatchesAcrossPrincipals", flowPendingForUser},
		{"ExtendKeepsApprovalAliveAndRearmsReminder", flowExtend},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, newApprovalRig(env)) })
	}
}

func flowOpenApprove(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	r := rig.planRequest(t, tenantID)
	a, err := rig.open.Execute(asUser(tenantID, r.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	if a.DueAt == nil || a.Stage != "awaiting_plan_approval" || !a.SelfApprovalAllowed {
		t.Fatalf("opened = %+v", a)
	}
	snap, _ := rig.env.Approvers.ListForApproval(tenOf(tenantID), tenantID, a.ID)
	if len(snap) != 2 {
		t.Fatalf("snapshot = %v", snap)
	}
	res, err := rig.decide.Execute(asUser(tenantID, r.ReporterID, "user"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", Comment: "ship it", ExpectedDigest: a.SubjectDigest})
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestStatus != domain.RequestStatusExecuting || res.Approval.Status != domain.ApprovalStatusApproved || res.Approval.Version != 2 {
		t.Fatalf("result = %+v", res)
	}
	got := mustGetRequest(t, rig, tenantID, r.ID)
	if got.Status != domain.RequestStatusExecuting {
		t.Fatalf("request = %s", got.Status)
	}
	if n := len(rowsOf(rig.env, t, tenantID, probeSubject)); n != 1 {
		t.Fatalf("probe rows = %d", n)
	}
	for _, subj := range []string{"orca.request.approval.requested", "orca.request.approval.decided", "orca.request.request.status_changed"} {
		rows := rowsOf(rig.env, t, tenantID, subj)
		if len(rows) != 1 {
			t.Fatalf("%s rows = %d", subj, len(rows))
		}
		if strings.Contains(string(rows[0].Payload), "ship it") || strings.Contains(string(rows[0].Payload), `"comment"`) {
			t.Fatalf("%s payload must not carry the comment: %s", subj, rows[0].Payload)
		}
	}
	var p domain.ApprovalRequestedPayload
	_ = json.Unmarshal(rowsOf(rig.env, t, tenantID, "orca.request.approval.requested")[0].Payload, &p)
	if p.ApprovalID != a.ID || p.ReporterID != r.ReporterID || p.RequestNumber != r.Number || !p.SelfApprovalAllowed || p.DueAt == nil {
		t.Fatalf("requested payload = %+v", p)
	}
}

func mustGetRequest(t *testing.T, rig *approvalRig, tenantID, id string) domain.Request {
	t.Helper()
	r, err := rig.env.Requests.Get(tenOf(tenantID), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func flowHandlerFailureRollsBack(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	r := rig.planRequest(t, tenantID)
	ctx := asUser(tenantID, r.ReporterID, "user")
	a, err := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	rig.art.setFail(true)
	if _, err := rig.decide.Execute(ctx, usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err == nil {
		t.Fatal("handler failure must surface")
	}
	rig.art.setFail(false)
	got, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
	if got.Status != domain.ApprovalStatusPending || got.Version != 1 {
		t.Fatalf("approval after rollback = %+v", got)
	}
	if mustGetRequest(t, rig, tenantID, r.ID).Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatal("request must not move when the handler failed")
	}
	if n := len(rowsOf(rig.env, t, tenantID, probeSubject)); n != 0 {
		t.Fatalf("the probe row written before the failure must be rolled back, found %d", n)
	}
	if n := len(rowsOf(rig.env, t, tenantID, "orca.request.approval.decided")); n != 0 {
		t.Fatalf("decided events = %d", n)
	}
	if _, err := rig.decide.Execute(ctx, usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatalf("a retry after the failure must work: %v", err)
	}
}

func flowReject(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	r := rig.planRequest(t, tenantID)
	ctx := asUser(tenantID, r.ReporterID, "user")
	a, _ := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if _, err := rig.decide.Execute(ctx, usecase.DecideApprovalInput{ID: a.ID, Decision: "reject", Comment: " ", ExpectedDigest: a.SubjectDigest}); errCode(err) != "REQUEST_APPROVAL_COMMENT_REQUIRED" {
		t.Fatalf("reject without a comment = %v", err)
	}
	res, err := rig.decide.Execute(ctx, usecase.DecideApprovalInput{ID: a.ID, Decision: "reject", Comment: "scope too wide", ExpectedDigest: a.SubjectDigest})
	if err != nil {
		t.Fatal(err)
	}
	got := mustGetRequest(t, rig, tenantID, r.ID)
	if res.Approval.Status != domain.ApprovalStatusRejected || got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStagePlan ||
		got.ReturnedCategory != domain.ReturnCategoryRejected || got.ReturnReason != "scope too wide" {
		t.Fatalf("approval=%s request=%+v", res.Approval.Status, got)
	}
	if n := len(rowsOf(rig.env, t, tenantID, "orca.request.request.returned")); n != 1 {
		t.Fatalf("returned events = %d", n)
	}
}

func flowCancelRacesApprove(t *testing.T, rig *approvalRig) {
	for i := 0; i < 50; i++ {
		tenantID := newTenant()
		r := rig.planRequest(t, tenantID)
		ctx := asUser(tenantID, r.ReporterID, "user")
		a, err := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
		if err != nil {
			t.Fatal(err)
		}
		tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var approveErr, cancelErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			approveErr = retryDeadlock(func() error {
				_, err := rig.decide.Execute(tctx, usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
				return err
			})
		}()
		go func() {
			defer wg.Done()
			cancelErr = retryDeadlock(func() error { return rig.canceler.Execute(tctx, r.ID, "returned") })
		}()
		wg.Wait()
		stalled := tctx.Err() != nil
		cancel()
		if stalled {
			t.Fatalf("iteration %d: deadlock or stall (context expired)", i)
		}
		if cancelErr != nil {
			t.Fatalf("iteration %d: cancel pending failed: %v", i, cancelErr)
		}
		got, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
		switch got.Status {
		case domain.ApprovalStatusApproved:
			if approveErr != nil {
				t.Fatalf("iteration %d: approved but the call failed: %v", i, approveErr)
			}
		case domain.ApprovalStatusCancelled:
			if errCode(approveErr) != "REQUEST_APPROVAL_ALREADY_DECIDED" {
				t.Fatalf("iteration %d: cancelled so approve must report ALREADY_DECIDED, got %v", i, approveErr)
			}
		default:
			t.Fatalf("iteration %d: final status %s", i, got.Status)
		}
		if n := len(rowsOf(rig.env, t, tenantID, "orca.request.approval.decided")); n != 1 {
			t.Fatalf("iteration %d: exactly one decided event expected, got %d", i, n)
		}
	}
}

func flowConcurrentApproves(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	r := rig.planRequest(t, tenantID)
	ctx := asUser(tenantID, r.ReporterID, "user")
	a, _ := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	admins := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	var wg sync.WaitGroup
	results := make(chan error, len(admins))
	for _, adm := range admins {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- retryDeadlock(func() error {
				_, err := rig.decide.Execute(asUser(tenantID, adm, "admin"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
				return err
			})
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errCode(err) == "REQUEST_APPROVAL_ALREADY_DECIDED":
		default:
			t.Fatalf("unexpected error %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("exactly one approver must win, got %d", ok)
	}
	if n := len(rowsOf(rig.env, t, tenantID, probeSubject)); n != 1 {
		t.Fatalf("the artifact effect ran %d times", n)
	}
}

func flowExpireAcrossTenants(t *testing.T, rig *approvalRig) {
	tenants := []string{newTenant(), newTenant()}
	type opened struct {
		tenant string
		a      *domain.Approval
		r      domain.Request
	}
	var all []opened
	for _, tn := range tenants {
		rig.setPolicy(t, tn, func(p *domain.ApprovalPolicy) { d := time.Second; p.DueAfter = &d })
		r := rig.planRequest(t, tn)
		a, err := rig.open.Execute(asUser(tn, r.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, opened{tn, a, r})
	}
	time.Sleep(1500 * time.Millisecond)
	n, err := rig.expire.Execute(context.Background(), 100) // no tenant in ctx, like the real sweeper
	if err != nil || n < 2 {
		t.Fatalf("expired %d (err %v), want at least our two", n, err)
	}
	for _, o := range all {
		got, _ := rig.env.Approvals.Get(tenOf(o.tenant), o.tenant, o.a.ID)
		rq := mustGetRequest(t, rig, o.tenant, o.r.ID)
		if got.Status != domain.ApprovalStatusExpired || rq.Status != domain.RequestStatusRequestBacklog || rq.ReturnReason != "approval_expired" || rq.ReturnedFromStage != domain.ReturnStagePlan {
			t.Fatalf("tenant %s: approval=%s request=%+v", o.tenant, got.Status, rq)
		}
		rows := rowsOf(rig.env, t, o.tenant, "orca.request.approval.decided")
		if len(rows) != 1 || !strings.Contains(string(rows[0].Payload), `"expired"`) {
			t.Fatalf("decided rows = %v", rows)
		}
	}
	if n, _ := rig.expire.Execute(context.Background(), 100); n != 0 {
		t.Fatalf("a second sweep must find nothing of ours, expired %d", n)
	}
}

func flowExpireRacesApprove(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	rig.setPolicy(t, tenantID, func(p *domain.ApprovalPolicy) { d := time.Second; p.DueAfter = &d })
	r := rig.planRequest(t, tenantID)
	ctx := asUser(tenantID, r.ReporterID, "user")
	a, err := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1300 * time.Millisecond)
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	var decideErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		decideErr = retryDeadlock(func() error {
			_, err := rig.decide.Execute(tctx, usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
			return err
		})
	}()
	go func() {
		defer wg.Done()
		_ = retryDeadlock(func() error { _, err := rig.expire.Execute(context.Background(), 100); return err })
	}()
	wg.Wait()
	if tctx.Err() != nil {
		t.Fatal("stalled")
	}
	if c := errCode(decideErr); c != "REQUEST_APPROVAL_EXPIRED" && c != "REQUEST_APPROVAL_ALREADY_DECIDED" {
		t.Fatalf("a late approver must see EXPIRED or ALREADY_DECIDED, got %v", decideErr)
	}
	got, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
	if got.Status != domain.ApprovalStatusExpired {
		t.Fatalf("final status %s", got.Status)
	}
	if n := len(rowsOf(rig.env, t, tenantID, "orca.request.approval.decided")); n != 1 {
		t.Fatalf("decided events = %d", n)
	}
}

func flowReminderOnce(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	rig.setPolicy(t, tenantID, func(p *domain.ApprovalPolicy) { d := 6 * time.Second; p.DueAfter = &d })
	r := rig.planRequest(t, tenantID)
	ctx := asUser(tenantID, r.ReporterID, "user")
	a, err := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := rig.remind.Execute(context.Background(), 100); n != 0 {
		// other tests' approvals may be due; ours must not be
		got, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
		if got.RemindedAt != nil {
			t.Fatal("reminded before 75% of the window")
		}
	}
	time.Sleep(4800 * time.Millisecond) // 75% of 6s is 4.5s, the deadline is still 1.2s away
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = retryDeadlock(func() error { _, err := rig.remind.Execute(context.Background(), 100); return err })
		}()
	}
	wg.Wait()
	reminders := 0
	for _, row := range rowsOf(rig.env, t, tenantID, "orca.request.approval.requested") {
		var p domain.ApprovalRequestedPayload // jsonb text output re-spaces keys, so decode instead of matching text
		if json.Unmarshal(row.Payload, &p) == nil && p.Reason == "reminder" {
			reminders++
		}
	}
	if reminders != 1 {
		t.Fatalf("two sweepers must produce exactly one reminder, got %d", reminders)
	}
	got, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
	if got.RemindedAt == nil || got.Status != domain.ApprovalStatusPending {
		t.Fatalf("approval = %+v", got)
	}
}

func flowTeamPolicy(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	member, outsider := uuid.NewString(), uuid.NewString()
	teamID := "team-" + uuid.NewString()[:8]
	rig.teams.members[teamID], rig.teams.teamsOf[member] = []string{member, uuid.NewString()}, []string{teamID}
	rig.setPolicy(t, tenantID, func(p *domain.ApprovalPolicy) {
		p.Approvers = []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: teamID}}
	})
	r := rig.planRequest(t, tenantID)
	a, err := rig.open.Execute(asUser(tenantID, r.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rig.decide.Execute(asUser(tenantID, outsider, "user"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); errCode(err) != "REQUEST_APPROVAL_NOT_APPROVER" {
		t.Fatalf("outsider = %v", err)
	}
	if _, err := rig.decide.Execute(asUser(tenantID, member, "user"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatalf("team member = %v", err)
	}
}

func flowSnapshotImmuneToPolicyEdit(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	userA, userB := uuid.NewString(), uuid.NewString()
	rig.setPolicy(t, tenantID, func(p *domain.ApprovalPolicy) {
		p.Approvers = []domain.Principal{{Kind: domain.PrincipalKindUser, ID: userA}}
	})
	r := rig.planRequest(t, tenantID)
	a, err := rig.open.Execute(asUser(tenantID, r.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	list, _, _ := rig.env.Policies.List(tenOf(tenantID), tenantID, usecase.PolicyListFilter{PageSize: 10})
	changed := list[0]
	changed.Approvers = []domain.Principal{{Kind: domain.PrincipalKindUser, ID: userB}}
	if _, err := rig.env.Policies.Upsert(tenOf(tenantID), changed, changed.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := rig.decide.Execute(asUser(tenantID, userB, "user"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); errCode(err) != "REQUEST_APPROVAL_NOT_APPROVER" {
		t.Fatalf("the new approver was not on the snapshot: %v", err)
	}
	if _, err := rig.decide.Execute(asUser(tenantID, userA, "user"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatalf("the snapshotted approver still decides: %v", err)
	}
}

func flowPreDeployNoEligible(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	requester := uuid.NewString()
	rig.admins.admins = []string{requester}
	r := createRequest(t, rig.env.Env, tenOf(tenantID), func(rq *domain.Request) {
		rq.Status, rq.Type, rq.Size, rq.ReporterID = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeHotfix, domain.RequestSizeS, requester
	})
	_, err := rig.open.Execute(asUser(tenantID, requester, "admin"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPreDeploy})
	if errCode(err) != "REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER" {
		t.Fatalf("a pre_deploy where the requester is the only admin = %v", err)
	}
	list, _, _ := rig.env.Approvals.List(tenOf(tenantID), tenantID, usecase.ApprovalListFilter{RequestID: r.ID, PageSize: 10})
	if len(list) != 0 {
		t.Fatalf("NO_ELIGIBLE_APPROVER must not leave a row, found %d", len(list))
	}
	rig.admins.admins = []string{requester, uuid.NewString()}
	if _, err := rig.open.Execute(asUser(tenantID, requester, "admin"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPreDeploy}); err != nil {
		t.Fatalf("with a second admin it opens: %v", err)
	}
}

func flowCrossTenant(t *testing.T, rig *approvalRig) {
	tenantA, tenantB := newTenant(), newTenant()
	r := rig.planRequest(t, tenantA)
	a, err := rig.open.Execute(asUser(tenantA, r.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	spy := asUser(tenantB, r.ReporterID, "admin")
	if _, err := rig.decide.Execute(spy, usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); errCode(err) != "REQUEST_APPROVAL_NOT_FOUND" {
		t.Fatalf("decide across tenants = %v", err)
	}
	if _, err := rig.cancel.Execute(spy, a.ID, "x"); errCode(err) != "REQUEST_APPROVAL_NOT_FOUND" {
		t.Fatalf("cancel across tenants = %v", err)
	}
	if _, err := rig.extend.Execute(spy, usecase.ExtendApprovalInput{ID: a.ID, ExtendSeconds: 60}); errCode(err) != "REQUEST_APPROVAL_NOT_FOUND" {
		t.Fatalf("extend across tenants = %v", err)
	}
	if _, err := rig.open.Execute(spy, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan}); errCode(err) != "REQUEST_NOT_FOUND" {
		t.Fatalf("open on a foreign request = %v", err)
	}
	if list, _, _ := rig.pending.Execute(spy, "", 50, ""); len(list) != 0 {
		t.Fatal("inbox leaked across tenants")
	}
}

func flowRequestTypeConfirm(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	r := createRequest(t, rig.env.Env, tenOf(tenantID), func(rq *domain.Request) {
		rq.Status, rq.Type, rq.TypeSource, rq.Size, rq.Urgency = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeBug, domain.TypeSourceAI, domain.RequestSizeM, domain.UrgencyNormal
	})
	tr := usecase.NewTransitionRequest(rig.env.Requests, rig.env.TxScope, rig.env.Outbox)
	rig.typeH.Confirm = usecase.NewConfirmRequestType(rig.env.Requests, rig.env.History, tr, rig.recorder, rig.env.Tx, rig.env.Outbox)
	if err := rig.recorder.RequestTypeApproval(tenOf(tenantID), r.ID); err != nil {
		t.Fatal(err)
	}
	list, _, _ := rig.env.Approvals.List(tenOf(tenantID), tenantID, usecase.ApprovalListFilter{RequestID: r.ID, Status: domain.ApprovalStatusPending, PageSize: 10})
	if len(list) != 1 || list[0].SubjectType != domain.SubjectRequestType || list[0].RequestedBy != "system" {
		t.Fatalf("pending = %+v", list)
	}
	a := list[0]
	res, err := rig.decide.Execute(asUser(tenantID, r.ReporterID, "user"), usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestStatus != domain.RequestStatusAnalyzing || res.Approval.Status != domain.ApprovalStatusApproved {
		t.Fatalf("result = %+v", res)
	}
	if n := len(rowsOf(rig.env, t, tenantID, "orca.request.request.type_confirmed")); n != 1 {
		t.Fatalf("type_confirmed events = %d", n)
	}
	if n := len(rowsOf(rig.env, t, tenantID, "orca.request.approval.decided")); n != 1 {
		t.Fatalf("decided events = %d (confirm must not settle the approval a second time)", n)
	}
}

func flowRequestTypeDirectConfirm(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	r := createRequest(t, rig.env.Env, tenOf(tenantID), func(rq *domain.Request) {
		rq.Status, rq.Type, rq.TypeSource, rq.Size, rq.Urgency = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeTask, domain.TypeSourceAI, domain.RequestSizeS, domain.UrgencyNormal
	})
	tr := usecase.NewTransitionRequest(rig.env.Requests, rig.env.TxScope, rig.env.Outbox)
	confirm := usecase.NewConfirmRequestType(rig.env.Requests, rig.env.History, tr, rig.recorder, rig.env.Tx, rig.env.Outbox)
	if err := rig.recorder.RequestTypeApproval(tenOf(tenantID), r.ID); err != nil {
		t.Fatal(err)
	}
	actor := uuid.NewString()
	if _, err := confirm.Execute(asUser(tenantID, actor, "user"), usecase.ConfirmInput{
		RequestID: r.ID, Type: "task", Size: "S", Urgency: "normal", ActorID: actor, ActorKind: domain.ActorKindUser,
	}); err != nil {
		t.Fatal(err)
	}
	list, _, _ := rig.env.Approvals.List(tenOf(tenantID), tenantID, usecase.ApprovalListFilter{RequestID: r.ID, PageSize: 10})
	if len(list) != 1 || list[0].Status != domain.ApprovalStatusApproved || list[0].DecidedBy == nil || *list[0].DecidedBy != actor {
		t.Fatalf("direct confirmation must settle the pending approval: %+v", list)
	}
	if got := mustGetRequest(t, rig, tenantID, r.ID).Status; got != domain.RequestStatusPlanning && got != domain.RequestStatusAnalyzing {
		t.Fatalf("request status %s", got)
	}
}

func flowPendingForUser(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	member, outsider := uuid.NewString(), uuid.NewString()
	teamID := "team-" + uuid.NewString()[:8]
	rig.teams.members[teamID], rig.teams.teamsOf[member] = []string{member}, []string{teamID}
	rig.setPolicy(t, tenantID, func(p *domain.ApprovalPolicy) {
		p.Approvers = []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: teamID}}
		p.AllowRequesterApprove = true
	})
	r := rig.planRequest(t, tenantID)
	if _, err := rig.open.Execute(asUser(tenantID, r.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan}); err != nil {
		t.Fatal(err)
	}
	for who, want := range map[string]int{member: 1, outsider: 0} {
		list, _, err := rig.pending.Execute(asUser(tenantID, who, "user"), "", 50, "")
		if err != nil || len(list) != want {
			t.Fatalf("%s sees %d, want %d (%v)", who, len(list), want, err)
		}
	}
	list, _, _ := rig.pending.Execute(asUser(tenantID, outsider, "admin"), "", 50, "")
	if len(list) != 1 || list[0].RequestNumber != r.Number || list[0].RequestTitle != r.Title {
		t.Fatalf("admin inbox = %+v", list)
	}
}

func flowExtend(t *testing.T, rig *approvalRig) {
	tenantID := newTenant()
	rig.setPolicy(t, tenantID, func(p *domain.ApprovalPolicy) { d := 2 * time.Second; p.DueAfter = &d })
	r := rig.planRequest(t, tenantID)
	ctx := asUser(tenantID, r.ReporterID, "user")
	a, err := rig.open.Execute(ctx, usecase.OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	ext, err := rig.extend.Execute(ctx, usecase.ExtendApprovalInput{ID: a.ID, ExtendSeconds: 3600, ExpectedVersion: a.Version})
	if err != nil || !ext.DueAt.After(a.DueAt.Add(59*time.Minute)) || ext.Version != 2 {
		t.Fatalf("extend = %+v %v", ext, err)
	}
	time.Sleep(2300 * time.Millisecond)
	if _, err := rig.expire.Execute(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	got, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
	if got.Status != domain.ApprovalStatusPending {
		t.Fatalf("an extended approval must survive the original deadline, status %s", got.Status)
	}
	if _, err := rig.extend.Execute(asUser(tenantID, uuid.NewString(), "user"), usecase.ExtendApprovalInput{ID: a.ID, ExtendSeconds: 60}); errCode(err) != "REQUEST_APPROVAL_FORBIDDEN" {
		t.Fatalf("a stranger extending = %v", err)
	}
}
