package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// apprStore adds in-memory approvals on top of lcStore (requests, outbox, transactions with rollback).
type apprStore struct {
	*lcStore
	amu       sync.Mutex
	approvals map[string]domain.Approval
	approvers map[string][]domain.Principal
	policies  []domain.ApprovalPolicy
	now       time.Time
	calls     []string // lock order and tenant-switch log
}

func newApprStore() *apprStore {
	return &apprStore{
		lcStore: newLcStore(), approvals: map[string]domain.Approval{}, approvers: map[string][]domain.Principal{},
		now: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC),
	}
}

func (s *apprStore) cloneApprovals() (map[string]domain.Approval, map[string][]domain.Principal) {
	a := make(map[string]domain.Approval, len(s.approvals))
	for k, v := range s.approvals {
		a[k] = v
	}
	p := make(map[string][]domain.Principal, len(s.approvers))
	for k, v := range s.approvers {
		p[k] = append([]domain.Principal(nil), v...)
	}
	return a, p
}

// InTx rolls approvals back together with the lcStore state when fn fails.
func (s *apprStore) InTx(ctx context.Context, fn func(context.Context) error) error {
	if s.InTransaction(ctx) {
		return fn(ctx)
	}
	var savedA map[string]domain.Approval
	var savedP map[string][]domain.Principal
	err := s.lcStore.InTx(ctx, func(txCtx context.Context) error {
		savedA, savedP = s.cloneApprovals()
		callsBefore := len(s.calls)
		err := fn(txCtx)
		if err != nil {
			s.approvals, s.approvers = savedA, savedP
			s.calls = s.calls[:callsBefore]
		}
		return err
	})
	return err
}

func (s *apprStore) log(entry string) { s.calls = append(s.calls, entry) }

type apprRepo struct{ s *apprStore }

func (r apprRepo) Insert(ctx context.Context, a domain.Approval) error {
	for _, e := range r.s.approvals {
		if e.Status == domain.ApprovalStatusPending && a.Status == domain.ApprovalStatusPending && e.SubjectType == a.SubjectType && e.SubjectID == a.SubjectID {
			return ErrPendingExists
		}
	}
	r.s.approvals[a.ID] = a
	return nil
}
func (r apprRepo) Get(_ context.Context, tenantID, id string) (domain.Approval, error) {
	a, ok := r.s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return domain.Approval{}, ErrApprovalNotFound
	}
	return a, nil
}
func (r apprRepo) GetForUpdate(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	r.s.log("lock:approval")
	return r.Get(ctx, tenantID, id)
}
func (r apprRepo) FindPendingBySubject(_ context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error) {
	for _, a := range r.s.approvals {
		if a.TenantID == tenantID && a.Status == domain.ApprovalStatusPending && a.SubjectType == st && a.SubjectID == subjectID {
			c := a
			return &c, nil
		}
	}
	return nil, nil
}
func (r apprRepo) FindByIdempotencyKey(_ context.Context, tenantID, key string) (*domain.Approval, error) {
	for _, a := range r.s.approvals {
		if a.TenantID == tenantID && a.IdempotencyKey != nil && *a.IdempotencyKey == key {
			c := a
			return &c, nil
		}
	}
	return nil, nil
}
func (r apprRepo) UpdateDecision(_ context.Context, a domain.Approval, expected int64) (bool, error) {
	cur, ok := r.s.approvals[a.ID]
	if !ok || cur.Status != domain.ApprovalStatusPending || cur.Version != expected {
		return false, nil
	}
	a.Version = expected + 1
	r.s.approvals[a.ID] = a
	return true, nil
}
func (r apprRepo) UpdateDue(_ context.Context, a domain.Approval, expected int64) (bool, error) {
	cur, ok := r.s.approvals[a.ID]
	if !ok || cur.Status != domain.ApprovalStatusPending || cur.Version != expected {
		return false, nil
	}
	cur.DueAt, cur.RemindedAt, cur.Version = a.DueAt, nil, expected+1
	r.s.approvals[a.ID] = cur
	return true, nil
}
func (r apprRepo) MarkReminded(_ context.Context, tenantID, id string, at time.Time) (bool, error) {
	a, ok := r.s.approvals[id]
	if !ok || a.TenantID != tenantID || a.Status != domain.ApprovalStatusPending || a.RemindedAt != nil {
		return false, nil
	}
	a.RemindedAt = &at
	r.s.approvals[id] = a
	return true, nil
}
func (r apprRepo) UpdatePendingDigest(context.Context, string, domain.SubjectType, string, string) (bool, error) {
	return false, errors.New("unused")
}
func (r apprRepo) CancelPendingForRequest(_ context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error) {
	var out []domain.Approval
	for id, a := range r.s.approvals {
		if a.TenantID == tenantID && a.RequestID == requestID && a.Status == domain.ApprovalStatusPending {
			a.Status, a.Comment, a.Version, a.DecidedAt = domain.ApprovalStatusCancelled, why, a.Version+1, &now
			r.s.approvals[id] = a
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (r apprRepo) List(_ context.Context, tenantID string, f ApprovalListFilter) ([]domain.Approval, string, error) {
	var out []domain.Approval
	for _, a := range r.s.approvals {
		if a.TenantID != tenantID || (f.RequestID != "" && a.RequestID != f.RequestID) || (f.Status != "" && a.Status != f.Status) ||
			(f.SubjectType != "" && a.SubjectType != f.SubjectType) {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, "", nil
}
func (r apprRepo) ListPendingForUser(_ context.Context, tenantID string, f PendingForUserFilter) ([]PendingApproval, string, error) {
	var out []PendingApproval
	for _, a := range r.s.approvals {
		if a.TenantID != tenantID || a.Status != domain.ApprovalStatusPending {
			continue
		}
		req := r.s.requests[a.RequestID]
		if !a.SelfApprovalAllowed && (req.ReporterID == f.UserID || a.RequestedBy == f.UserID) {
			continue
		}
		if f.Role != "admin" && !matchesSnapshot(r.s.approvers[a.ID], f, req.ReporterID) {
			continue
		}
		out = append(out, PendingApproval{Approval: a, RequestTitle: req.Title, RequestNumber: req.Number})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, "", nil
}

func matchesSnapshot(ps []domain.Principal, f PendingForUserFilter, reporter string) bool {
	for _, p := range ps {
		switch p.Kind {
		case domain.PrincipalKindUser:
			if p.ID == f.UserID {
				return true
			}
		case domain.PrincipalKindRole:
			if p.ID == f.Role {
				return true
			}
		case domain.PrincipalKindReporter:
			if reporter == f.UserID {
				return true
			}
		case domain.PrincipalKindTeam:
			for _, t := range f.TeamIDs {
				if t == p.ID {
					return true
				}
			}
		}
	}
	return false
}

// ClaimDue returns due approvals of every tenant, like the real cross-tenant scan.
func (r apprRepo) ClaimDue(context.Context, int) ([]ApprovalClaim, error) {
	return r.claims(func(a domain.Approval) bool { return a.DueAt != nil && !r.s.now.Before(*a.DueAt) }), nil
}
func (r apprRepo) ClaimDueForReminder(context.Context, int) ([]ApprovalClaim, error) {
	return r.claims(func(a domain.Approval) bool {
		if a.DueAt == nil || a.RemindedAt != nil || !r.s.now.Before(*a.DueAt) {
			return false
		}
		thresh := a.CreatedAt.Add(a.DueAt.Sub(a.CreatedAt) * 3 / 4)
		return !r.s.now.Before(thresh)
	}), nil
}
func (r apprRepo) claims(match func(domain.Approval) bool) []ApprovalClaim {
	var out []ApprovalClaim
	for _, a := range r.s.approvals {
		if a.Status == domain.ApprovalStatusPending && match(a) {
			out = append(out, ApprovalClaim{TenantID: a.TenantID, ApprovalID: a.ID, RequestID: a.RequestID})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ApprovalID < out[j].ApprovalID })
	return out
}
func (r apprRepo) NowDB(context.Context) (time.Time, error) { return r.s.now, nil }

type apprApprovers struct{ s *apprStore }

func (a apprApprovers) InsertSnapshot(_ context.Context, id, _ string, ps []domain.Principal) error {
	a.s.approvers[id] = append([]domain.Principal(nil), ps...)
	return nil
}
func (a apprApprovers) ListForApproval(_ context.Context, _, id string) ([]domain.Principal, error) {
	return a.s.approvers[id], nil
}

type apprPolicies struct{ s *apprStore }

func (p apprPolicies) ListEnabledCandidates(_ context.Context, _ string, st domain.SubjectType, project, rtype, size, urgency string) ([]domain.ApprovalPolicy, error) {
	var out []domain.ApprovalPolicy
	for _, pol := range p.s.policies {
		if pol.SubjectType == st && pol.Enabled {
			out = append(out, pol)
		}
	}
	return out, nil
}
func (p apprPolicies) Get(context.Context, string, string) (domain.ApprovalPolicy, error) {
	return domain.ApprovalPolicy{}, errors.New("unused")
}
func (p apprPolicies) List(_ context.Context, _ string, _ PolicyListFilter) ([]domain.ApprovalPolicy, string, error) {
	return p.s.policies, "", nil
}
func (p apprPolicies) Upsert(_ context.Context, pol domain.ApprovalPolicy, _ int64) (domain.ApprovalPolicy, error) {
	pol.Version++
	p.s.policies = append(p.s.policies, pol)
	return pol, nil
}
func (p apprPolicies) Delete(context.Context, string, string, int64) error { return nil }
func (p apprPolicies) NowDB(context.Context) (time.Time, error)            { return p.s.now, nil }

type apprLocker struct{ s *apprStore }

func (l apprLocker) LockRequest(ctx context.Context, id string) (domain.Request, error) {
	if !l.s.InTransaction(ctx) {
		return domain.Request{}, errors.New("LockRequest outside a transaction")
	}
	tid, _ := tenant.TenantID(ctx)
	l.s.log("lock:request@" + tid)
	return l.s.lcStore.Get(ctx, id)
}

type fakeTeams struct {
	teamsOf map[string][]string
	members map[string][]string
	err     error
	calls   int
	s       *apprStore
	inTx    int    // lookups made while a transaction was open: must stay 0
	onCall  func() // runs at the start of every lookup (to simulate concurrent changes)
}

func (f *fakeTeams) note(ctx context.Context) {
	f.calls++
	if f.s != nil && f.s.InTransaction(ctx) {
		f.inTx++
	}
	if f.onCall != nil {
		f.onCall()
	}
}

func (f *fakeTeams) TeamsForUser(ctx context.Context, u string) ([]string, error) {
	f.note(ctx)
	return f.teamsOf[u], f.err
}
func (f *fakeTeams) MembersOfTeam(ctx context.Context, t string) ([]string, error) {
	f.note(ctx)
	return f.members[t], f.err
}

type fakeAdmins struct {
	admins []string
	err    error
	s      *apprStore
	inTx   int
}

func (f *fakeAdmins) ListAdmins(ctx context.Context, _ string) ([]string, error) {
	if f.s != nil && f.s.InTransaction(ctx) {
		f.inTx++
	}
	return f.admins, f.err
}

// fakeArtifacts is a SubjectArtifacts that records decisions and can fail on demand.
type fakeArtifacts struct {
	digest     string
	describeEr error
	decidedEr  error
	decided    []bool
	closed     []string
}

func (f *fakeArtifacts) Describe(_ context.Context, req domain.Request, st domain.SubjectType, hinted string) (string, string, error) {
	if f.describeEr != nil {
		return "", "", f.describeEr
	}
	id := hinted
	if id == "" {
		id = string(st) + "-" + req.ID
	}
	return id, domain.SubjectDigest(st, id, f.digest), nil
}
func (f *fakeArtifacts) Decided(_ context.Context, _ domain.Approval, approved bool) error {
	if f.decidedEr != nil {
		return f.decidedEr
	}
	f.decided = append(f.decided, approved)
	return nil
}
func (f *fakeArtifacts) Closed(_ context.Context, _ domain.Approval, why string) error {
	f.closed = append(f.closed, why)
	return nil
}

// apprEnv wires the real use cases over the fakes.
type apprEnv struct {
	s        *apprStore
	teams    *fakeTeams
	admins   *fakeAdmins
	art      *fakeArtifacts
	registry *SubjectHandlerRegistry
	open     *OpenApproval
	decide   *DecideApproval
	cancel   *CancelApproval
	expire   *ExpireApprovals
	remind   *RemindPendingApprovals
	extend   *ExtendApproval
	pending  *ListPendingApprovalsForUser
	canceler *CancelPendingApprovalsForRequest
	ret      *ReturnRequestToBacklog
}

func newApprEnv() *apprEnv {
	s := newApprStore()
	e := &apprEnv{s: s, teams: &fakeTeams{teamsOf: map[string][]string{}, members: map[string][]string{}}, admins: &fakeAdmins{}, art: &fakeArtifacts{digest: "v1"}}
	e.teams.s, e.admins.s = s, s
	e.registry = NewSubjectHandlerRegistry()
	tr := NewTransitionRequest(s, s, s)
	repo, ar, locker := apprRepo{s}, apprApprovers{s}, apprLocker{s}
	canceler := &CancelPendingApprovalsForRequest{Repo: repo, Tx: s, Locker: locker, Registry: e.registry, Outbox: s}
	e.canceler = canceler
	e.ret = NewReturnRequestToBacklog(s, tr, lcHistory{s.lcStore}, &PendingApprovalCanceller{Inner: canceler}, NoActiveExecutionGuard{}, s, s)
	for _, st := range domain.AllSubjectTypes {
		if st == domain.SubjectRequestType {
			continue
		}
		e.registry.Register(st, &TransitionSubjectHandler{Artifacts: e.art, Transition: tr, Returner: e.ret, Requests: locker})
	}
	e.registry.Register(domain.SubjectRequestType, &RequestTypeApprovalHandler{Requests: locker, Returner: e.ret, Confirm: noConfirm{}})
	e.open = &OpenApproval{Repo: repo, Tx: s, Requests: s, Locker: locker, Registry: e.registry, Resolver: &ResolveApproverPolicy{Repo: apprPolicies{s}},
		ApproverRepo: ar, Outbox: s, Teams: e.teams, Admins: e.admins}
	e.expire = &ExpireApprovals{Repo: repo, Tx: s, Locker: locker, Registry: e.registry, Returner: e.ret, Outbox: s}
	e.decide = &DecideApproval{Repo: repo, Tx: s, Locker: locker, Registry: e.registry, Outbox: s, Expirer: e.expire,
		Authorizer: &AuthorizeApprovalDecision{ApproverRepo: ar, Teams: e.teams}}
	e.cancel = &CancelApproval{Repo: repo, Tx: s, Locker: locker, Registry: e.registry, Outbox: s}
	e.remind = &RemindPendingApprovals{Repo: repo, Requests: s, Tx: s, Outbox: s}
	e.extend = &ExtendApproval{Repo: repo, Tx: s, Locker: locker}
	e.pending = &ListPendingApprovalsForUser{Repo: repo, Teams: e.teams}
	return e
}

type noConfirm struct{}

func (noConfirm) Execute(context.Context, ConfirmInput) (domain.Request, error) {
	return domain.Request{}, nil
}

func userCtx(user, role string) context.Context {
	return tenant.WithRole(tenant.WithUserID(lcCtx(), user), role)
}

// planRequest seeds a change_request waiting for plan approval.
func (e *apprEnv) planRequest() domain.Request {
	return e.s.seed(func(r *domain.Request) {
		r.Status, r.Type, r.Size, r.ReporterID = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeChangeRequest, domain.RequestSizeM, uuid.NewString()
	})
}

func (e *apprEnv) openPlan(r domain.Request, by string) (*domain.Approval, error) {
	return e.open.Execute(userCtx(by, "user"), OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectPlan})
}

func (e *apprEnv) eventSubjects() string { return strings.Join(e.s.subjects(), ",") }
