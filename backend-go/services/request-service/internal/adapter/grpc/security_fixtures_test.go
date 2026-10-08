package grpc

import (
	"context"
	"sync"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	testTenant   = "11111111-1111-4111-8111-111111111111"
	otherTenant  = "22222222-2222-4222-8222-222222222222"
	testProject  = "33333333-3333-4333-8333-333333333333"
	testRequest  = "44444444-4444-4444-8444-444444444444"
	testApproval = "55555555-5555-4555-8555-555555555555"
	testEntity   = "66666666-6666-4666-8666-666666666666"

	userAdmin    = "a0000000-0000-4000-8000-000000000001"
	userOwner    = "a0000000-0000-4000-8000-000000000002"
	userMember   = "a0000000-0000-4000-8000-000000000003"
	userReporter = "a0000000-0000-4000-8000-000000000004"
	userStranger = "a0000000-0000-4000-8000-000000000005"
)

// accessRequests serves one Request and hides it from other tenants exactly like the tenant-scoped repository.
type accessRequests struct{}

func (accessRequests) Get(ctx context.Context, id string) (domain.Request, error) {
	t, _ := tenant.TenantID(ctx)
	if id != testRequest || t != testTenant {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	return domain.Request{ID: testRequest, TenantID: testTenant, ProjectID: testProject, ReporterID: userReporter}, nil
}

type accessRoles struct {
	mu       sync.Mutex
	roles    map[string]string
	projects map[string][]string
	err      error
	calls    int
}

func newAccessRoles() *accessRoles {
	return &accessRoles{
		roles:    map[string]string{userOwner: "owner", userMember: "member"},
		projects: map[string][]string{userOwner: {testProject}, userMember: {testProject}},
	}
}

func (r *accessRoles) RoleOf(_ context.Context, _, _, userID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.roles[userID], r.err
}

func (r *accessRoles) ProjectsOf(_ context.Context, _, userID string) ([]string, error) {
	return r.projects[userID], r.err
}

type accessApprovals struct{}

func (accessApprovals) RequestIDOf(_ context.Context, id string) (string, error) {
	if id != testApproval {
		return "", domain.ErrApprovalNotFound
	}
	return testRequest, nil
}

type fixedEntity struct{}

func (fixedEntity) RequestIDOf(context.Context, string) (string, error) { return testRequest, nil }

type recordingAuditor struct {
	mu     sync.Mutex
	denied []usecase.DeniedAccess
}

func (a *recordingAuditor) RecordDenied(_ context.Context, d usecase.DeniedAccess) {
	a.mu.Lock()
	a.denied = append(a.denied, d)
	a.mu.Unlock()
}

type erroringPolicy struct{}

func (erroringPolicy) Allow(context.Context, domain.AccessInput) (bool, error) {
	return true, context.DeadlineExceeded
}

func newAuthorizer(policy usecase.AccessPolicy, roles *accessRoles, aud *recordingAuditor) *usecase.AuthorizeRequestAction {
	a := usecase.NewAuthorizeRequestAction(accessRequests{}, accessApprovals{}, roles, policy, aud)
	for _, e := range domain.Catalog {
		if e.Locator == domain.LocEntityID {
			a.RegisterEntity(e.Entity, fixedEntity{})
		}
	}
	return a
}
