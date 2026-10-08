package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

type fakePlanTreeCreator struct {
	got usecase.CreatePlanTreeInput
	res usecase.CreatePlanTreeResult
	err error
}

func (f *fakePlanTreeCreator) Execute(_ context.Context, in usecase.CreatePlanTreeInput) (usecase.CreatePlanTreeResult, error) {
	f.got = in
	return f.res, f.err
}

func TestServer_CreatePlanTree_MapsErrors(t *testing.T) {
	cases := map[string]struct {
		err  error
		code codes.Code
	}{
		"TASK_PLAN_HAS_RUNNING_TASKS":  {apperrors.New(apperrors.KindFailedPrecondition, "TASK_PLAN_HAS_RUNNING_TASKS", "x", nil), codes.FailedPrecondition},
		"TASK_PLAN_SUPERSEDE_MISMATCH": {apperrors.New(apperrors.KindFailedPrecondition, "TASK_PLAN_SUPERSEDE_MISMATCH", "x", nil), codes.FailedPrecondition},
		"TASK_PLAN_TREE_INVALID":       {apperrors.New(apperrors.KindInvalidArgument, "TASK_PLAN_TREE_INVALID", "x", nil), codes.InvalidArgument},
	}
	for name, c := range cases {
		s := (&Server{}).WithCreatePlanTree(&fakePlanTreeCreator{err: c.err})
		_, err := s.CreatePlanTree(context.Background(), &taskv1.CreatePlanTreeRequest{})
		if status.Code(err) != c.code {
			t.Errorf("%s: got %v want %v", name, status.Code(err), c.code)
		}
	}
}

func TestServer_CreatePlanTree_MapsRequestAndResponse(t *testing.T) {
	fake := &fakePlanTreeCreator{res: usecase.CreatePlanTreeResult{
		Plan:   domain.Task{ID: "p", Type: domain.TypePlan},
		Phases: []domain.Task{{ID: "ph"}}, Tasks: []domain.Task{{ID: "t1"}, {ID: "t2"}}, AlreadyExists: true,
	}}
	s := (&Server{}).WithCreatePlanTree(fake)
	resp, err := s.CreatePlanTree(context.Background(), &taskv1.CreatePlanTreeRequest{
		RequestId: "r", ProjectId: "pr", Title: "T", CreatorId: "u", SupersedesPlanId: "old",
		Phases: []*taskv1.PlanTreePhase{{Title: "P", DependsOnPhaseIndices: []int32{0}, Tasks: []*taskv1.PlanTreeTask{{Title: "a", DependsOnIndices: []int32{1}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Plan.Id != "p" || len(resp.Phases) != 1 || len(resp.Tasks) != 2 || !resp.AlreadyExists {
		t.Fatalf("bad response: %+v", resp)
	}
	if fake.got.RequestID != "r" || fake.got.SupersedesPlanID != "old" || len(fake.got.Phases) != 1 ||
		fake.got.Phases[0].DependsOnPhaseIndices[0] != 0 || fake.got.Phases[0].Tasks[0].DependsOnIndices[0] != 1 {
		t.Fatalf("bad input mapping: %+v", fake.got)
	}
}

func TestServer_CreatePlanTree_NotWired_Unimplemented(t *testing.T) {
	_, err := (&Server{}).CreatePlanTree(context.Background(), &taskv1.CreatePlanTreeRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v", err)
	}
}
