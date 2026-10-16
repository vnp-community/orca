package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// A Request that has not entered executing shows the mode it will run with, so the approval can be described before it happens.
func TestRequestEngineResolver_PreviewsTheExecutionModeOfAnUnpinnedRequest(t *testing.T) {
	settings := staticEngineSettings{&domain.ProjectEngineSettings{ExecutionMode: domain.ExecutionContinuous}}
	reqs := []domain.Request{
		{ID: "a", ProjectID: "p", Type: domain.RequestTypeBug},
		{ID: "b", ProjectID: "p", Type: domain.RequestTypeBug, ExecutionMode: domain.ExecutionStepByStep},
	}
	(&RequestEngineResolver{Settings: settings}).Annotate(context.Background(), reqs)
	if reqs[0].ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("unpinned: got %q, want the project setting", reqs[0].ExecutionMode)
	}
	if reqs[1].ExecutionMode != domain.ExecutionStepByStep {
		t.Fatalf("pinned: got %q, want the pinned mode to win", reqs[1].ExecutionMode)
	}
}

func TestRequestEngineResolver_FallsBackToTheDeploymentDefault(t *testing.T) {
	reqs := []domain.Request{{ID: "a", ProjectID: "p", Type: domain.RequestTypeTask}}
	r := &RequestEngineResolver{Defaults: domain.EngineDefaults{ExecutionMode: domain.ExecutionContinuous}}
	r.Annotate(context.Background(), reqs)
	if reqs[0].ExecutionMode != domain.ExecutionContinuous || reqs[0].Engine != domain.EngineNative {
		t.Fatalf("got mode %q engine %q", reqs[0].ExecutionMode, reqs[0].Engine)
	}
}
