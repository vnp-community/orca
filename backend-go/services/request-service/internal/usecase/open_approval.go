package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const systemActor = "system"

type OpenApprovalInput struct {
	RequestID   string
	SubjectType domain.SubjectType
	// SubjectID is optional; handlers that own several subjects per Request (phases, pre-deploy gates) read it.
	SubjectID      string
	IdempotencyKey *string
	// RequestedBy overrides the ctx user, for system-opened approvals (classification proposals).
	RequestedBy string
}

// OpenApproval opens one pending approval per subject. It runs in the caller's transaction or its own, and
// takes the Request lock first (lock order: Request, then Approval).
type OpenApproval struct {
	Repo ApprovalRepository
	// Tx tells whether the caller already holds a transaction: only then directory lookups cannot happen before it.
	Tx TxScope
	// Requests reads the Request before the transaction so directory lookups (gRPC) stay out of it.
	Requests     RequestReader
	Locker       RequestLocker
	Registry     *SubjectHandlerRegistry
	Resolver     *ResolveApproverPolicy
	ApproverRepo ApprovalApproverRepository
	Outbox       OutboxWriter
	Teams        TeamMembershipResolver
	Admins       AdminDirectoryResolver
	Log          *slog.Logger
}

func (uc *OpenApproval) Execute(ctx context.Context, in OpenApprovalInput) (*domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if !in.SubjectType.Valid() {
		return nil, domain.ErrApprovalSubjectTypeInvalid
	}
	handler := uc.Registry.Get(in.SubjectType)
	if handler == nil {
		return nil, domain.ErrApprovalSubjectTypeNotAllowed
	}
	requestedBy := in.RequestedBy
	if requestedBy == "" {
		requestedBy, _ = tenant.UserID(ctx)
	}
	if requestedBy == "" {
		requestedBy = systemActor
	}

	// No outbound calls inside the transaction: look the directories up first and replay the answers inside it.
	var pre *openPrefetch
	if !uc.Tx.InTransaction(ctx) {
		pre = uc.prefetch(ctx, tenantID, in, requestedBy)
	}
	for attempt := 0; ; attempt++ {
		out, err := uc.open(ctx, tenantID, in, requestedBy, handler, pre)
		if errors.Is(err, errPrefetchStale) && attempt < 2 {
			pre = uc.prefetch(ctx, tenantID, in, requestedBy) // the policy changed between the lookup and the lock
			continue
		}
		if err != nil {
			return nil, err
		}
		return &out, nil
	}
}

func (uc *OpenApproval) open(ctx context.Context, tenantID string, in OpenApprovalInput, requestedBy string, handler SubjectHandler, pre *openPrefetch) (domain.Approval, error) {
	var out domain.Approval
	err := uc.Tx.InTx(ctx, func(ctx context.Context) error {
		req, err := uc.Locker.LockRequest(ctx, in.RequestID)
		if err != nil {
			return err
		}
		if in.IdempotencyKey != nil {
			prior, err := uc.Repo.FindByIdempotencyKey(ctx, tenantID, *in.IdempotencyKey)
			if err != nil {
				return err
			}
			if prior != nil {
				if prior.RequestID != req.ID || prior.SubjectType != in.SubjectType {
					return domain.ErrApprovalIdempotency
				}
				out = *prior
				return nil
			}
		}
		if err := uc.checkFlow(req, in.SubjectType); err != nil {
			return err
		}
		if in.SubjectID != "" {
			ctx = WithRequestedSubjectID(ctx, in.SubjectID)
		}
		subjectID, digest, err := handler.ValidateForRequest(ctx, ctx, req, in.SubjectType)
		if err != nil {
			return err
		}
		// Pre-check instead of catching the unique-index error: on Postgres a failed INSERT aborts the transaction.
		existing, err := uc.Repo.FindPendingBySubject(ctx, tenantID, in.SubjectType, subjectID)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.SubjectDigest != digest {
				return domain.ErrApprovalPendingExists
			}
			out = *existing
			return nil
		}

		policy, err := uc.Resolver.Resolve(ctx, req, in.SubjectType)
		if err != nil {
			return err
		}
		if !policy.AllowRequesterApprove {
			lookup := directoryLookup(uc.liveLookup(ctx, tenantID))
			if pre != nil {
				if pre.signature != policySignature(policy) {
					return errPrefetchStale
				}
				lookup = pre
			}
			if !uc.hasEligibleApprover(lookup, policy.Approvers, req.ReporterID, requestedBy) {
				return domain.ErrApprovalNoEligibleApprover
			}
		}
		now, err := uc.Repo.NowDB(ctx)
		if err != nil {
			return err
		}
		var dueAt *time.Time
		if policy.DueAfter != nil {
			t := now.Add(*policy.DueAfter)
			dueAt = &t
		}
		a := domain.Approval{
			ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, SubjectType: in.SubjectType, SubjectID: subjectID,
			Stage: string(req.Status), Status: domain.ApprovalStatusPending, RequestedBy: requestedBy, DueAt: dueAt, Version: 1,
			SubjectDigest: digest, SelfApprovalAllowed: policy.AllowRequesterApprove, IdempotencyKey: in.IdempotencyKey,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.Repo.Insert(ctx, a); err != nil {
			return err
		}
		if err := uc.ApproverRepo.InsertSnapshot(ctx, a.ID, tenantID, policy.Approvers); err != nil {
			return err
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectApprovalRequested, domain.NewApprovalRequestedPayload(a, req, ""))
		if err != nil {
			return err
		}
		if err := uc.Outbox.InsertOutboxEvent(ctx, ev); err != nil {
			return err
		}
		out = a
		return nil
	})
	return out, err
}

// checkFlow rejects a subject the Request's type has no gate for, and one that cannot be open in the current status.
func (uc *OpenApproval) checkFlow(req domain.Request, st domain.SubjectType) error {
	var flow domain.FlowDefinition
	if req.Type != "" {
		f, err := domain.FlowFor(req.Type)
		if err != nil {
			return err
		}
		flow = f
	}
	if !domain.ApprovalSubjectAllowedByFlow(flow, req.Size, req.Type != "", st) {
		return domain.ErrApprovalSubjectTypeNotAllowed
	}
	if !domain.ApprovalAllowedInStatus(st, req.Status) {
		return domain.ErrApprovalStageMismatch
	}
	return nil
}

var errPrefetchStale = errors.New("approval policy changed since the directory lookup")

// directoryLookup is how eligibility reads team members and admins: live (resolvers) or replayed from a prefetch.
type directoryLookup interface {
	Members(team string) ([]string, error)
	Admins() ([]string, error)
}

type liveLookup struct {
	ctx      context.Context
	tenantID string
	teams    TeamMembershipResolver
	admins   AdminDirectoryResolver
}

func (uc *OpenApproval) liveLookup(ctx context.Context, tenantID string) liveLookup {
	return liveLookup{ctx: ctx, tenantID: tenantID, teams: uc.Teams, admins: uc.Admins}
}

func (l liveLookup) Members(team string) ([]string, error) {
	if l.teams == nil {
		return nil, domain.ErrApprovalDirectoryUnavailable
	}
	return l.teams.MembersOfTeam(l.ctx, team)
}

func (l liveLookup) Admins() ([]string, error) {
	if l.admins == nil {
		return nil, domain.ErrApprovalDirectoryUnavailable
	}
	return l.admins.ListAdmins(l.ctx, l.tenantID)
}

type lookupResult struct {
	users []string
	err   error
}

// openPrefetch records the directory answers needed by the policy resolved before the transaction.
type openPrefetch struct {
	signature string
	members   map[string]lookupResult
	admins    *lookupResult
}

func (p *openPrefetch) Members(team string) ([]string, error) {
	if r, ok := p.members[team]; ok {
		return r.users, r.err
	}
	return nil, domain.ErrApprovalDirectoryUnavailable
}

func (p *openPrefetch) Admins() ([]string, error) {
	if p.admins == nil {
		return nil, domain.ErrApprovalDirectoryUnavailable
	}
	return p.admins.users, p.admins.err
}

func policySignature(p domain.ApprovalPolicy) string {
	return fmt.Sprintf("%v|%s", p.AllowRequesterApprove, strings.Join(domain.ApproverStrings(p.Approvers), ","))
}

// prefetch resolves the policy and the directory data its eligibility check needs, outside any transaction.
// Failures are recorded, not returned: eligibility treats a failed lookup as "assume eligible".
func (uc *OpenApproval) prefetch(ctx context.Context, tenantID string, in OpenApprovalInput, requestedBy string) *openPrefetch {
	if uc.Requests == nil {
		return nil
	}
	req, err := uc.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return nil // the transaction reports the real error
	}
	policy, err := uc.Resolver.Resolve(ctx, req, in.SubjectType)
	if err != nil || policy.AllowRequesterApprove {
		return &openPrefetch{signature: policySignature(policy)}
	}
	p := &openPrefetch{signature: policySignature(policy), members: map[string]lookupResult{}}
	live := uc.liveLookup(ctx, tenantID)
	for _, pr := range policy.Approvers {
		switch {
		case pr.Kind == domain.PrincipalKindTeam:
			users, err := live.Members(pr.ID)
			p.members[pr.ID] = lookupResult{users, err}
		case pr.Kind == domain.PrincipalKindRole && pr.ID == "admin" && p.admins == nil:
			users, err := live.Admins()
			p.admins = &lookupResult{users, err}
		}
	}
	return p
}

// hasEligibleApprover answers "could anyone other than the requester decide?" when self-approval is off.
// A directory outage counts as eligible: opening must not fail because auth-service or tenant-service is down;
// the decision itself is still checked against the snapshot.
func (uc *OpenApproval) hasEligibleApprover(lookup directoryLookup, approvers []domain.Principal, reporterID, requestedBy string) bool {
	isRequester := func(u string) bool { return u == reporterID || (u == requestedBy && requestedBy != systemActor) }
	for _, p := range approvers {
		switch p.Kind {
		case domain.PrincipalKindUser:
			if !isRequester(p.ID) {
				return true
			}
		case domain.PrincipalKindTeam:
			members, err := lookup.Members(p.ID)
			if err != nil {
				uc.warn("team lookup failed while opening approval; assuming an eligible approver", err)
				return true
			}
			if anyNotRequester(members, isRequester) {
				return true
			}
		case domain.PrincipalKindRole:
			if p.ID != "admin" {
				return true
			}
			admins, err := lookup.Admins()
			if err != nil {
				uc.warn("admin lookup failed while opening approval; assuming an eligible approver", err)
				return true
			}
			if anyNotRequester(admins, isRequester) {
				return true
			}
		}
	}
	return false
}

func anyNotRequester(users []string, isRequester func(string) bool) bool {
	for _, u := range users {
		if !isRequester(u) {
			return true
		}
	}
	return false
}

func (uc *OpenApproval) warn(msg string, err error) {
	log := uc.Log
	if log == nil {
		log = slog.Default()
	}
	log.Warn(msg, slog.Any("error", err))
}
