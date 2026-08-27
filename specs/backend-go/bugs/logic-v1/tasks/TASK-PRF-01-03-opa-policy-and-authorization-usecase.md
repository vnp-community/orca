# TASK-PRF-01-03: Port the OPA-gating trio into `tenant-service` (`tenant.rego` + `opaclient` + `authorization.go`)

**From Solution:** SOL-PRF-01
**Priority:** P0 — TASK-PRF-01-05/06 (usecase wiring) call `requireCompanyAdmin`/`requireDepartmentAccess` from this file
**Service:** `tenant-service`
**File:** `backend-go/policy/orca-authz/tenant.rego`
**Depends on:** TASK-PRF-01-02
**Status:** `[ ]` TODO

---

## Context

`tenant-service` has no authorization layer at all today — `UpdateCompany`/
`UpdateDepartment`/`CreateDepartment` execute unconditionally regardless of
caller role. This is a structural port of `project-service`'s already-working
OPA trio (`internal/usecase/authorization.go` + `internal/adapter/opaclient`
+ `policy/orca-authz/project.rego`), the same embedded/in-process pattern
`07-security-architecture.md` documents (no network hop — doesn't violate
`tenant-service.md` §7's "no synchronous calls to any other Orca service").

## Changes to make

Create `backend-go/policy/orca-authz/tenant.rego`:

```rego
# tenant-service RBAC — company_edit is admin-only; department_edit is admin
# OR a lead of the same department (same_department precomputed by the
# caller — see tenant-service's authorization.go doc comment for why OPA
# doesn't do its own department lookup, mirroring task_grant.rego's
# BFS-precomputed-input pattern).
#
# input shape: {"caller_role": <string>, "action": <string>, "same_department": <bool>}
# caller_role is "admin" | "lead" | "user" | "" (no role claim yet — see
# common/tenant.Role's doc comment for the known upstream gap).
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

Create `backend-go/services/tenant-service/internal/adapter/opaclient/client.go`
(mirrors `project-service/internal/adapter/opaclient/client.go` exactly):

```go
// Package opaclient adapts common/policy.Evaluator to tenant-service's
// caller-role authorization check. Mirrors project-service/auth-service/
// annotation-service/task-service's own internal/adapter/opaclient shape.
package opaclient

import (
	"context"

	"github.com/stablyai/orca-go/common/policy"
)

// decisionQuery is the fully-qualified Rego rule this service evaluates —
// see backend-go/policy/orca-authz/tenant.rego.
const decisionQuery = "data.orca.authz.tenant.allow"

// Client evaluates tenant-service's role-based authorization policy via the
// shared embedded-OPA evaluator (common/policy).
type Client struct {
	evaluator *policy.Evaluator
}

// New wraps evaluator for tenant-service's authorization check. evaluator is
// constructed once in cmd/server/main.go, pointed at config.Config's
// OPABundlePath, and shared across every decide call.
func New(evaluator *policy.Evaluator) *Client {
	return &Client{evaluator: evaluator}
}

func (c *Client) Decision(ctx context.Context, callerRole, action string, sameDepartment bool) (bool, error) {
	return c.evaluator.Decision(ctx, decisionQuery, map[string]any{
		"caller_role":     callerRole,
		"action":          action,
		"same_department": sameDepartment,
	})
}
```

Add to `backend-go/services/tenant-service/internal/usecase/ports.go`:

```go
// OPAClient is the authorization port UpdateCompany/UpdateDepartment/
// CreateDepartment use for the "does this caller_role/same_department
// authorize this action" decision — implemented by internal/adapter/
// opaclient against the shared embedded OPA evaluator (common/policy),
// consuming backend-go/policy/orca-authz/tenant.rego's
// data.orca.authz.tenant.allow rule. Mirrors project-service's own
// OPAClient port shape.
type OPAClient interface {
	Decision(ctx context.Context, callerRole, action string, sameDepartment bool) (bool, error)
}
```

Create `backend-go/services/tenant-service/internal/usecase/authorization.go`:

```go
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

## Verify

```bash
cd /opt/repos/orca/backend-go
opa test policy/orca-authz/tenant.rego policy/orca-authz/tenant_test.rego -v
go build ./services/tenant-service/...
```

Add `backend-go/policy/orca-authz/tenant_test.rego` covering the three
`allow` branches plus the default-deny-on-empty-role case, mirroring
`project_test.rego`'s coverage shape. Add
`internal/adapter/opaclient/client_test.go` asserting the input map shape
sent to `evaluator.Decision`.
