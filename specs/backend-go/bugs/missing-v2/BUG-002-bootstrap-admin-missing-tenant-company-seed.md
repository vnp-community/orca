# BUG-002: Bootstrap admin has no `tenant-service` company/department row — every `profile.*` call fails

**Service:** `auth-service` (root cause), symptoms surface in `tenant-service`
**File:** `services/auth-service/internal/usecase/bootstrap.go`
**Severity:** High — blocks `profile.getResolved`, `profile.getUserProfile`, `profile.listDepts` for the ONLY user that exists on a fresh deployment (the bootstrap admin), i.e. first-run is broken for this entire namespace
**Symptom:** Every `profile.*` RPC fails for the bootstrap admin:
  - `profile.getResolved` → `NotFound: TENANT_COMPANY_NOT_FOUND: company does not exist`
  - `profile.getUserProfile` → `Internal: TENANT_PROFILE_LOOKUP_FAILED: failed to look up user profile`
  - `profile.listDepts` → `Internal: TENANT_LIST_DEPARTMENTS_FAILED: failed to list departments`
**Status:** 🔴 Open — found live 2026-08-27 via `tests/client/rpc-catalog.spec.ts` against `172.20.2.39:6769`.

---

## Description

`auth-service`'s first-boot bootstrap (`Bootstrap.EnsureAdmin`, added per
`docs/execution-plan.md` §0's "operator-chosen admin password" update)
creates exactly one row: an admin `User` in `auth-service`'s own database,
stamped with `TenantID: cfg.TenantID` (from `BOOTSTRAP_TENANT_ID`):

```go
// bootstrap.go:81-88
user, err := domain.NewUser(uuid.NewString(), cfg.TenantID, cfg.Email, "Admin", domain.RoleAdmin, true, now)
// ...
created, err := b.users.CreateUser(ctx, user, passwordHash)
```

It does **not** create any corresponding row in `tenant-service`'s own
database (no `Company`, no `Department`) for that same `tenant_id` — and
`tenant-service` is a separate service with its own Postgres schema, so the
`tenant_id` string `auth-service` stamped on the user has no referent
there. Every `tenant-service` usecase that resolves a company/profile for
that tenant then fails:

```go
// get_resolved_profile.go:45
return domain.ResolvedProfile{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
```

```go
// get_user_profile.go:34, update_user_profile.go:45, set_user_department.go:55
return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
```

```go
// list_departments.go:26
return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_DEPARTMENTS_FAILED", "failed to list departments", err)
```

Practical impact: on a genuinely fresh deployment (the exact scenario
`Bootstrap.EnsureAdmin` exists to support — "zero users exist anywhere"),
the ONLY user who can possibly be logged in is this admin, and every one of
their `profile.*` calls 500s. There is no working path to view or edit the
admin's own profile, company, or department list after first boot.

## Confirmed

- `services/auth-service/internal/usecase/bootstrap.go:52-100` — full
  `EnsureAdmin` body; only `b.users.CreateUser` is called, no call to
  `tenant-service` (no client dependency for it exists on `Bootstrap` at
  all — `users`/`audit`/`hasher`/`clock` are its only fields).
- `services/tenant-service/internal/usecase/get_resolved_profile.go:45`,
  `get_user_profile.go:34`, `list_departments.go:26`,
  `set_user_department.go:55` — all four confirmed error sites.
- Live-verified 2026-08-27 against `172.20.2.39:6769`: logged in as
  `admin@b15.openledger.vn` (the deployed bootstrap admin), all three RPCs
  above reproduced the exact errors quoted.

## Scope Note

This is scoped to the **bootstrap-created admin specifically** — a user
created later via `auth-service.CreateUser` by an admin (the "ongoing
admin-console path" `missing-v1`'s §0 update describes) may or may not hit
the same gap; not verified here, since the deployed environment has no
non-bootstrap user with departments/company set up to test against. If
`CreateUser` also never provisions a `tenant-service` company/department
row, this is a systemic gap in user provisioning, not just the one-time
bootstrap path — worth a follow-up check before fixing.

## Suggested Fix

Either:
1. **Provision synchronously at bootstrap** — `Bootstrap.EnsureAdmin` gains
   a `tenant-service` client dependency and creates a default `Company` (+
   optionally a default `Department`) for `cfg.TenantID` in the same
   bootstrap pass, before/after creating the `User` row.
2. **Provision lazily on first profile access** — `get_resolved_profile.go`
   / `get_user_profile.go` create a default company/department on
   `TENANT_COMPANY_NOT_FOUND` instead of erroring, if "every tenant
   implicitly has a company" is meant to be an invariant.

Option 1 matches the existing bootstrap's own framing ("mirrors the old TS
backend's own first-boot admin bootstrap... same operator-facing behavior")
more directly — the old TS backend's equivalent flow guaranteed a working
profile out of the box.
