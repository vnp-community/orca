package domain

// Department belongs to a Company and holds its own Settings override layer
// — the second-lowest-priority layer in ResolveProfile (tenant-service.md
// §4/§5). Deliberately no parent_department_id field: the design doc's
// self-referencing tree shape (§5) isn't exercised by any RPC in
// tenant.proto's current surface (no ListDepartments/hierarchy RPC), so it
// isn't modeled here — see this service's README "Known gaps".
type Department struct {
	ID        string
	CompanyID string
	Name      string
	Settings  Settings
}

// NewDepartment constructs a Department, enforcing the invariants every row
// in tenant.departments must satisfy.
func NewDepartment(id, companyID, name string, settings Settings) (Department, error) {
	if id == "" {
		return Department{}, ErrEmptyID
	}
	if companyID == "" {
		return Department{}, ErrEmptyID
	}
	if name == "" {
		return Department{}, ErrEmptyName
	}
	return Department{ID: id, CompanyID: companyID, Name: name, Settings: emptySettings(settings)}, nil
}

// DepartmentSettingsPatch carries UpdateDepartment's field-mask semantics —
// same "" = no change convention as CompanySettingsPatch.
type DepartmentSettingsPatch struct {
	Name         string
	SettingsJSON string
}

// ValidateDepartmentSettings rejects a "security" top-level key — see
// ErrSecurityLockedToCompany in company.go.
func ValidateDepartmentSettings(s Settings) error {
	if _, present := s["security"]; present {
		return ErrSecurityLockedToCompany
	}
	return nil
}
