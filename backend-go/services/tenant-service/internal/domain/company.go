package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrEmptyID is returned when a required id/companyID field is empty —
	// no entity in this service is ever meaningfully anonymous.
	ErrEmptyID = errors.New("domain: id is required")
	// ErrEmptyName is returned when a required name field is empty.
	ErrEmptyName = errors.New("domain: name is required")
)

// Company is the tenant root. tenant-service is the ONE service where an
// entity's own id doubles as the tenant_id every other service's schema
// logically references — "tenant.companies.id IS the tenant_id" per
// tenant-service.md §1/§5. Unlike every other entity in this service,
// Company has no companyID/tenantID field of its own to be scoped by.
type Company struct {
	ID   string
	Name string
	// Settings is the company-layer defaults — the base (lowest-priority)
	// layer of every profile merge, and the ONLY layer allowed to define
	// the "security" section (company-locked, tenant-service.md §4/§9).
	Settings Settings
}

// NewCompany constructs a Company, enforcing the invariants every row in
// tenant.companies must satisfy.
func NewCompany(id, name string, settings Settings) (Company, error) {
	if id == "" {
		return Company{}, ErrEmptyID
	}
	if name == "" {
		return Company{}, ErrEmptyName
	}
	return Company{ID: id, Name: name, Settings: emptySettings(settings)}, nil
}

// CompanySettingsPatch carries UpdateCompany's field-mask semantics: an
// empty string means "leave unchanged" — mirrors project-service's
// ProjectUpdatePatch convention (project-service/internal/domain/project.go).
type CompanySettingsPatch struct {
	Name         string
	SettingsJSON string // "" = no change; parsed to Settings by the usecase
}

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
