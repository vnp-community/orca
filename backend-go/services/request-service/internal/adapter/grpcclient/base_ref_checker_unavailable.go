package grpcclient

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// UnavailableBaseRefChecker is wired while no git-gateway based checker is connected. It answers Unreachable for every ref, so
// the use cases store the value flagged as unverified (the fail-open path) instead of rejecting it or pretending it was checked.
type UnavailableBaseRefChecker struct{}

var _ usecase.BaseRefChecker = UnavailableBaseRefChecker{}

func (UnavailableBaseRefChecker) Check(context.Context, string, string) (usecase.BaseRefCheck, error) {
	return usecase.BaseRefCheck{Status: usecase.BaseRefUnreachable}, nil
}
