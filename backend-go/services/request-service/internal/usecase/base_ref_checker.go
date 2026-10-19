package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// BaseRefStatus is what git said about a ref.
type BaseRefStatus string

const (
	BaseRefFound    BaseRefStatus = "found"
	BaseRefNotFound BaseRefStatus = "not_found"
	// BaseRefUnreachable covers every failure that says nothing about the ref itself: dev server offline, network error, timeout.
	BaseRefUnreachable BaseRefStatus = "unreachable"
)

// BaseRefCheck is the answer of a BaseRefChecker; Suggestions are known refs close to the one asked about (NotFound only).
type BaseRefCheck struct {
	Status      BaseRefStatus
	Suggestions []string
}

// BaseRefChecker asks the repository of the project whether a branch or tag exists. It must tell "git says no" (NotFound)
// from "could not ask" (Unreachable): the first rejects the value, the second only flags it as unverified.
type BaseRefChecker interface {
	Check(ctx context.Context, projectID, baseRef string) (BaseRefCheck, error)
}

// verifyBaseRef is the one policy for both CreateRequest and SetRequestBaseRef. It fails closed only when git says the ref does
// not exist; a checker that cannot answer (an error or Unreachable) leaves the value stored but unverified, so an offline dev
// server never blocks filing a Request. An empty ref means the repository default and needs no check.
func verifyBaseRef(ctx context.Context, checker BaseRefChecker, requestID, projectID string, ref domain.WorktreeBaseRef) (unverified bool, err error) {
	if ref == "" {
		return false, nil
	}
	if checker == nil {
		return false, domain.ErrRequestBaseRefCheckNotConfigured()
	}
	res, cerr := checker.Check(ctx, projectID, string(ref))
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if cerr != nil {
		var ae *apperrors.AppError
		if errors.As(cerr, &ae) && ae.Code == "REQUEST_BASE_REF_CHECK_NOT_CONFIGURED" {
			return false, cerr // a deployment defect, not an unreachable dev server: say so instead of storing the ref unchecked
		}
		slog.WarnContext(ctx, "base ref could not be checked; stored unverified", slog.String("request_id", requestID), slog.Any("error", cerr))
		return true, nil
	}
	switch res.Status {
	case BaseRefFound:
		return false, nil
	case BaseRefNotFound:
		return false, domain.ErrRequestBaseRefNotFound(string(ref), res.Suggestions)
	default:
		return true, nil
	}
}
