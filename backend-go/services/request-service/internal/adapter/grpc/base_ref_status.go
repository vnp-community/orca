package grpc

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

// baseRefErrorDomain scopes the ErrorInfo of the base ref errors for clients that read status details.
const baseRefErrorDomain = "orca.request.v1"

// baseRefStatus is apperrors.ToGRPCStatus plus an ErrorInfo detail carrying what the three REQUEST_BASE_REF_* errors know
// (the lock reason, the suggested refs), so a client does not parse them out of the message.
func baseRefStatus(err error) error {
	st := status.Convert(apperrors.ToGRPCStatus(err))
	details, ok := domain.BaseRefDetails(err)
	if !ok {
		return st.Err()
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		return st.Err()
	}
	withInfo, derr := st.WithDetails(&errdetails.ErrorInfo{Reason: ae.Code, Domain: baseRefErrorDomain, Metadata: details})
	if derr != nil {
		return st.Err() // the status itself is still right; only the extra detail is lost
	}
	return withInfo.Err()
}
