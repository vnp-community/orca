package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type accessRepo struct {
	projects    map[string]domain.Project
	roles       map[string]domain.ProjectMember
	getErr      error
	membershipE error
}

func (r accessRepo) Get(_ context.Context, tenantID, id string) (domain.Project, error) {
	if r.getErr != nil {
		return domain.Project{}, r.getErr
	}
	p, ok := r.projects[id]
	if !ok || p.TenantID != tenantID {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	return p, nil
}

func (r accessRepo) GetMembership(_ context.Context, projectID, userID string) (domain.ProjectMember, error) {
	if r.membershipE != nil {
		return domain.ProjectMember{}, r.membershipE
	}
	m, ok := r.roles[projectID+"/"+userID]
	if !ok {
		return domain.ProjectMember{}, domain.ErrMembershipNotFound
	}
	return m, nil
}

func TestResolveProjectAccess(t *testing.T) {
	repo := accessRepo{
		projects: map[string]domain.Project{"p1": {ID: "p1", TenantID: "t1"}, "other": {ID: "other", TenantID: "t2"}},
		roles:    map[string]domain.ProjectMember{"p1/owner-user": {Role: domain.ProjectRoleOwner}, "p1/member-user": {Role: domain.ProjectRoleMember}},
	}
	uc := NewResolveProjectAccess(repo)
	ctx := tenant.WithTenantID(context.Background(), "t1")

	cases := []struct {
		name, project, user string
		want                ProjectAccess
	}{
		{"owner", "p1", "owner-user", ProjectAccess{Exists: true, Role: "owner"}},
		{"member", "p1", "member-user", ProjectAccess{Exists: true, Role: "member"}},
		{"exists but the user is not a member", "p1", "stranger", ProjectAccess{Exists: true}},
		{"no user asked for", "p1", "", ProjectAccess{Exists: true}},
		{"project does not exist", "missing", "owner-user", ProjectAccess{}},
		{"another tenant's project is not visible", "other", "owner-user", ProjectAccess{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := uc.Execute(ctx, tc.project, tc.user)
			if err != nil || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestResolveProjectAccess_Errors(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), "t1")
	kind := func(err error) apperrors.Kind {
		var ae *apperrors.AppError
		if errors.As(err, &ae) {
			return ae.Kind
		}
		return apperrors.KindUnknown
	}
	if _, err := NewResolveProjectAccess(accessRepo{}).Execute(context.Background(), "p1", "u"); kind(err) != apperrors.KindUnauthenticated {
		t.Errorf("no tenant: %v", err)
	}
	if _, err := NewResolveProjectAccess(accessRepo{}).Execute(ctx, "", "u"); kind(err) != apperrors.KindInvalidArgument {
		t.Errorf("empty project id: %v", err)
	}
	if _, err := NewResolveProjectAccess(accessRepo{getErr: errors.New("db down")}).Execute(ctx, "p1", "u"); kind(err) != apperrors.KindInternal {
		t.Errorf("repository failure must stay an error, not look like a missing project: %v", err)
	}
	repo := accessRepo{projects: map[string]domain.Project{"p1": {ID: "p1", TenantID: "t1"}}, membershipE: errors.New("db down")}
	if _, err := NewResolveProjectAccess(repo).Execute(ctx, "p1", "u"); kind(err) != apperrors.KindInternal {
		t.Errorf("membership failure: %v", err)
	}
}
