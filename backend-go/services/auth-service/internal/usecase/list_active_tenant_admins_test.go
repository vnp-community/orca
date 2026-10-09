package usecase

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func seedUser(t *testing.T, users *fakeUserRepository, id, tenantID string, role domain.Role, active bool) {
	t.Helper()
	u, err := domain.NewUser(id, tenantID, id+"@example.com", id, role, active, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	users.seed(u, "hash")
}

func TestListActiveTenantAdmins_NeedsNoActorAndReturnsOnlyActiveAdminsOfTheContextTenant(t *testing.T) {
	users := newFakeUserRepository()
	seedUser(t, users, "a1", "t1", domain.RoleAdmin, true)
	seedUser(t, users, "a2", "t1", domain.RoleAdmin, true)
	seedUser(t, users, "a-off", "t1", domain.RoleAdmin, false)
	seedUser(t, users, "u1", "t1", domain.RoleUser, true)
	seedUser(t, users, "a-other", "t2", domain.RoleAdmin, true)

	// Tenant only, no user: this is the outbox relay's context.
	out, err := NewListActiveTenantAdmins(users).Execute(tenant.WithTenantID(context.Background(), "t1"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out.UserIDs)
	if !reflect.DeepEqual(out.UserIDs, []string{"a1", "a2"}) || out.Truncated {
		t.Fatalf("got %+v, want only t1's active admins", out)
	}
}

func TestListActiveTenantAdmins_RequiresTenant(t *testing.T) {
	_, err := NewListActiveTenantAdmins(newFakeUserRepository()).Execute(context.Background())
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindUnauthenticated {
		t.Fatalf("err = %v, want unauthenticated without a tenant", err)
	}
}

func TestListActiveTenantAdmins_NoAdminsIsAnEmptyListNotAnError(t *testing.T) {
	out, err := NewListActiveTenantAdmins(newFakeUserRepository()).Execute(tenant.WithTenantID(context.Background(), "t1"))
	if err != nil || out.UserIDs == nil || len(out.UserIDs) != 0 {
		t.Fatalf("got %+v %v", out, err)
	}
}

// pagedUsers serves fixed pages keyed by the incoming token and records the tenant it was asked about.
type pagedUsers struct {
	UserRepository
	pages   map[string][]domain.User
	next    map[string]string
	tenants []string
	err     error
}

func (p *pagedUsers) ListUsers(_ context.Context, tenantID, token string, _ int32) ([]domain.User, string, error) {
	p.tenants = append(p.tenants, tenantID)
	return p.pages[token], p.next[token], p.err
}

func adminUser(id string) domain.User {
	return domain.User{ID: id, TenantID: "t1", Role: domain.RoleAdmin, IsActive: true}
}

func TestListActiveTenantAdmins_FollowsPagesAndReportsTruncation(t *testing.T) {
	repo := &pagedUsers{
		pages: map[string][]domain.User{"": {adminUser("a1")}, "p2": {adminUser("a2")}},
		next:  map[string]string{"": "p2"},
	}
	ctx := tenant.WithTenantID(context.Background(), "t1")
	out, err := NewListActiveTenantAdmins(repo).Execute(ctx)
	if err != nil || !reflect.DeepEqual(out.UserIDs, []string{"a1", "a2"}) || out.Truncated {
		t.Fatalf("got %+v %v", out, err)
	}
	for _, tn := range repo.tenants {
		if tn != "t1" {
			t.Fatalf("repository asked about tenant %q, want the context tenant only", tn)
		}
	}

	// A repository that never ends its paging must stop at the bound and say the answer is partial.
	endless := &pagedUsers{pages: map[string][]domain.User{}, next: map[string]string{}}
	for i := 0; i <= activeAdminScanMaxPages; i++ {
		endless.pages[strconv.Itoa(i)] = []domain.User{adminUser("a" + strconv.Itoa(i))}
		endless.next[strconv.Itoa(i)] = strconv.Itoa(i + 1)
	}
	endless.pages[""] = []domain.User{adminUser("first")}
	endless.next[""] = "0"
	out, err = NewListActiveTenantAdmins(endless).Execute(ctx)
	if err != nil || !out.Truncated || len(endless.tenants) != activeAdminScanMaxPages {
		t.Fatalf("got truncated=%v pages=%d err=%v", out.Truncated, len(endless.tenants), err)
	}
}

func TestListActiveTenantAdmins_DropsRowsOfAnotherTenantEvenIfTheRepositoryReturnsThem(t *testing.T) {
	foreign := adminUser("x")
	foreign.TenantID = "t2"
	repo := &pagedUsers{pages: map[string][]domain.User{"": {adminUser("a1"), foreign}}}
	out, err := NewListActiveTenantAdmins(repo).Execute(tenant.WithTenantID(context.Background(), "t1"))
	if err != nil || !reflect.DeepEqual(out.UserIDs, []string{"a1"}) {
		t.Fatalf("got %+v %v", out, err)
	}
}

func TestListActiveTenantAdmins_RepositoryFailureIsInternal(t *testing.T) {
	_, err := NewListActiveTenantAdmins(&pagedUsers{err: errors.New("db down")}).Execute(tenant.WithTenantID(context.Background(), "t1"))
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindInternal {
		t.Fatalf("err = %v", err)
	}
}
