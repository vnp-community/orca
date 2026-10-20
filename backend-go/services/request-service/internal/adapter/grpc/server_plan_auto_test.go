package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetryPlanGeneration_UnimplementedUntilWired(t *testing.T) {
	_, err := NewServer(nil, nil).RetryPlanGeneration(context.Background(), &requestv1.RetryPlanGenerationRequest{RequestId: "r"})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("err = %v", err)
	}
}

func TestToProtoPlanAuto(t *testing.T) {
	if toProtoPlanAuto(nil) != nil {
		t.Fatal("a server without the feature sends no block")
	}
	next := time.Date(2026, 10, 20, 10, 30, 0, 0, time.UTC)
	got := toProtoPlanAuto(&usecase.PlanAutoView{Enabled: true, State: "pending", Origin: "event", Attempt: 2, MaxAttempts: 2, ReasonCode: "REQUEST_AI_COMPLETE_TIMEOUT", NextAttemptAt: next})
	want := &requestv1.PlanAutoState{Enabled: true, State: "pending", Origin: "event", Attempt: 2, MaxAttempts: 2, ReasonCode: "REQUEST_AI_COMPLETE_TIMEOUT", NextAttemptAt: "2026-10-20T10:30:00Z"}
	if got.String() != want.String() {
		t.Fatalf("got %v, want %v", got, want)
	}
	if empty := toProtoPlanAuto(&usecase.PlanAutoView{Enabled: false}); empty == nil || empty.GetNextAttemptAt() != "" || empty.GetEnabled() {
		t.Fatalf("empty view = %v", empty)
	}
}

// requestsFailingGet answers every other method with the zero value through the embedded interface being nil: only Get is used.
type requestsFailingGet struct {
	usecase.RequestRepository
	err error
}

func (r requestsFailingGet) Get(context.Context, string) (domain.Request, error) {
	return domain.Request{}, r.err
}

func TestRetryPlanGeneration_MapsUseCaseErrorsToStatusCodes(t *testing.T) {
	retry := usecase.NewRetryPlanGeneration(requestsFailingGet{err: domain.ErrPlanWrongState(domain.RequestStatusAnalyzing)}, nil, nil, nil, usecase.PlanAutoSwitch{})
	s := NewServer(nil, nil).WithPlan(PlanUseCases{Retry: retry})
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")
	_, err := s.RetryPlanGeneration(ctx, &requestv1.RetryPlanGenerationRequest{RequestId: "r"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("err = %v", err)
	}
	_, err = s.RetryPlanGeneration(context.Background(), &requestv1.RetryPlanGenerationRequest{RequestId: "r"})
	if status.Code(err) == codes.OK {
		t.Fatal("a call without a tenant must be refused")
	}
}
