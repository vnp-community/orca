package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type projectScopeKey struct{}

type accessProjectKey struct{}

// AccessProject is the project of the Request (or project id) the authorized RPC targets, "" when unknown.
// Later interceptors use it for per-project limits without a second lookup.
func AccessProject(ctx context.Context) string {
	p, _ := ctx.Value(accessProjectKey{}).(string)
	return p
}

// WithProjectScope limits list RPCs to these projects (the caller's memberships); a nil slice means unrestricted.
func WithProjectScope(ctx context.Context, projectIDs []string) context.Context {
	return context.WithValue(ctx, projectScopeKey{}, append([]string{}, projectIDs...))
}

// ProjectScope returns the projects a list may cover and whether the scope applies.
func ProjectScope(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(projectScopeKey{}).([]string)
	return v, ok
}

// AuthorizeRequestAction decides whether the caller may run an RPC against the Request it targets.
// Roles come from project-service and requests.reporter_id; the matrix lives in request.rego.
type AuthorizeRequestAction struct {
	requests  RequestReader
	approvals ApprovalLocator
	entities  map[string]EntityRequestResolver
	roles     ProjectRoleResolver
	policy    AccessPolicy
	audit     AccessAuditor
}

func NewAuthorizeRequestAction(requests RequestReader, approvals ApprovalLocator, roles ProjectRoleResolver, policy AccessPolicy, audit AccessAuditor) *AuthorizeRequestAction {
	return &AuthorizeRequestAction{requests: requests, approvals: approvals, roles: roles, policy: policy, audit: audit, entities: map[string]EntityRequestResolver{}}
}

// RegisterEntity wires the resolver of a child entity kind named in the RPC catalog.
func (a *AuthorizeRequestAction) RegisterEntity(kind string, r EntityRequestResolver) {
	a.entities[kind] = r
}

// Authorize returns a ctx that may carry a ProjectScope for list RPCs. targetID is the value of the
// catalog Field in the request message ("" when absent).
func (a *AuthorizeRequestAction) Authorize(ctx context.Context, method, targetID string) (context.Context, error) {
	e, ok := domain.Catalog[method]
	if !ok {
		return ctx, domain.ErrRequestForbidden()
	}
	if e.Group == domain.GroupInternal {
		return ctx, nil // internalcaller.Guard has already admitted the service token.
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil || tenantID == "" {
		return ctx, domain.ErrRequestTenantRequired()
	}
	userID, _ := tenant.UserID(ctx)
	if userID == "" {
		return ctx, apperrors.New(apperrors.KindUnauthenticated, "REQUEST_NO_USER", "no user in request context", nil)
	}
	globalRole, _ := tenant.Role(ctx)
	in := domain.AccessInput{
		Action: e.Group, RPC: domain.RPCName(method), CallerGlobalRole: globalRole, ActorType: tenant.ActorType(ctx),
	}
	isAdmin := globalRole == "admin"
	requestID := ""

	switch e.Group {
	case domain.GroupAdmin, domain.GroupAuthenticated:
		// No Request to locate.
	default:
		var t accessTarget
		if t, err = a.locate(ctx, e, tenantID, userID, targetID, isAdmin); err != nil {
			return ctx, err
		}
		ctx, requestID, in.CallerProjectRole, in.IsReporter = t.ctx, t.requestID, t.projectRole, t.reporter
	}

	allowed, err := a.policy.Allow(ctx, in)
	if err != nil {
		return ctx, domain.ErrRequestPolicyUnavailable(err)
	}
	if !allowed {
		a.audit.RecordDenied(ctx, DeniedAccess{RPC: in.RPC, RequestID: requestID, ActorType: in.ActorType})
		return ctx, domain.ErrRequestForbidden()
	}
	return ctx, nil
}

type accessTarget struct {
	ctx         context.Context
	requestID   string
	projectRole string
	reporter    bool
}

// locate resolves project role and reporter flag for the targeted Request or project. A Request that
// does not exist and one of another tenant are both REQUEST_NOT_FOUND (the repository is tenant scoped).
func (a *AuthorizeRequestAction) locate(ctx context.Context, e domain.Entry, tenantID, userID, targetID string, isAdmin bool) (accessTarget, error) {
	t := accessTarget{ctx: ctx}
	projectID := ""
	switch e.Locator {
	case domain.LocRequestID, domain.LocApprovalID, domain.LocEntityID:
		id := targetID
		if id == "" {
			return t, apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ID_REQUIRED", "the target id is required", nil)
		}
		var err error
		if e.Locator == domain.LocApprovalID {
			if id, err = a.approvals.RequestIDOf(ctx, id); err != nil {
				return t, err
			}
		}
		if e.Locator == domain.LocEntityID {
			r, ok := a.entities[e.Entity]
			if !ok {
				if isAdmin {
					return t, nil // admin passes; the handler reports the RPC unimplemented.
				}
				return t, domain.ErrRequestNotFound(id)
			}
			if id, err = r.RequestIDOf(ctx, id); err != nil {
				return t, err
			}
		}
		req, err := a.requests.Get(ctx, id)
		if err != nil {
			return t, err
		}
		t.requestID, projectID, t.reporter = req.ID, req.ProjectID, req.ReporterID == userID
	case domain.LocProjectID:
		projectID = targetID
		if projectID == "" && e.Group == domain.GroupRead {
			scoped, err := a.scopeToMemberProjects(ctx, tenantID, userID, isAdmin)
			if err != nil {
				return t, err
			}
			// Every project in scope is one the caller belongs to, so the read check runs as a member.
			t.ctx, t.projectRole = scoped, "member"
			return t, nil
		}
	}
	if projectID != "" {
		t.ctx = context.WithValue(t.ctx, accessProjectKey{}, projectID)
	}
	if isAdmin || projectID == "" || e.Group == domain.GroupDecide {
		return t, nil
	}
	role, err := a.roles.RoleOf(ctx, tenantID, projectID, userID)
	if err != nil {
		var ae *apperrors.AppError
		if errors.As(err, &ae) {
			return t, err
		}
		return t, domain.ErrProjectRoleUnavailable(err)
	}
	t.projectRole = role
	return t, nil
}

func (a *AuthorizeRequestAction) scopeToMemberProjects(ctx context.Context, tenantID, userID string, isAdmin bool) (context.Context, error) {
	if isAdmin {
		return ctx, nil
	}
	ids, err := a.roles.ProjectsOf(ctx, tenantID, userID)
	if err != nil {
		return ctx, domain.ErrProjectRoleUnavailable(err)
	}
	return WithProjectScope(ctx, ids), nil
}
