# SOL-PRF-01: RBAC gating, write-time field validation, and audit logging for `profile.*` CRUD

**Resolves:** [BUG-PRF-01](../BUG-PRF-01-profile-crud-validation-rbac-missing.md)
**Service:** `tenant-service` (+ a small, explicitly-flagged `notification-service`/`auth-service` consumer follow-up for audit-event consumption)
**Affected files (proposed):**
- `backend-go/policy/orca-authz/tenant.rego`, `tenant_test.rego` (new)
- `backend-go/services/tenant-service/internal/adapter/opaclient/client.go` (new)
- `backend-go/services/tenant-service/internal/usecase/authorization.go` (new)
- `backend-go/services/tenant-service/internal/usecase/ports.go` (extend: `OPAClient`, `AuditPublisher`, `DepartmentRepository.ExistsByName`)
- `backend-go/services/tenant-service/internal/usecase/update_company.go`, `update_department.go`, `update_user_profile.go`, `create_department.go` (edit: role gate, patch validation, audit emission)
- `backend-go/services/tenant-service/internal/domain/company.go`, `department.go`, `user_profile.go` (edit: add `Validate*Settings` functions + `SUPPORTED_MODELS`)
- `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go` (extend: `PublishAuditEvent`)
- `backend-go/services/tenant-service/cmd/server/main.go` (wire OPA evaluator + opaclient)
- `backend-go/common/tenant/tenant.go` (extend: `Role(ctx)` accessor — see "Known gap" below)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`tenant-service.md` §4/§9 already establishes the two load-bearing facts this
solution builds on:

- **§9**: "`security` profile-section values ... the merge algorithm's
  refusal to let department/team/user override that section is a security
  control, **enforced in the domain layer**, not an adapter-level check that
  could be bypassed." BUG-PRF-01's finding is that this enforcement exists
  only at *resolve* time (`profile_resolution.go:171-186`'s `lockSecurity`,
  a silent discard) and not at *write* time (a 400, per
  `docs/logic/profile/BL-PRF-01-profile-crud.md:123`'s Error Cases table).
  This solution adds the write-time half; the resolve-time half is untouched
  (defense in depth: even a validation bypass can't leak into a resolved
  profile).
- **§3**: "Every request carries `tenant_id` explicitly ... via a bound
  field — never inferred from a nested resource ID." The company/department
  scoping this solution's checks run against is exactly this pattern, already
  used by `UpdateDepartment`/`UpdateUserProfile` (`tenant.RequireTenantID`).

**RBAC mechanism**: `07-security-architecture.md`'s AuthZ section
("Each service calls OPA (embedded, in-process — no network hop...) for
fine-grained, domain-specific checks that need data OPA doesn't have context
for at the gateway") is the load-bearing citation for *how* this gets built,
and `project-service`'s `internal/usecase/authorization.go` +
`internal/adapter/opaclient/client.go` + `policy/orca-authz/project.rego` is
the concrete, already-implemented instance of that pattern in this codebase
— this solution is a direct structural port of that trio into `tenant-service`,
not a new mechanism. Critically, OPA evaluation is **embedded/in-process**
(07-security-architecture.md's AuthZ section), so adding it does **not**
violate `tenant-service.md` §7's "no synchronous calls to any other Orca
service" rule — that rule is about outbound gRPC to other services, and OPA
policy evaluation makes no network hop at all.

**Known gap this solution inherits, not invents**: `project-service`'s
`callerGlobalRole` (`authorization.go:30-44`) is a documented stub returning
`""` always, because "no role claim propagates from api-gateway into a
service's request context yet." `auth-service`'s `requireAdminActor`
(`internal/usecase/authorization.go:19-39`) is the one place in the codebase
that *does* have a real role — but only because `auth-service` owns the
`users` table itself and loads `domain.User` directly; it never needed a
context-borne role claim. `tenant-service` is in `project-service`'s
position, not `auth-service`'s: it doesn't own `users`/roles, and per
`tenant-service.md` §7 it must **not** call `auth-service` synchronously to
fetch one (that rule's own example is specifically about the request path of
a hot RPC, and applies with equal force here — calling `auth-service` from
`UpdateCompany`/`UpdateDepartment` would create exactly the "depend on a
service that transitively depends on you" risk §7 forbids). This solution
therefore extends `common/tenant` with a `Role(ctx) (string, bool)` accessor
— parallel to the already-existing `TenantID`/`UserID` accessors
(`common/tenant/tenant.go:39-54`) — populated once the upstream JWT-role-claim
propagation gap (tracked identically at `project-service/internal/usecase/authorization.go:30-44`
and referenced by `annotation-service`'s `OPAClient.Decision` doc comment)
closes. Until then, `tenant.Role(ctx)` returns `"", false` and this
solution's authorization checks fail closed (deny), exactly like
`callerGlobalRole`'s inert admin-override branch — proven correct at the
Rego layer by `tenant_test.rego`, not reachable through Go code yet. This is
a **pre-existing, cross-cutting gap flagged consistently everywhere role
claims are needed**, not a new one this bug introduces.

**Company `agent.approvedModels`/`security.sessionTimeoutHours` fields**:
`tenant-service`'s `Company` has no dedicated Go fields for these — per
`domain/settings.go:7-18`, `Company.Settings` is a generic
`map[string]any`, ported 1:1 from the TS system's free-form profile JSON.
So "Validate `approved_models ⊆ SUPPORTED_MODELS`" means reading
`settings["agent"]["approvedModels"]` (an array of strings) out of the
already-decoded `Settings` map, matching how `ResolveProfile` itself reads
`agent.preferredModel` the same way (`profile_resolution.go`'s generic
`mergeInto` treats `agent` as an ordinary nested object).

---

## Design — domain validation

```go
// internal/domain/company.go (additions)

// SupportedModels is the closed list BL-PRF-01's "approved_models ⊆
// SUPPORTED_MODELS" rule validates against. A flat const list (not a DB
// table) — matches the TS reference's SUPPORTED_MODELS constant; revisit as
// a config value if it needs to change without a redeploy.
var SupportedModels = map[string]bool{
	"claude-opus-4-5":   true,
	"claude-sonnet-4-5": true,
	"codex":             true,
	"gemini":            true,
	"opencode":          true,
}

var (
	ErrUnsupportedModel     = errors.New("domain: model not in supported models list")
	ErrSessionTimeoutRange  = errors.New("domain: session_timeout_hours must be between 1 and 168")
)

// ValidateCompanySettings enforces BL-PRF-01's Company-layer field rules —
// called by usecase.UpdateCompany before persisting, never at resolve time
// (resolve time only enforces the security lock, per profile_resolution.go).
func ValidateCompanySettings(s Settings) error {
	if agent, ok := asMap(s["agent"]); ok {
		if models, ok := agent["approvedModels"].([]any); ok {
			for _, m := range models {
				name, _ := m.(string)
				if !SupportedModels[name] {
					return fmt.Errorf("%w: %q", ErrUnsupportedModel, name)
				}
			}
		}
	}
	if sec, ok := asMap(s["security"]); ok {
		if raw, present := sec["sessionTimeoutHours"]; present {
			hours, ok := raw.(float64) // JSON numbers decode as float64
			if !ok || hours < 1 || hours > 168 {
				return ErrSessionTimeoutRange
			}
		}
	}
	return nil
}

// ErrSecurityLockedToCompany is returned when a Department/User patch tries
// to set the "security" key — BL-PRF-01's "Dept setting security field ->
// 400" row. lockSecurity (profile_resolution.go) silently discards this at
// resolve time; this is the write-time rejection the spec's Error Cases
// table separately requires.
var ErrSecurityLockedToCompany = errors.New("domain: security settings can only be set at company level")

// ValidateDepartmentSettings rejects a "security" top-level key.
func ValidateDepartmentSettings(s Settings) error {
	if _, present := s["security"]; present {
		return ErrSecurityLockedToCompany
	}
	return nil
}

// ErrIntegrationsGithubOrgLocked mirrors ErrSecurityLockedToCompany for the
// one additional User-layer restriction BL-PRF-01 §4 names explicitly
// ("cannot set security.* or integrations.githubOrg").
var ErrIntegrationsGithubOrgLocked = errors.New("domain: integrations.githubOrg cannot be set at user level")

// ValidateUserSettings rejects "security" and "integrations.githubOrg".
func ValidateUserSettings(s Settings) error {
	if _, present := s["security"]; present {
		return ErrSecurityLockedToCompany
	}
	if integ, ok := asMap(s["integrations"]); ok {
		if _, present := integ["githubOrg"]; present {
			return ErrIntegrationsGithubOrgLocked
		}
	}
	return nil
}
```

`asMap`/nested-JSON-number handling reuse `domain/settings.go`'s existing
helpers — no new decoding convention introduced.

---

## Design — authorization (`tenant.rego` + `opaclient` + `usecase/authorization.go`)

Structural port of `project-service`'s trio
(`internal/usecase/authorization.go:1-84`, `internal/adapter/opaclient/client.go`,
`policy/orca-authz/project.rego`), with one addition: department-edit
authorization needs a same-department fact OPA can't compute itself (it has
no department-membership lookup) — the `07-security-architecture.md`
pattern this mirrors is `task-service`'s BFS-ancestor precedent: "the input
document passed to OPA includes whatever task-graph lookup `task-service`
already did; OPA evaluates the policy, `task-service` doesn't reimplement
the policy logic itself." Here, `tenant-service` already has the caller's
own `UserProfile.DepartmentID` one repository call away
(`UserProfileRepository.Get`), so it's passed as a precomputed boolean.

```rego
# backend-go/policy/orca-authz/tenant.rego
# input: {"caller_role": <string>, "action": <string>, "same_department": <bool>}
# caller_role is "admin" | "lead" | "user" | "" (no role claim yet — see
# tenant-service's authorization.go doc comment for the known upstream gap).
# action is "company_edit" | "department_edit".
package orca.authz.tenant

import rego.v1

default allow := false

allow if {
	input.action == "company_edit"
	input.caller_role == "admin"
}

allow if {
	input.action == "department_edit"
	input.caller_role == "admin"
}

allow if {
	input.action == "department_edit"
	input.caller_role == "lead"
	input.same_department == true
}
```

```go
// internal/usecase/authorization.go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

const (
	actionCompanyEdit    = "company_edit"
	actionDepartmentEdit = "department_edit"
)

// requireCompanyAdmin gates UpdateCompany — admin role only, per
// BL-PRF-01's Error Cases table ("Not admin (company edit) -> 403").
func requireCompanyAdmin(ctx context.Context, opa OPAClient) error {
	return decide(ctx, opa, actionCompanyEdit, false)
}

// requireDepartmentAccess gates UpdateDepartment/CreateDepartment — admin,
// or lead of the SAME department. sameDepartment is precomputed by the
// caller (it already has the actor's UserProfile.DepartmentID and the
// target department's id in hand) — OPA never does its own department
// lookup, per this file's doc comment.
func requireDepartmentAccess(ctx context.Context, opa OPAClient, sameDepartment bool) error {
	return decide(ctx, opa, actionDepartmentEdit, sameDepartment)
}

func decide(ctx context.Context, opa OPAClient, action string, sameDepartment bool) error {
	if _, ok := tenant.UserID(ctx); !ok {
		return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_ACTOR", "no authenticated user in request context", nil)
	}
	role, _ := tenant.Role(ctx) // "" until the upstream claim-propagation gap closes — fails closed below
	allowed, err := opa.Decision(ctx, role, action, sameDepartment)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_POLICY_EVAL_FAILED", "failed to evaluate authorization policy", err)
	}
	if !allowed {
		return apperrors.New(apperrors.KindPermissionDenied, "TENANT_NOT_AUTHORIZED", "caller is not authorized for this action", nil)
	}
	return nil
}
```

`OPAClient` port (added to `ports.go`):

```go
type OPAClient interface {
	Decision(ctx context.Context, callerRole, action string, sameDepartment bool) (bool, error)
}
```

---

## Design — usecase wiring

`UpdateCompany.Execute` (edit): after the existing `uc.companies.Update`
call, insert `requireCompanyAdmin` at the top and `domain.ValidateCompanySettings`
before the write:

```go
func (uc *UpdateCompany) Execute(ctx context.Context, in UpdateCompanyInput) (domain.Company, error) {
	if err := requireCompanyAdmin(ctx, uc.opa); err != nil {
		return domain.Company{}, err
	}
	if in.Patch.SettingsJSON != "" {
		var settings domain.Settings
		if err := json.Unmarshal([]byte(in.Patch.SettingsJSON), &settings); err != nil {
			return domain.Company{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_SETTINGS_JSON", "malformed settings_json", err)
		}
		if err := domain.ValidateCompanySettings(settings); err != nil {
			return domain.Company{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_COMPANY_SETTINGS", err.Error(), err)
		}
	}
	company, found, err := uc.companies.Update(ctx, in.ID, in.Patch)
	// ... unchanged from here ...

	if uc.audit != nil {
		actorID, _ := tenant.UserID(ctx)
		_ = uc.audit.PublishAuditEvent(ctx, in.ID, actorID, "company.profile.updated", in.ID)
	}
	return company, nil
}
```

`UpdateDepartment.Execute` (edit): resolve the actor's own department via
`uc.profiles.Get(ctx, companyID, actorID)`, compute `sameDepartment := actorProfile.DepartmentID == in.ID`,
call `requireDepartmentAccess`, then `domain.ValidateDepartmentSettings` on
the parsed patch before `uc.departments.Update`. Emits `department.profile.updated`
after success.

`CreateDepartment.Execute` (edit): same `requireDepartmentAccess` gate
(admin only in practice, since a lead can't create a department that
doesn't exist yet to be "their own" — `sameDepartment` is always `false`
here, so only `caller_role == "admin"` can pass, matching
`docs/logic/profile/BL-PRF-01-profile-crud.md:52`'s flow which only shows
`Admin` as the actor for department creation). Adds a name-uniqueness check
before `uc.departments.Create`:

```go
exists, err := uc.departments.ExistsByName(ctx, in.CompanyID, in.Name)
if err != nil {
	return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_DEPARTMENT_NAME_LOOKUP_FAILED", "failed to check department name uniqueness", err)
}
if exists {
	return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_DEPARTMENT_NAME_TAKEN", "a department with this name already exists", nil)
}
```

`DepartmentRepository` (`ports.go`) gains:
```go
// ExistsByName backs CreateDepartment's name-uniqueness check — scoped by
// companyID, same isolation posture as every other DepartmentRepository
// method (tenant-service.md §9).
ExistsByName(ctx context.Context, companyID, name string) (bool, error)
```
then `audit_log('department.created', adminId, deptId)` via `uc.audit`.

`UpdateUserProfile.Execute` (edit): two additions, both grounded directly
in BL-PRF-01 §4's flow text ("User Edits are self-service") which the
current code doesn't enforce at all — `in.UserID` is accepted from the
request with no check that it matches the caller:

```go
actorID, ok := tenant.UserID(ctx)
if !ok {
	return domain.UserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_ACTOR", "no authenticated user in request context", nil)
}
if actorID != in.UserID {
	// Self-service only — no admin-on-behalf-of path exists in BL-PRF-01's
	// flow (§4 shows only "User -> Settings -> My Profile -> Edit"); adding
	// one is a product decision, not this bug's scope.
	return domain.UserProfile{}, apperrors.New(apperrors.KindPermissionDenied, "TENANT_NOT_SELF", "users may only edit their own profile", nil)
}
if in.SetSettings {
	if err := domain.ValidateUserSettings(in.Settings); err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_USER_SETTINGS", err.Error(), err)
	}
}
```
No audit call added here — BL-PRF-01 §4 explicitly exempts personal-pref
updates ("no audit log for personal prefs — privacy"); this is a
**deliberate absence**, not an oversight, flagged so a future pass doesn't
"fix" it by adding one.

---

## Design — audit event emission (outbox, not a sync call to `auth-service`)

`07-security-architecture.md`'s Audit logging section: "`auth-service` owns
the system-wide audit log ... but every service emits structured audit
events (via the outbox pattern, same mechanism as domain events) for
security-relevant actions in its own domain." Confirmed by code: `auth-service`
exposes only `QueryAuditLog` (read) — no RPC exists for another service to
call and append a row — and `auth.audit_log`
(`auth-service/internal/adapter/postgres/audit_repository.go:19-27`) lives
in `auth-service`'s own physical database, which `tenant-service` cannot
write to directly (`05-data-architecture.md`'s "no cross-database queries...
full stop"). Combined with `tenant-service.md` §7's "no synchronous calls to
any other Orca service," the only TDD-consistent mechanism is: publish, per
service, don't call.

`tenant-service` already has exactly this shape —
`internal/adapter/eventbus/publisher.go`'s `PublishProfileInvalidated`
(`Subject = "orca.tenant.profile.invalidated"`, `StreamName = "TENANT"`).
Extend it with a second, best-effort-not-required publish (audit events, per
`credential-broker-service.md` §8's stricter "must never be best-effort"
posture, arguably deserve stronger delivery than cache invalidation — this
solution keeps the outbox-pattern shape but flags upgrading from
`commoneventbus.Publisher.Publish`'s at-least-once JetStream delivery to a
transactional-outbox-with-relay implementation, per `05-data-architecture.md`'s
default pattern, as the correctness bar to hit before this is treated as a
compliance-grade audit trail, not merely "logged"):

```go
// internal/adapter/eventbus/publisher.go (addition)
const AuditSubject = "orca.tenant.audit.recorded"

type auditPayload struct {
	Action   string `json:"action"`
	ActorID  string `json:"actor_id"`
	Target   string `json:"target"`
}

func (p *Publisher) PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error {
	payload, err := json.Marshal(auditPayload{Action: action, ActorID: actorID, Target: target})
	if err != nil {
		return fmt.Errorf("eventbus: marshal audit payload: %w", err)
	}
	return p.pub.Publish(ctx, AuditSubject, commoneventbus.Event{TenantID: tenantID, Payload: payload})
}
```

**Explicitly flagged as a needed follow-up, out of this bug's own scope**:
something must consume `orca.tenant.audit.recorded` and append into
`auth.audit_log` for these events to actually reach `QueryAuditLog` — most
naturally a small consumer inside `auth-service` (it already owns writes to
that table). This solution's fix is the **emission** side (closes "No audit
logging... zero non-test matches" per the bug report); wiring the
consumption side is `auth-service`'s work, not proposed here since it's a
different service's usecase/adapter surface — flag to whoever picks up
`auth-service`'s own audit-log TDD gaps.

`OPAClient`/`AuditPublisher` ports wire into `cmd/server/main.go` alongside
the existing `eventbus.Publisher` construction — no new external dependency
(OPA bundle path config mirrors `project-service`'s `main.go`).

---

## Test plan

- `internal/domain/company_test.go`: `ValidateCompanySettings` — approved
  model in list passes; unapproved model rejected; `sessionTimeoutHours`
  in `[1,168]` passes; `0`/`169`/non-numeric rejected; absent fields are a
  no-op (not required).
- `internal/domain/department_test.go`, `user_profile_test.go`:
  `ValidateDepartmentSettings`/`ValidateUserSettings` reject a `security`
  key; `ValidateUserSettings` additionally rejects `integrations.githubOrg`;
  both accept a patch with neither key present.
- `internal/usecase/update_company_test.go`: fake `OPAClient` returning
  `false` for role `"lead"`/`"user"`/`""` — `Execute` returns
  `KindPermissionDenied` without calling `companies.Update`. Fake returning
  `true` for `"admin"` — write proceeds. Invalid settings JSON short-circuits
  before the OPA call even runs (cheap-check-first) — or after, whichever
  order is chosen; assert exactly one ordering, not either.
- `internal/usecase/update_department_test.go`: `sameDepartment` computed
  correctly from a fake `UserProfileRepository.Get` — lead editing own dept
  passes, lead editing a different dept denied, admin passes regardless.
- `internal/usecase/create_department_test.go`: fake
  `DepartmentRepository.ExistsByName` returning `true` — `Execute` returns
  `KindInvalidArgument` without calling `Create`.
- `internal/usecase/update_user_profile_test.go`: `actorID != in.UserID` ->
  `KindPermissionDenied`; a `security`/`integrations.githubOrg` key in
  `in.Settings` -> `KindInvalidArgument`; confirm **no**
  `PublishAuditEvent` call happens on a successful user-profile update
  (regression guard for the deliberate audit exemption).
- `internal/adapter/opaclient/client_test.go` + `policy/orca-authz/tenant_test.rego`
  (`opa test`): the Rego policy's three `allow` branches, admin-override,
  and the default-deny-on-empty-role case — mirrors
  `policy/orca-authz/project_test.rego`'s coverage shape.
- `internal/adapter/eventbus/publisher_test.go`: `PublishAuditEvent`
  marshals the expected payload shape and calls `Publish` with
  `AuditSubject`.

## References

- `specs/backend-go/tdd/services/tenant-service.md:89-94` (bound `tenant_id`
  field posture), `:114-150` (merge algorithm, security-lock precedent),
  `:230-254` (§7 dependencies — "no synchronous calls to any other Orca
  service"), `:278-306` (§9 security notes — security-lock-is-a-security-control)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:24-52`
  (OPA AuthZ design, embedded/in-process, task-service BFS-input precedent),
  `:68-80` (audit logging — outbox pattern, per-service emission)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:75-98`
  (no cross-database writes; transactional outbox default pattern)
- `backend-go/services/project-service/internal/usecase/authorization.go:1-84`,
  `internal/adapter/opaclient/client.go`, `backend-go/policy/orca-authz/project.rego`
  — the OPA-gating trio this solution structurally ports
- `backend-go/services/auth-service/internal/usecase/authorization.go:19-39`
  (`requireAdminActor` — why `tenant-service` can't reuse this exact shape:
  it has no local `users` table and must not call `auth-service`)
- `backend-go/common/tenant/tenant.go:39-54` (`TenantID`/`UserID` accessor
  shape `Role(ctx)` extends)
- `backend-go/services/tenant-service/internal/domain/profile_resolution.go:171-186`
  (`lockSecurity` — resolve-time discard this solution's write-time reject complements)
- `backend-go/services/tenant-service/internal/domain/settings.go:7-45`
  (`Settings` as generic JSON map; `asMap` helper reused)
- `backend-go/services/tenant-service/internal/usecase/update_company.go`,
  `update_department.go`, `update_user_profile.go`, `create_department.go`,
  `ports.go` — usecases and ports this solution edits
- `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go`,
  `consumer.go` — existing best-effort NATS publish/subscribe shape extended
- `backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go:1-27`
  — `auth.audit_log`'s schema and why it can't be written cross-database
- `docs/logic/profile/BL-PRF-01-profile-crud.md:29-97` (flows), `:101-124`
  (Validation Rules / Error Cases tables — the exact source of every check
  this solution adds)
