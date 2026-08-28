package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Bootstrap creates the first admin user in a completely fresh deployment
// — the gap found live on 2026-08-17 (docs/execution-plan.md §0): a fresh
// Postgres has zero users, CreateUser requires an existing admin caller
// (requireAdminActor), and there was no way to break that cycle. Mirrors
// the old TS backend's own first-boot admin bootstrap
// (ORCA_ADMIN_EMAIL/ORCA_ADMIN_PASSWORD, auto-generated + logged if unset)
// — same operator-facing behavior, reimplemented for backend-go's
// tenant-scoped user model.
//
// Runs ONCE, at service startup (cmd/server/main.go), never via an RPC —
// this is intentionally not reachable from any client, to avoid it being a
// standing "create an admin with no auth" attack surface after first boot.
type Bootstrap struct {
	users  UserRepository
	audit  AuditRepository
	hasher PasswordHasher
	clock  Clock
}

func NewBootstrap(users UserRepository, audit AuditRepository, hasher PasswordHasher, clock Clock) *Bootstrap {
	return &Bootstrap{users: users, audit: audit, hasher: hasher, clock: clock}
}

// BootstrapConfig carries the operator-supplied (or auto-generated) admin
// credentials — see cmd/server/main.go for where these come from
// (BOOTSTRAP_TENANT_ID/BOOTSTRAP_ADMIN_EMAIL/BOOTSTRAP_ADMIN_PASSWORD).
type BootstrapConfig struct {
	TenantID string
	Email    string
	Password string // empty => auto-generate and log it
}

// EnsureAdmin is a no-op if any user already exists anywhere (not just this
// tenant) — bootstrap is strictly a "first boot of a totally fresh
// deployment" action, never something that re-runs against a live system.
// Returns the plaintext password ONLY when one was auto-generated (so
// main.go can log it once, the same "printed to stdout on first boot, never
// stored" contract the old backend used) — empty string when the operator
// supplied their own via BOOTSTRAP_ADMIN_PASSWORD.
func (b *Bootstrap) EnsureAdmin(ctx context.Context, cfg BootstrapConfig, logger *slog.Logger) (generatedPassword string, err error) {
	if cfg.TenantID == "" || cfg.Email == "" {
		logger.Info("auth-service: bootstrap skipped (BOOTSTRAP_TENANT_ID/BOOTSTRAP_ADMIN_EMAIL not set)")
		return "", nil
	}

	exists, err := b.users.HasAnyUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("bootstrap: checking for existing users: %w", err)
	}
	if exists {
		logger.Info("auth-service: bootstrap skipped (users already exist)")
		return "", nil
	}

	password := cfg.Password
	if password == "" {
		password, err = generateRandomToken(16)
		if err != nil {
			return "", fmt.Errorf("bootstrap: generating admin password: %w", err)
		}
	}

	passwordHash, err := b.hasher.Hash(password)
	if err != nil {
		return "", fmt.Errorf("bootstrap: hashing admin password: %w", err)
	}

	now := b.clock.Now()
	user, err := domain.NewUser(uuid.NewString(), cfg.TenantID, cfg.Email, "Admin", domain.RoleAdmin, true, now)
	if err != nil {
		return "", fmt.Errorf("bootstrap: constructing admin user: %w", err)
	}
	created, err := b.users.CreateUser(ctx, user, passwordHash)
	if err != nil {
		return "", fmt.Errorf("bootstrap: creating admin user: %w", err)
	}

	if entry, err := domain.NewAuditEntry(uuid.NewString(), created.TenantID, created.ID, "user.bootstrap_created", "user", created.ID, map[string]any{}, "", now); err == nil {
		_ = b.audit.Append(ctx, entry)
	}

	logger.Info("auth-service: bootstrap admin created", slog.String("email", cfg.Email), slog.String("tenant_id", cfg.TenantID))

	if cfg.Password == "" {
		return password, nil // caller logs this once; never persisted in plaintext anywhere
	}
	return "", nil
}
