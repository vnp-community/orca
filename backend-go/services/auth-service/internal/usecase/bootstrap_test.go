package usecase

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestBootstrap_CreatesAdmin_OnFreshDeployment(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	bootstrap := NewBootstrap(users, audit, hasher, clock)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		TenantID: "tenant-1", Email: "admin@example.com", Password: "",
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
	bootstrap := NewBootstrap(users, audit, hasher, clock)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		TenantID: "tenant-1", Email: "admin@example.com", Password: "",
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
}

func TestBootstrap_NoOp_WhenConfigIncomplete(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	bootstrap := NewBootstrap(users, audit, hasher, clock)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if generated != "" {
		t.Error("expected no-op when TenantID/Email are unset")
	}
}

func TestBootstrap_UsesSuppliedPassword_WithoutReturningIt(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	bootstrap := NewBootstrap(users, audit, hasher, clock)

	generated, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		TenantID: "tenant-1", Email: "admin@example.com", Password: "operator-chosen-password",
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
