package grpc

import (
	"context"
	"reflect"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPlanProposalMapping_RoundTrips(t *testing.T) {
	p := domain.PlanProposal{
		Title: "Plan", Summary: "s", Notes: "n",
		Phases: []domain.PhaseProposal{
			{Title: "P1", Description: "d", Tasks: []domain.TaskProposal{{Title: "a", TaskType: "task", EstimatedHours: 2.5, Labels: []string{"x"}, Irreversible: true}}},
			{Title: "P2", DependsOnPhaseIndices: []int{0}, Tasks: []domain.TaskProposal{{Title: "b", TaskType: "bug", DependsOnIndices: nil}}},
		},
		Tasks: nil,
	}
	got := planProposalFromProto(toProtoPlanProposal(p))
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("round trip changed the proposal:\n got %+v\nwant %+v", got, p)
	}
	flat := domain.PlanProposal{Title: "F", Tasks: []domain.TaskProposal{{Title: "a", TaskType: "task"}, {Title: "b", TaskType: "task", DependsOnIndices: []int{0}}}}
	if got := planProposalFromProto(toProtoPlanProposal(flat)); !reflect.DeepEqual(got, flat) {
		t.Fatalf("flat plan changed: %+v", got)
	}
	if planProposalFromProto(nil).Title != "" {
		t.Fatal("a missing proposal maps to an empty one")
	}
}

func TestPlanRPCs_AreUnimplementedUntilWired(t *testing.T) {
	s := NewServer(nil, nil)
	ctx := context.Background()
	_, err := s.GeneratePlan(ctx, &requestv1.GeneratePlanRequest{RequestId: "r"})
	_, err2 := s.GetPlanProposal(ctx, &requestv1.GetPlanProposalRequest{RequestId: "r"})
	_, err3 := s.CommitPlan(ctx, &requestv1.CommitPlanRequest{RequestId: "r"})
	for i, e := range []error{err, err2, err3} {
		if status.Code(e) != codes.Unimplemented {
			t.Errorf("rpc %d: %v", i, e)
		}
	}
}
