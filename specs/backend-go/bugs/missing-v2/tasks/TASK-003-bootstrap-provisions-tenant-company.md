# TASK-003: `auth-service` bootstrap provisions a `tenant-service` company, uses its returned `tenant_id`

**From Solution:** SOL-002
**Priority:** P0
**Service:** `auth-service`
**File:** `services/auth-service/internal/usecase/bootstrap.go`, `services/auth-service/internal/adapter/grpcclient/tenant_provisioner.go` (new), `services/auth-service/internal/config/config.go`, `services/auth-service/cmd/server/main.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`Bootstrap.EnsureAdmin` stamps an operator-supplied `BOOTSTRAP_TENANT_ID`
onto the created `User` row, but no `tenant-service` company exists for
that ID — every `profile.*` call then 500s (BUG-002).
`tenant-service.CreateCompany` (`internal/usecase/create_company.go`)
**generates** the tenant ID itself (`domain.NewCompany(uuid.NewString(), in.Name, nil)`)
— it has no field to accept a caller-supplied one. Bootstrap must be
inverted to call `CreateCompany` first and use the ID it returns, not
invent its own.

## Changes to make

### Step 1 — new adapter: `internal/adapter/grpcclient/tenant_provisioner.go`

Mirrors `project-service`'s `internal/adapter/grpcclient/infra_fleet_dev_server_lister.go`
pattern exactly (typed client wrapper dialing one peer service):

```go
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TenantProvisioner implements usecase.TenantProvisioner by dialing
// tenant-service's CreateCompany RPC — used only by the first-boot
// bootstrap (bootstrap.go), which needs to originate a tenant, not join
// an existing one. See specs/backend-go/bugs/missing-v2/BUG-002.
type TenantProvisioner struct {
	conn   *grpc.ClientConn
	client tenantv1.TenantServiceClient
}

func NewTenantProvisioner(addr string) (*TenantProvisioner, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial tenant-service at %q: %w", addr, err)
	}
	return &TenantProvisioner{conn: conn, client: tenantv1.NewTenantServiceClient(conn)}, nil
}

func (c *TenantProvisioner) Close() error {
	return c.conn.Close()
}

// CreateCompany returns the newly-originated tenant ID (== the created
// Company's id) — bootstrap.go uses this as the admin User's tenant_id.
func (c *TenantProvisioner) CreateCompany(ctx context.Context, name string) (string, error) {
	resp, err := c.client.CreateCompany(ctx, &tenantv1.CreateCompanyRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("grpcclient: tenant-service CreateCompany: %w", err)
	}
	return resp.GetCompany().GetId(), nil
}
```

### Step 2 — `internal/usecase/bootstrap.go`: saga

Add the `TenantProvisioner` port, remove `BootstrapConfig.TenantID`, and
reorder `EnsureAdmin` to provision the company before constructing the
`User`:

```go
// TenantProvisioner is the one tenant-service call bootstrap needs to
// originate a tenant for the admin it's about to create — see
// tenant-service/internal/usecase/create_company.go's own doc comment:
// "the operation that creates one," i.e. tenant_id must come FROM this
// call, never be supplied by the caller. Implemented by
// internal/adapter/grpcclient.TenantProvisioner.
type TenantProvisioner interface {
	CreateCompany(ctx context.Context, name string) (tenantID string, err error)
}

type Bootstrap struct {
	users   UserRepository
	audit   AuditRepository
	hasher  PasswordHasher
	clock   Clock
	tenants TenantProvisioner
}

func NewBootstrap(users UserRepository, audit AuditRepository, hasher PasswordHasher, clock Clock, tenants TenantProvisioner) *Bootstrap {
	return &Bootstrap{users: users, audit: audit, hasher: hasher, clock: clock, tenants: tenants}
}

// BootstrapConfig.TenantID is REMOVED — tenant-service originates it now.
// CompanyName is new and optional; empty defers to defaultCompanyName.
type BootstrapConfig struct {
	CompanyName string
	Email       string
	Password    string
}

func (b *Bootstrap) EnsureAdmin(ctx context.Context, cfg BootstrapConfig, logger *slog.Logger) (generatedPassword string, err error) {
	if cfg.Email == "" {
		logger.Info("auth-service: bootstrap skipped (BOOTSTRAP_ADMIN_EMAIL not set)")
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

	// Saga step 1: originate the tenant. No User row exists yet, so a
	// failure here needs no compensation — bootstrap simply didn't run,
	// safe to retry on next boot.
	tenantID, err := b.tenants.CreateCompany(ctx, cmp.Or(cfg.CompanyName, defaultCompanyName(cfg.Email)))
	if err != nil {
		return "", fmt.Errorf("bootstrap: provisioning tenant company: %w", err)
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
	user, err := domain.NewUser(uuid.NewString(), tenantID, cfg.Email, "Admin", domain.RoleAdmin, true, now)
	if err != nil {
		return "", fmt.Errorf("bootstrap: constructing admin user: %w", err)
	}
	created, err := b.users.CreateUser(ctx, user, passwordHash)
	if err != nil {
		// Saga step 2 failed — no DeleteCompany RPC exists on tenant-service
		// (deliberately not added for this rare, first-boot-only path — see
		// SOL-002's "Design" section). Log the orphaned tenant loudly; safe
		// to retry bootstrap on next boot.
		logger.Error("bootstrap: user creation failed after tenant company was provisioned — company left orphaned",
			slog.String("tenant_id", tenantID), slog.Any("err", err))
		return "", fmt.Errorf("bootstrap: creating admin user: %w", err)
	}

	if entry, err := domain.NewAuditEntry(uuid.NewString(), created.TenantID, created.ID, "user.bootstrap_created", created.ID, now); err == nil {
		_ = b.audit.Append(ctx, entry)
	}

	logger.Info("auth-service: bootstrap admin created", slog.String("email", cfg.Email), slog.String("tenant_id", tenantID))

	if cfg.Password == "" {
		return password, nil
	}
	return "", nil
}

// defaultCompanyName derives a placeholder company name from the admin
// email's domain (e.g. "admin@acme.com" -> "acme.com") when the operator
// doesn't set BOOTSTRAP_COMPANY_NAME — renameable later via the real
// profile.updateCompany RPC, matching this bootstrap's existing
// "auto-generate, operator fixes up later if they care" posture for the
// password.
func defaultCompanyName(email string) string {
	if _, domain, ok := strings.Cut(email, "@"); ok && domain != "" {
		return domain
	}
	return "Default Company"
}
```

Add `"cmp"` and `"strings"` to `bootstrap.go`'s imports.

### Step 3 — `internal/config/config.go`: replace `BootstrapTenantID` with `BootstrapCompanyName` + add `TenantServiceAddr`

```go
// BEFORE:
// BootstrapTenantID      string
// AFTER:
BootstrapCompanyName string // optional; empty => derive from BootstrapAdminEmail's domain
TenantServiceAddr    string
```

```go
// BEFORE:
// BootstrapTenantID:      os.Getenv("BOOTSTRAP_TENANT_ID"),
// AFTER:
BootstrapCompanyName: os.Getenv("BOOTSTRAP_COMPANY_NAME"),
TenantServiceAddr:    commonconfig.StringEnv("TENANT_SERVICE_ADDR", "tenant-service:9090"),
```

(Match whatever helper — `os.Getenv` vs. `commonconfig.StringEnv` — this
file already uses for its other `*ServiceAddr` fields, if any exist;
`TenantServiceAddr` needs a real default matching how other services
reference `tenant-service` in `docker-compose.yml`/Kubernetes Service DNS,
e.g. `project-service/internal/config/config.go`'s
`INFRA_FLEET_SERVICE_ADDR` default as the precedent to copy the shape
from.)

### Step 4 — `cmd/server/main.go`: wire the new dependency

```go
// Before bootstrap construction, dial tenant-service:
tenantProvisioner, err := grpcclient.NewTenantProvisioner(cfg.TenantServiceAddr)
if err != nil {
	return fmt.Errorf("dialing tenant-service: %w", err)
}
defer func() { _ = tenantProvisioner.Close() }()

bootstrap := usecase.NewBootstrap(repo, repo, hasher, clock, tenantProvisioner)
generatedPassword, err := bootstrap.EnsureAdmin(ctx, usecase.BootstrapConfig{
	CompanyName: cfg.BootstrapCompanyName,
	Email:       cfg.BootstrapAdminEmail,
	Password:    cfg.BootstrapAdminPassword,
}, logger)
```

Only dial `tenant-service` if bootstrap will actually run
(`cfg.BootstrapAdminEmail != ""`) — dialing unconditionally would add a
required, always-on dependency on `tenant-service` being reachable at
every `auth-service` boot, not just first boot. Guard the dial+bootstrap
block together:

```go
if cfg.BootstrapAdminEmail != "" {
	tenantProvisioner, err := grpcclient.NewTenantProvisioner(cfg.TenantServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing tenant-service: %w", err)
	}
	defer func() { _ = tenantProvisioner.Close() }()

	bootstrap := usecase.NewBootstrap(repo, repo, hasher, clock, tenantProvisioner)
	generatedPassword, err := bootstrap.EnsureAdmin(ctx, usecase.BootstrapConfig{
		CompanyName: cfg.BootstrapCompanyName,
		Email:       cfg.BootstrapAdminEmail,
		Password:    cfg.BootstrapAdminPassword,
	}, logger)
	if err != nil {
		return fmt.Errorf("bootstrapping admin user: %w", err)
	}
	if generatedPassword != "" {
		logger.Warn("auth-service: AUTO-GENERATED ADMIN PASSWORD (save this now, it will not be shown again)",
			slog.String("email", cfg.BootstrapAdminEmail),
			slog.String("password", generatedPassword))
	}
}
```

(`EnsureAdmin` still independently no-ops on `HasAnyUsers` — this outer
guard is purely to avoid the `tenant-service` dial on every ordinary boot
of an already-bootstrapped deployment, not a duplicate of that check.)

## Verify

```bash
cd backend-go
go build ./services/auth-service/...
go vet ./services/auth-service/...
go test ./services/auth-service/... -count=1
```

Expected: clean build (watch for the removed `BootstrapConfig.TenantID`
field breaking any other reference — `grep -rn "BootstrapConfig{" services/auth-service`
to confirm `main.go` was the only construction site), all existing tests
pass. TASK-004 adds the new tests this change specifically needs.
