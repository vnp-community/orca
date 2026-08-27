# TASK-PRF-01-01: Add write-time field validation functions to Company/Department/UserProfile domain

**From Solution:** SOL-PRF-01
**Priority:** P0 — every usecase-wiring task in this set depends on these existing
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/domain/company.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-PRF-01's Error Cases table requires 400s for unapproved models, out-of-range
session timeouts, and any attempt to set `security`/`integrations.githubOrg`
below the company layer — none of these checks exist today (`ResolveProfile`
only enforces the security lock at *resolve* time, per
`profile_resolution.go`'s `lockSecurity`). This task adds the write-time
domain functions the usecase-wiring tasks (TASK-PRF-01-05/06/07) call before
persisting a patch.

## Changes to make

In `backend-go/services/tenant-service/internal/domain/company.go`, append:

```go
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
	ErrUnsupportedModel    = errors.New("domain: model not in supported models list")
	ErrSessionTimeoutRange = errors.New("domain: session_timeout_hours must be between 1 and 168")
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
```

`import` block needs `"fmt"` added alongside the existing `"errors"`.

In `backend-go/services/tenant-service/internal/domain/department.go`, append:

```go
// ValidateDepartmentSettings rejects a "security" top-level key — see
// ErrSecurityLockedToCompany in company.go.
func ValidateDepartmentSettings(s Settings) error {
	if _, present := s["security"]; present {
		return ErrSecurityLockedToCompany
	}
	return nil
}
```

In `backend-go/services/tenant-service/internal/domain/user_profile.go`, append:

```go
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

`user_profile.go` currently has no `import` block (no stdlib use) — add
`import "errors"` at the top.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/tenant-service/...
go vet ./services/tenant-service/internal/domain/...
```

Add table-driven tests to `company_test.go`/`department_test.go`/
`user_profile_test.go` per SOL-PRF-01's Test plan (approved/unapproved model,
timeout in/out of `[1,168]`, absent fields are a no-op, `security`/
`integrations.githubOrg` rejection), then:

```bash
go test ./services/tenant-service/internal/domain/... -run 'Validate' -v
```
