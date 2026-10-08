package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// OpenApprovalOpener adapts OpenApproval to the solution worker's SolutionApprovalOpener port. The handler computes the
// digest, so the caller's digest is only a cross-check that both sides hash the same document.
type OpenApprovalOpener struct {
	Approvals *OpenApproval
}

var _ SolutionApprovalOpener = OpenApprovalOpener{}

func (o OpenApprovalOpener) Open(ctx context.Context, in OpenSolutionApprovalInput) error {
	a, err := o.Approvals.Execute(ctx, OpenApprovalInput{
		RequestID: in.Request.ID, SubjectType: in.SubjectType, SubjectID: in.SubjectID, RequestedBy: systemActor,
	})
	if err != nil {
		return err
	}
	if in.Digest != "" && a.SubjectDigest != in.Digest {
		return fmt.Errorf("%w: handler digest %s differs from the proposal digest", domain.ErrApprovalDigestMismatch, a.SubjectDigest)
	}
	return nil
}
