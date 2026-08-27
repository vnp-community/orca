package domain

import "errors"

// UserProfile is the per-user profile-override row — 1:1 with a user
// (logical FK to auth-service), holding the department assignment and the
// user's own Settings override layer, the highest-priority layer in
// ResolveProfile (tenant-service.md §4).
type UserProfile struct {
	UserID    string
	CompanyID string
	// DepartmentID empty means company-only inheritance: no department
	// layer contributes to this user's resolved profile.
	DepartmentID string
	Settings     Settings
}

// NewUserProfile constructs a UserProfile, enforcing the invariants every
// row in tenant.user_profiles must satisfy. DepartmentID is intentionally
// not validated as non-empty — an unset department is a valid, common state
// (tenant-service.md §5).
func NewUserProfile(userID, companyID, departmentID string, settings Settings) (UserProfile, error) {
	if userID == "" {
		return UserProfile{}, ErrEmptyUserID
	}
	if companyID == "" {
		return UserProfile{}, ErrEmptyID
	}
	return UserProfile{
		UserID:       userID,
		CompanyID:    companyID,
		DepartmentID: departmentID,
		Settings:     emptySettings(settings),
	}, nil
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
