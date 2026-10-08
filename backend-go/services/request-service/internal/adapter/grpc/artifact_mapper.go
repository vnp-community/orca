package grpc

import (
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// canonicalJSONString returns the canonical form of stored JSON, or the input when it cannot be canonicalised.
func canonicalJSONString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if c, err := domain.CanonicalJSON(raw); err == nil {
		return string(c)
	}
	return string(raw)
}

func toProtoRevisionSummary(r domain.RequestRevision) *requestv1.RequestRevisionSummary {
	cause := string(r.Cause)
	if r.Cause == domain.RevisionCauseClarificationAnswered {
		cause = "clarified" // the wire vocabulary of artifact.proto
	}
	if r.Cause == domain.RevisionCauseEdited && strings.Contains(string(r.Snapshot), `"waiver"`) {
		cause = "waived"
	}
	return &requestv1.RequestRevisionSummary{
		Revision: int32(r.Revision), Cause: cause, Digest: r.Digest, ActorId: r.ActorID, ActorKind: string(r.ActorKind),
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
}

func toProtoEdge(e usecase.ArtifactEdge) *requestv1.ArtifactEdge {
	return &requestv1.ArtifactEdge{Rel: string(e.Rel), FromKind: string(e.FromKind), FromId: e.FromID, ToKind: string(e.ToKind), ToId: e.ToID}
}

func toProtoCoverageRow(r domain.CoverageRow) *requestv1.CoverageRow {
	return &requestv1.CoverageRow{AcId: r.ACID, TaskId: r.TaskID, CheckId: r.CheckID, PlanTaskId: r.PlanTaskID}
}
