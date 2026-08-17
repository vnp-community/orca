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
