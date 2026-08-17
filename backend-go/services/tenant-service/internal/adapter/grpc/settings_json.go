package grpc

import (
	"encoding/json"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// marshalSettings is the gRPC layer's own JSON boundary (companies.md's
// settings_json wire field) — kept separate from
// internal/adapter/postgres's identical helper since the two packages must
// not import each other, per architecture/03's layering rules.
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

// unmarshalSettings is this boundary's inbound counterpart to
// marshalSettings — used where a request carries a caller-supplied
// settings_json (currently only CreateTeamRequest).
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
