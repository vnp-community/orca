package main

import (
	"testing"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func TestPlanAutoUnavailable(t *testing.T) {
	cases := []struct {
		name                           string
		env, flow, bus, commit, wantOn bool
	}{
		{"all on", true, true, true, true, true},
		{"deployment switch off", false, true, true, true, false},
		{"request flow disabled", true, false, true, true, false},
		{"no event bus", true, true, false, true, false},
		{"approval off so no CommitPlan", true, true, true, false, false},
	}
	for _, c := range cases {
		reason := planAutoUnavailable(c.env, c.flow, c.bus, c.commit)
		if (reason == "") != c.wantOn {
			t.Errorf("%s: reason = %q", c.name, reason)
		}
	}
}

func planAutoTestWiring() (requestgrpc.PlanUseCases, *requestStores, *solutionWiring, *approvalDirectories) {
	plan := requestgrpc.PlanUseCases{Generate: &usecase.GeneratePlan{}, Get: usecase.NewGetPlanProposal(nil, nil), Commit: &usecase.CommitPlan{}}
	dirs := &approvalDirectories{teams: grpcclient.UnavailableDirectory{}, admins: grpcclient.UnavailableDirectory{}}
	return plan, &requestStores{planAutoJobs: jobsStub{}}, &solutionWiring{owner: "host-1"}, dirs
}

// jobsStub only has to exist: building the wiring never calls the job store.
type jobsStub struct{ usecase.PlanAutoJobRepository }

func TestBuildPlanAuto_BuildsEverythingWhenAllConditionsHold(t *testing.T) {
	plan, stores, solution, dirs := planAutoTestWiring()
	parts, reason := buildPlanAuto(config.Config{RequestFlowEnabled: true}, stores, plan, solution, dirs, &commoneventbus.Consumer{}, domain.DefaultPlanAutoPolicy, true)
	if reason != "" || parts.consumer == nil || parts.worker == nil || parts.plan.Retry == nil {
		t.Fatalf("reason=%q parts=%+v", reason, parts)
	}
	if !parts.plan.Generate.CommitsRevisions() {
		t.Fatal("with the worker running, a change request must queue the job that commits it (and approvers may ask for one)")
	}
}

func TestBuildPlanAuto_NothingIsBuiltWhenAConditionIsMissing(t *testing.T) {
	cases := map[string]struct {
		cfg      config.Config
		bus      *commoneventbus.Consumer
		env      bool
		noCommit bool
	}{
		"env off":       {config.Config{RequestFlowEnabled: true}, &commoneventbus.Consumer{}, false, false},
		"flow off":      {config.Config{}, &commoneventbus.Consumer{}, true, false},
		"no bus":        {config.Config{RequestFlowEnabled: true}, nil, true, false},
		"no CommitPlan": {config.Config{RequestFlowEnabled: true}, &commoneventbus.Consumer{}, true, true},
	}
	for name, c := range cases {
		plan, stores, solution, dirs := planAutoTestWiring()
		if c.noCommit {
			plan.Commit = nil
		}
		parts, reason := buildPlanAuto(c.cfg, stores, plan, solution, dirs, c.bus, domain.DefaultPlanAutoPolicy, c.env)
		if reason == "" || parts.consumer != nil || parts.worker != nil || parts.plan.Retry != nil {
			t.Errorf("%s: reason=%q parts=%+v", name, reason, parts)
		}
		if parts.plan.Get == nil {
			t.Errorf("%s: the Plan read must stay available", name)
		}
		if parts.plan.Generate.CommitsRevisions() {
			t.Errorf("%s: without the worker nobody commits a revision, so none may be queued and approvers may not revise", name)
		}
	}
}
