// Package mysql implements tenant-service's repository ports (defined in
// internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — the multi-dialect rollout adapter for
// CR-DB-002/CR-DB-003, mirroring internal/adapter/postgres's behavior
// (tenant scoping, upsert semantics, not-found sentinels) 1:1 against the
// dialect-safe schema created by migrations/mysql (BE-DB-SOL-011). One
// repository type per aggregate, same package-layout convention as
// internal/adapter/postgres.
package mysql

import (
	"encoding/json"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// marshalSettings/unmarshalSettings mirror
// internal/adapter/postgres.marshalSettings/unmarshalSettings exactly — the
// wire representation (a JSON string) is dialect-agnostic, only the column
// type storing it differs (JSONB vs JSON, see migrations/mysql/0001_init.up.sql).
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
