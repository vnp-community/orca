package domain

import "errors"

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
