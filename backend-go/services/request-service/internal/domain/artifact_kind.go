package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ArtifactKind names a versioned document type with its own JSON Schema (CR-REQ-027 section 2.1).
type ArtifactKind string

const (
	ArtifactKindRequest   ArtifactKind = "request"
	ArtifactKindSolution  ArtifactKind = "solution"
	ArtifactKindDiagnosis ArtifactKind = "diagnosis"
	ArtifactKindFindings  ArtifactKind = "findings"
	ArtifactKindAnswer    ArtifactKind = "answer"
	ArtifactKindPlan      ArtifactKind = "plan"
	ArtifactKindPhase     ArtifactKind = "phase"
	ArtifactKindTask      ArtifactKind = "task"
)

func AllArtifactKinds() []ArtifactKind {
	return []ArtifactKind{
		ArtifactKindRequest, ArtifactKindSolution, ArtifactKindDiagnosis, ArtifactKindFindings,
		ArtifactKindAnswer, ArtifactKindPlan, ArtifactKindPhase, ArtifactKindTask,
	}
}

func ParseArtifactKind(s string) (ArtifactKind, error) {
	for _, k := range AllArtifactKinds() {
		if string(k) == s {
			return k, nil
		}
	}
	return "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ARTIFACT_KIND_INVALID", fmt.Sprintf("unknown artifact kind: %s", s), nil)
}

// LatestSchemaVersion is the major version writers must emit; readers upgrade older ones on read.
func LatestSchemaVersion(ArtifactKind) int { return 1 }

// MaxArtifactBytes holds proposed (unmeasured) size caps per document.
func MaxArtifactBytes(k ArtifactKind) int {
	switch k {
	case ArtifactKindRequest:
		return 128 * 1024
	case ArtifactKindPlan:
		return 256 * 1024
	case ArtifactKindTask:
		return 32 * 1024
	default:
		return 64 * 1024
	}
}
