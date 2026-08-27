// Package postgres implements tenant-service's repository ports (defined in
// internal/usecase) against this service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in tenant-service
// that knows SQL exists. One repository type per aggregate (CompanyRepository,
// DepartmentRepository, TeamRepository, UserProfileRepository), per
// tenant-service.md §6's package-layout notes.
package postgres

import (
	"encoding/json"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// marshalSettings/unmarshalSettings are the ONLY place tenant-service
// converts domain.Settings to/from its settings_json JSONB wire
// representation — domain code never touches encoding/json directly, per
// architecture/03-clean-architecture-guidelines.md's "pure domain, no I/O"
// rule.
func marshalSettings(s domain.Settings) (string, error) {
	if len(s) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]any(s))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalSettings(raw string) (domain.Settings, error) {
	if raw == "" {
		return domain.Settings{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return domain.Settings(m), nil
}
