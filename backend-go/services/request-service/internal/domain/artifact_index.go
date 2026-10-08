package domain

import (
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// IndexEntry is one row of artifact_index: a display id and the UUID behind it. Options carry their
// solution's UUID, since an option has no key of its own.
type IndexEntry struct {
	TenantID   string
	DisplayID  string
	Kind       DisplayKind
	RequestID  string
	ArtifactID string
	CreatedAt  time.Time
}

// IndexKindFor maps a parsed display id onto the kind stored in artifact_index (the CHECK has six values).
func IndexKindFor(k DisplayKind) (string, bool) {
	switch k {
	case DisplayKindRequest, DisplayKindSolution, DisplayKindOption, DisplayKindPlan, DisplayKindPhase, DisplayKindTask:
		return string(k), true
	}
	return "", false
}

// CoverageRow links an acceptance criterion to the task (and optionally the check) that covers it.
type CoverageRow struct {
	ACID       string
	TaskID     string
	CheckID    string
	PlanTaskID string
}

func ErrArtifactNotFound(ref string) error {
	return newNotFound("REQUEST_ARTIFACT_NOT_FOUND", "artifact not found: "+ref)
}

func ErrArtifactSeqConflict() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ARTIFACT_SEQ_CONFLICT", "another solution took this sequence number; retry", nil)
}
