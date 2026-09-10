# TASK-BE-004: Role Propagation Plumbing (prerequisite)

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files modified:**
> - `backend-go/common/tenant/tenant.go` (+ new `tenant_test.go`)
> - `backend-go/common/grpcmw/grpcmw.go` (+ new `grpcmw_test.go`)
> - `backend-go/services/api-gateway/internal/usecase/validate_identity.go`
> - `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`
> - `backend-go/services/api-gateway/internal/adapter/authclient/session_validator.go` (+ new `session_validator_test.go`, extended `jwks_client_test.go`'s shared fake)
> - `backend-go/services/api-gateway/internal/adapter/grpc/dial.go`

**Not part of any original solution doc** — discovered mid-implementation of
BE-SOL-002: admin-gating requires knowing the caller's role, and no working
mechanism for that existed anywhere in backend-go (`common/tenant` only
carried `tenant_id`/`user_id`; every service with an admin-override concept
had it hardcoded inert — see project-service/annotation-service's
`callerGlobalRole`).

## What changed (all additive — verified via `go build ./...` across all 17
backend-go services before and after)

1. `common/tenant.WithRole`/`Role(ctx)` — new context key, same shape as
   `WithTenantID`/`TenantID`.
2. `common/grpcmw.MetadataRole = "x-orca-role"` — new gRPC metadata key;
   `TenantExtractionInterceptor` reads it into context alongside
   tenant_id/user_id.
3. `usecase.Identity`/`wscompat.Identity` (api-gateway) gain a `Role` field.
4. `authclient.SessionValidator.ValidateToken` — previously discarded
   `user.GetRole()` entirely when building `wscompat.Identity`; now maps it
   through `roleString()` (auth-service's proto `Role` enum → lowercase
   `"admin"`/`"user"`, matching `domain.Role`'s own convention).
5. `gatewaygrpc.AttachIdentity` — now also stamps `MetadataRole` from
   `Identity.Role` onto outbound gRPC metadata.

## What did NOT change

- The ~100 existing `usecase.Identity{TenantID: ..., UserID: ...}` literal
  construction sites across `wscompat/channels_*.go` — Go struct literals
  don't require every field, so these compile unchanged and simply carry
  `Role: ""` (identical behavior to before this task).
- `usecase.AuthValidator.Validate` (the bearer-JWT path) — JWT claims don't
  carry a role claim yet; that path still resolves `Identity{Role: ""}`.
  Only the cookie/session path (the one every browser/web wscompat call
  goes through) populates Role.
- project-service/annotation-service's `callerGlobalRole` stubs — left
  as-is (still hardcoded `""`). They *could* now read `tenant.Role(ctx)`
  for real, but that's a separate, optional follow-up outside this CR's
  scope — not touched here to keep this pass's blast radius to
  infra-fleet-service's new admin-gated RPCs only.

## Verification

- `go build ./...` clean across all 17 backend-go services (both before
  writing any usecase code, to confirm the plumbing alone doesn't break
  anything, and after).
- New unit tests: `tenant_test.go` (4 tests), `grpcmw_test.go` (2 tests,
  including a backward-compatibility guard — missing `x-orca-role`
  metadata leaves the role absent, not fabricated), `session_validator_test.go`
  (4 tests, table-driven over admin/user/unspecified proto roles).
- Full `api-gateway`/`common` test suites pass unchanged otherwise.
