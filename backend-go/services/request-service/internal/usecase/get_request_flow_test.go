package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestGetRequestFlow_AllTypes(t *testing.T) {
	uc := NewGetRequestFlow()
	for _, typ := range domain.AllRequestTypes() {
		v, err := uc.Execute(lcCtx(), string(typ), "")
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if v.Flow.Type != typ || v.StatusPath[0] != domain.RequestStatusNew || v.StatusPath[len(v.StatusPath)-1] != domain.RequestStatusCompleted {
			t.Errorf("%s: %+v", typ, v)
		}
	}
	spike, _ := uc.Execute(lcCtx(), "spike", "")
	if spike.Flow.AnalysisKind != domain.AnalysisFindings || spike.Flow.AnalysisGate != domain.GateFindings || !spike.Flow.CompletesAfterAnalysis {
		t.Errorf("spike %+v", spike.Flow)
	}
	ops, _ := uc.Execute(lcCtx(), "ops_request", "")
	if ops.Flow.StartGate != domain.GatePlan || len(ops.Flow.ExecutionGates) != 1 || ops.Flow.ExecutionGates[0] != domain.GatePreDeploy {
		t.Errorf("ops %+v", ops.Flow)
	}
}

func TestGetRequestFlow_PhasesBySize(t *testing.T) {
	uc := NewGetRequestFlow()
	cases := []struct {
		typ, size string
		want      bool
	}{{"bug", "L", true}, {"bug", "S", false}, {"bug", "", false}, {"change_request", "S", true}, {"change_request", "", true}, {"hotfix", "L", false}}
	for _, c := range cases {
		v, err := uc.Execute(lcCtx(), c.typ, c.size)
		if err != nil || v.HasPhases != c.want {
			t.Errorf("%s/%s: %v %v", c.typ, c.size, v.HasPhases, err)
		}
	}
}

func TestGetRequestFlow_InvalidInputAndTenant(t *testing.T) {
	uc := NewGetRequestFlow()
	if _, err := uc.Execute(lcCtx(), "feature", ""); codeOf(err) != "REQUEST_INVALID_TYPE" {
		t.Errorf("type: %v", err)
	}
	if _, err := uc.Execute(lcCtx(), "bug", "XL"); codeOf(err) != "REQUEST_INVALID_SIZE" {
		t.Errorf("size: %v", err)
	}
	if _, err := uc.Execute(context.Background(), "bug", ""); codeOf(err) != "REQUEST_TENANT_REQUIRED" {
		t.Errorf("tenant: %v", err)
	}
}
