package grpc

import (
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSolutionRPCs_UnimplementedUntilWired(t *testing.T) {
	s := newTestServer(&stubRequestRepository{})
	if _, err := s.GenerateSolution(t.Context(), &requestv1.GenerateSolutionRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("GenerateSolution = %v", err)
	}
	if _, err := s.ListSolutions(t.Context(), &requestv1.ListSolutionsRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("ListSolutions = %v", err)
	}
	if _, err := s.ChooseSolutionOption(t.Context(), &requestv1.ChooseSolutionOptionRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("ChooseSolutionOption = %v", err)
	}
}

func TestSolutionRPCs_RequireAnActingUser(t *testing.T) {
	s := newTestServer(&stubRequestRepository{}).WithSolution(SolutionUseCases{
		Generate: &usecase.GenerateSolution{}, Choose: &usecase.ChooseSolutionOption{},
	})
	if _, err := s.GenerateSolution(t.Context(), &requestv1.GenerateSolutionRequest{RequestId: "r"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("GenerateSolution without a user = %v", err)
	}
	if _, err := s.ChooseSolutionOption(t.Context(), &requestv1.ChooseSolutionOptionRequest{RequestId: "r"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ChooseSolutionOption without a user = %v", err)
	}
}

func TestSolutionMapping(t *testing.T) {
	two := 1
	at := time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC)
	p := toProtoSolution(domain.Solution{
		ID: "s", RequestID: "r", Kind: domain.SolutionKindDiagnosis, Status: domain.SolutionStatusSuperseded, OptionsJSON: []byte(`{"a":"é"}`),
		ChosenOption: &two, ContentRef: "c", GenerationRunID: "run", CreatedAt: at, Version: 4,
	})
	if p.GetId() != "s" || p.GetKind() != requestv1.SolutionKind_SOLUTION_KIND_DIAGNOSIS || p.GetStatus() != requestv1.SolutionStatus_SOLUTION_STATUS_SUPERSEDED ||
		p.GetOptionsJson() != `{"a":"é"}` || p.GetChosenOption() != 1 || p.GetGenerationRunId() != "run" || p.GetVersion() != 4 || !p.GetCreatedAt().AsTime().Equal(at) {
		t.Fatalf("mapped = %+v", p)
	}
	if got := toProtoSolution(domain.Solution{Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusDraft}).GetChosenOption(); got != -1 {
		t.Fatalf("unchosen = %d, the wire uses -1", got)
	}

	code, msg := "REQUEST_SOLUTION_INVALID_OUTPUT", "bad"
	fin := at.Add(time.Minute)
	run := toProtoAnalysisRun(domain.AnalysisRun{ID: "run", Kind: domain.RunKindAnswer, Status: domain.RunStatusFailed, ErrorCode: &code, ErrorMessage: &msg, StartedAt: at, FinishedAt: &fin})
	if run.GetId() != "run" || run.GetKind() != requestv1.SolutionKind_SOLUTION_KIND_ANSWER || run.GetStatus() != "failed" || run.GetErrorCode() != code ||
		run.GetErrorMessage() != msg || run.GetFinishedAt() == nil {
		t.Fatalf("run = %+v", run)
	}
	if running := toProtoAnalysisRun(domain.AnalysisRun{Status: domain.RunStatusRunning, StartedAt: at}); running.GetFinishedAt() != nil || running.GetErrorCode() != "" {
		t.Fatalf("running run = %+v", running)
	}

	for in, want := range map[requestv1.AnalysisMode]domain.AnalysisMode{
		requestv1.AnalysisMode_ANALYSIS_MODE_UNSPECIFIED:    "",
		requestv1.AnalysisMode_ANALYSIS_MODE_COMPLETE:       domain.AnalysisModeComplete,
		requestv1.AnalysisMode_ANALYSIS_MODE_AGENT_READONLY: domain.AnalysisModeAgentReadonly,
	} {
		if got := modeFromProto(in); got != want {
			t.Errorf("mode %v -> %q, want %q", in, got, want)
		}
	}
	if kindFromProto(requestv1.SolutionKind_SOLUTION_KIND_UNSPECIFIED) != "" || statusFromProto(requestv1.SolutionStatus_SOLUTION_STATUS_UNSPECIFIED) != "" {
		t.Error("unspecified filters must stay empty")
	}
	for k, pk := range kindToProto {
		if kindFromProto(pk) != k {
			t.Errorf("kind round trip broke for %s", k)
		}
	}
	for st, ps := range statusToProto {
		if statusFromProto(ps) != st {
			t.Errorf("status round trip broke for %s", st)
		}
	}
}
