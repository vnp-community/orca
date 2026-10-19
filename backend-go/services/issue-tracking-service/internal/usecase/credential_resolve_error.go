package usecase

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
)

// credentialResolveError separates "the user has no usable connection" (a precondition the UI
// resolves by connecting) from a broker or database outage, which must not read as not-connected.
func credentialResolveError(err error, notConnectedMessage string) *apperrors.AppError {
	if errors.Is(err, ErrConnectionNotFound) {
		return apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", notConnectedMessage, err)
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound, codes.FailedPrecondition, codes.InvalidArgument, codes.PermissionDenied, codes.Unauthenticated:
			return apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", notConnectedMessage, err)
		case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
			return apperrors.New(apperrors.KindUnavailable, "ISSUETRACKING_CREDENTIAL_LOOKUP_FAILED", "credential lookup did not complete", err)
		}
	}
	return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREDENTIAL_LOOKUP_FAILED", "failed to resolve the provider credential", err)
}
