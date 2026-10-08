package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// MaxTaskSpecBytes caps one spec; a bigger one belongs in the Request's documents, not on the task.
const MaxTaskSpecBytes = 32 * 1024

var (
	ErrTaskSpecInvalid         = errors.New("task spec: invalid")
	ErrTaskSpecLocked          = errors.New("task spec: locked by an approved plan")
	ErrTaskSpecVersionConflict = errors.New("task spec: version conflict")
	ErrTaskSpecNotFound        = errors.New("task spec: not found")
)

// TaskSpec is the canonical, versioned spec of one task (CR-REQ-027). Spec holds canonical JSON
// so Digest is reproducible by any reader; Version is the optimistic-concurrency counter.
type TaskSpec struct {
	TaskID        string
	TenantID      string
	SchemaVersion int
	Spec          []byte
	Digest        string
	LockedAt      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int64
}

// NewTaskSpec validates spec (a JSON object up to MaxTaskSpecBytes) and fills the canonical form and digest.
func NewTaskSpec(taskID, tenantID string, schemaVersion int, spec []byte) (TaskSpec, error) {
	if taskID == "" || tenantID == "" {
		return TaskSpec{}, fmt.Errorf("%w: task id and tenant id are required", ErrTaskSpecInvalid)
	}
	if schemaVersion < 1 {
		return TaskSpec{}, fmt.Errorf("%w: schema_version must be >= 1", ErrTaskSpecInvalid)
	}
	if len(spec) > MaxTaskSpecBytes {
		return TaskSpec{}, fmt.Errorf("%w: spec is %d bytes, limit is %d", ErrTaskSpecInvalid, len(spec), MaxTaskSpecBytes)
	}
	if !json.Valid(spec) {
		return TaskSpec{}, fmt.Errorf("%w: spec is not valid JSON", ErrTaskSpecInvalid)
	}
	canonical, err := CanonicalJSON(spec)
	if err != nil {
		return TaskSpec{}, fmt.Errorf("%w: %v", ErrTaskSpecInvalid, err)
	}
	if len(canonical) == 0 || canonical[0] != '{' {
		return TaskSpec{}, fmt.Errorf("%w: spec must be a JSON object", ErrTaskSpecInvalid)
	}
	if len(canonical) > MaxTaskSpecBytes {
		return TaskSpec{}, fmt.Errorf("%w: canonical spec is %d bytes, limit is %d", ErrTaskSpecInvalid, len(canonical), MaxTaskSpecBytes)
	}
	return TaskSpec{TaskID: taskID, TenantID: tenantID, SchemaVersion: schemaVersion, Spec: canonical, Digest: DigestOfCanonical(canonical)}, nil
}

// DigestOfCanonical is the lowercase hex SHA-256 stored in task_specs.digest.
func DigestOfCanonical(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func (s TaskSpec) IsLocked() bool { return s.LockedAt != nil }

// Canonicalized re-renders Spec: JSONB/JSON columns hand back reformatted text, and readers
// hash the bytes they receive, so every read goes through here before leaving the service.
func (s TaskSpec) Canonicalized() (TaskSpec, error) {
	canonical, err := CanonicalJSON(s.Spec)
	if err != nil {
		return s, fmt.Errorf("%w: stored spec is not canonicalizable: %v", ErrTaskSpecInvalid, err)
	}
	s.Spec = canonical
	return s, nil
}
