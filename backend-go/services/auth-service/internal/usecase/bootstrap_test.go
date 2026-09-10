package usecase

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// fakeTenantProvisioner is a test double for TenantProvisioner — see
// specs/backend-go/bugs/missing-v2/BUG-002/SOL-002.
type fakeTenantProvisioner struct {
	nextTenantID string // returned by CreateCompany; defaults to "generated-tenant-1" if unset
	createErr    error
	calledWith   []string // company names CreateCompany was called with, in order
}

func (f *fakeTenantProvisioner) CreateCompany(_ context.Context, name string) (string, error) {
	f.calledWith = append(f.calledWith, name)
	if f.createErr != nil {
		return "", f.createErr
	}
	if f.nextTenantID == "" {
		return "generated-tenant-1", nil
	}
	return f.nextTenantID, nil
}

func TestBootstrap_CreatesAdmin_OnFreshDeployment(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if generated == "" {
		t.Error("expected a generated password to be returned when none was supplied")
	}

	user, hash, err := users.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("expected admin user to exist: %v", err)
	}
	if user.Role != domain.RoleAdmin {
		t.Errorf("expected role Admin, got %v", user.Role)
	}
	if user.TenantID != "generated-tenant-1" {
		t.Errorf("expected user.TenantID to come from CreateCompany's return value, got %q", user.TenantID)
	}
	if err := hasher.Compare(hash, generated); err != nil {
		t.Errorf("stored hash doesn't match the returned generated password: %v", err)
	}
}

func TestBootstrap_NoOp_WhenUsersAlreadyExist(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(domain.User{ID: "existing", TenantID: "t1", Email: "someone@example.com", Role: domain.RoleUser}, "hash")
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if generated != "" {
		t.Error("expected no-op (empty generated password) when users already exist")
	}
	if _, _, err := users.GetUserByEmail(context.Background(), "admin@example.com"); err == nil {
		t.Error("expected no admin user to be created")
	}
	if len(tenants.calledWith) != 0 {
		t.Errorf("expected CreateCompany never called when bootstrap no-ops, got %d calls", len(tenants.calledWith))
	}
}

func TestBootstrap_NoOp_WhenConfigIncomplete(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if generated != "" {
		t.Error("expected no-op when Email is unset")
	}
	if len(tenants.calledWith) != 0 {
		t.Errorf("expected CreateCompany never called when Email is unset, got %d calls", len(tenants.calledWith))
	}
}

func TestBootstrap_UsesSuppliedPassword_WithoutReturningIt(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "operator-chosen-password",
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if generated != "" {
		t.Error("expected empty return when the operator supplied their own password (never echo it back)")
	}

	_, hash, err := users.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("expected admin user to exist: %v", err)
	}
	if err := hasher.Compare(hash, "operator-chosen-password"); err != nil {
		t.Error("expected the admin's stored hash to match the operator-supplied password")
	}
}

// TestBootstrap_ProvisionsTenantBeforeCreatingUser is the direct
// regression test for the saga order SOL-002 designs: tenant-service is
// asked to originate a tenant BEFORE the admin User row is constructed,
// and the returned tenant id is what the User ends up with.
func TestBootstrap_ProvisionsTenantBeforeCreatingUser(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{nextTenantID: "tenant-xyz"}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	_, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants.calledWith) != 1 {
		t.Fatalf("expected CreateCompany called exactly once, got %d calls", len(tenants.calledWith))
	}
	user, _, err := users.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("expected admin user to exist: %v", err)
	}
	if user.TenantID != "tenant-xyz" {
		t.Errorf("expected user.TenantID %q (from CreateCompany's return value), got %q", "tenant-xyz", user.TenantID)
	}
}

// TestBootstrap_CreateCompanyFailure_NeverCreatesUser is the direct
// regression test for the saga's fail-fast-at-step-1 behavior.
func TestBootstrap_CreateCompanyFailure_NeverCreatesUser(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{createErr: errors.New("tenant-service unreachable")}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	_, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err == nil {
		t.Fatal("expected an error when CreateCompany fails")
	}
	if _, _, err := users.GetUserByEmail(context.Background(), "admin@example.com"); err == nil {
		t.Error("expected no admin user to be created when tenant provisioning failed")
	}
}

// TestBootstrap_CreateUserFailure_AfterTenantProvisioned_ReturnsOriginalError
// confirms the failure surfaces correctly when step 2 (CreateUser) fails
// after step 1 (CreateCompany) already succeeded — SOL-002 deliberately
// has no compensating DeleteCompany call (see bootstrap.go's doc comment
// on that branch), so this test only proves the error propagates, not
// that any cleanup RPC was called.
func TestBootstrap_CreateUserFailure_AfterTenantProvisioned_ReturnsOriginalError(t *testing.T) {
	users := newFakeUserRepository()
	users.createErr = errors.New("db write failed")
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{nextTenantID: "tenant-orphaned"}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	_, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err == nil {
		t.Fatal("expected the CreateUser error to propagate")
	}
	if len(tenants.calledWith) != 1 {
		t.Errorf("expected CreateCompany to have been called once before the CreateUser failure, got %d calls", len(tenants.calledWith))
	}
}

func TestDefaultCompanyName(t *testing.T) {
	cases := []struct{ email, want string }{
		{"admin@acme.com", "acme.com"},
		{"admin@sub.acme.com", "sub.acme.com"},
		{"not-an-email", "Default Company"},
		{"", "Default Company"},
	}
	for _, c := range cases {
		if got := defaultCompanyName(c.email); got != c.want {
			t.Errorf("defaultCompanyName(%q) = %q, want %q", c.email, got, c.want)
		}
	}
}
