# SOL-002: Fix BUG-002 — `auth-service`'s bootstrap provisions a `tenant-service` company/department as a synchronous saga step

**Resolves:** BUG-002
**Service:** `auth-service` (orchestrates), `tenant-service` (provides the RPC)
**Affected files:** `services/auth-service/internal/usecase/bootstrap.go`, `services/auth-service/cmd/server/main.go` (new `tenant-service` client dependency), `services/tenant-service/internal/adapter/grpc/server.go` (may need a new/existing `CreateCompany` RPC — verify against current proto before assuming one must be added)
**Priority:** High
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

`architecture/05-data-architecture.md`'s "Cross-service data consistency"
section gives exactly two sanctioned patterns for "service A does
something, service B needs to know" — and explicitly picks between them by
one question: *does the caller need to know the outcome before
responding?*

> **Synchronous saga** (only where a caller needs to know the outcome
> before responding): A's usecase layer calls B's gRPC API synchronously as
> one step in an explicit saga... If step 2 fails, step 1's compensating
> action runs. Used sparingly — e.g. `project-service.CreateProject` calling
> `infra-fleet-service` to validate a `devServerId` exists before
> committing the binding.

Bootstrap fits this exactly, arguably more strongly than the doc's own
example: an admin user that exists in `auth-service` but has no
`tenant-service` company is **not actually usable** — every `profile.*`
call 500s for them (BUG-002's whole symptom). The "caller" here is the
deploy operator watching first-boot logs for a working admin, not an
in-flight HTTP request, but the same principle applies: bootstrap should
not report success (log the generated password, let operators believe
first boot worked) while leaving the admin in a broken half-provisioned
state. The **outbox + async event** pattern (the doc's default) is the
wrong fit here specifically because it's eventually-consistent — an
operator logging in with the freshly-generated password moments after boot
would race the async consumer and likely still hit BUG-002's exact
failure.

## Design

**Correction found while designing this fix, not assumed going in:**
`tenant-service`'s existing `CreateCompany` usecase does **not** accept a
caller-supplied tenant ID — it *generates* one:

```go
// tenant-service/internal/usecase/create_company.go:19-21, 30-31
// CreateCompany originates a new tenant. Deliberately does NOT require a
// tenant already in context — this is the operation that creates one
// (tenant-service.md §1: "the service that originates tenant_id").
func (uc *CreateCompany) Execute(ctx context.Context, in CreateCompanyInput) (domain.Company, error) {
	company, err := domain.NewCompany(uuid.NewString(), in.Name, nil)
	// ...
```

This means `auth-service`'s current bootstrap design — taking an
operator-supplied `BOOTSTRAP_TENANT_ID` and stamping it directly onto the
`User` row — is itself misaligned with `tenant-service.md §1`'s ownership
model (`tenant-service` is documented as the sole originator of
`tenant_id`, not something another service or an operator should be
allowed to pick). The correct fix isn't "pass `BOOTSTRAP_TENANT_ID`
through to `CreateCompany`" (that RPC has no field for it); it's **invert
the saga's direction**: ask `tenant-service` to mint the tenant first, then
use the ID it returns for the `User` row — `BOOTSTRAP_TENANT_ID` as an
operator-supplied value stops being meaningful and should be dropped from
`BootstrapConfig` (an operator-supplied *company name*, or none at all
defaulting to something derived from the email, replaces it).

```go
// bootstrap.go — sketch
type Bootstrap struct {
	users   UserRepository
	audit   AuditRepository
	hasher  PasswordHasher
	clock   Clock
	tenants TenantProvisioner // NEW — thin interface, see below
}

// TenantProvisioner is the one tenant-service call bootstrap needs —
// scoped narrowly (mirrors this service's existing single-purpose
// adapter-interface convention, e.g. opaclient's Client) rather than
// depending on tenant-service's full gRPC client surface.
type TenantProvisioner interface {
	CreateCompany(ctx context.Context, name string) (tenantID string, err error)
}

// BootstrapConfig.TenantID is REMOVED — tenant-service now originates it.
type BootstrapConfig struct {
	CompanyName string // optional; empty => derive from Email's domain
	Email       string
	Password    string
}

func (b *Bootstrap) EnsureAdmin(ctx context.Context, cfg BootstrapConfig, logger *slog.Logger) (generatedPassword string, err error) {
	if cfg.Email == "" {
		logger.Info("auth-service: bootstrap skipped (BOOTSTRAP_ADMIN_EMAIL not set)")
		return "", nil
	}
	// ... unchanged: HasAnyUsers check ...

	// Saga step 1: originate the tenant via tenant-service — no User row
	// exists yet, so a failure here needs no compensation at all; bootstrap
	// simply didn't run, safe to retry on next boot.
	tenantID, err := b.tenants.CreateCompany(ctx, cmp.Or(cfg.CompanyName, defaultCompanyName(cfg.Email)))
	if err != nil {
		return "", fmt.Errorf("bootstrap: provisioning tenant company: %w", err)
	}

	// ... existing: hash password ...
	user, err := domain.NewUser(uuid.NewString(), tenantID, cfg.Email, "Admin", domain.RoleAdmin, true, now)
	// ...
	created, err := b.users.CreateUser(ctx, user, passwordHash)
	if err != nil {
		// Saga step 2 failed, leaving an orphaned (userless) Company row
		// from step 1. No DeleteCompany RPC exists on tenant-service today
		// — rather than adding a destructive RPC whose only caller would
		// ever be this rare, first-boot-only failure path, log loudly and
		// leave the orphan: an inert company with zero users is not a
		// correctness or security issue, just a small amount of unused
		// state, and bootstrap is safe to retry on the next boot (it will
		// mint a second orphaned company from the aborted first attempt,
		// same reasoning). Revisit only if this proves to be a real
		// operational annoyance in practice.
		logger.Error("bootstrap: user creation failed after tenant company was provisioned — company left orphaned",
			slog.String("tenant_id", tenantID), slog.Any("err", err))
		return "", fmt.Errorf("bootstrap: creating admin user: %w", err)
	}

	// ... existing: audit entry, logging, return password ...
}
```

`defaultCompanyName(email)` — a simple derivation (e.g. the email's domain
part) since most deployments won't set `BOOTSTRAP_COMPANY_NAME` explicitly;
renameable later via the already-real `profile.updateCompany` RPC, matching
this bootstrap's existing "auto-generate, operator fixes up later if they
care" posture for the password.

### Why not extend it to Department too

BUG-002 confirms `TENANT_COMPANY_NOT_FOUND` blocks `profile.getResolved`
outright, but `TENANT_LIST_DEPARTMENTS_FAILED`
(`list_departments.go:26`) is a **list** query — an empty department list
for a freshly-provisioned company is a valid, correctly-empty result (once
the underlying `Internal` error is fixed by having a real company row to
query against), not a missing-department error. Provisioning a default
Department the admin didn't ask for adds unrequested state; recommend
verifying `list_departments.go`'s exact failure mode against a company
that exists but has zero departments before assuming this needs a second
saga step — plausibly Company alone resolves BUG-002 in full.

### Scope note carried over from the bug report

This only fixes the **bootstrap** path. Per BUG-002's own "Scope Note", if
`auth-service.CreateUser` (the ongoing admin-console path) also never
provisions a tenant-service company for a *new* tenant, that's the same
gap recurring outside bootstrap — worth checking as a fast follow, not
assumed fixed by this change.

## Testing Plan

- Usecase-level test: `EnsureAdmin` with a fake `TenantProvisioner` —
  asserts `CreateCompany` is called before `users.CreateUser`, and that the
  `tenantID` it returns is the exact value threaded into `domain.NewUser`
  (not a separately-sourced value — this is the specific thing the original
  `BOOTSTRAP_TENANT_ID` design got backwards).
- Usecase-level test: `CreateCompany` failing → `EnsureAdmin` returns an
  error and `users.CreateUser` is never called (saga stops at step 1, no
  partial state, nothing to compensate).
- Usecase-level test: `CreateUser` failing after a successful
  `CreateCompany` → `EnsureAdmin` returns the `CreateUser` error and logs
  the orphaned `tenantID` (asserted via a test log-capture, not a
  compensating RPC call — there is none by design, see above).
- Integration test (real Postgres, both services' schemas): full
  `EnsureAdmin` run, then assert `profile.getResolved` (or the equivalent
  `tenant-service` usecase call directly) succeeds for the created admin —
  this is the actual regression BUG-002 describes, so the test should
  exercise it end-to-end, not just at the mock boundary.
- Re-run `tests/client/rpc-catalog.spec.ts`'s `profile.*` tests against a
  **freshly bootstrapped** deployment (not the already-broken
  `172.20.2.39` — this fix only helps a NEW bootstrap; the existing broken
  admin on that deployment needs a manual backfill, not a code fix, to
  recover without a full redeploy).
