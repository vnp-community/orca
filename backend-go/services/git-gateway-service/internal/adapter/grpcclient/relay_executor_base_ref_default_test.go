package grpcclient

import (
	"strings"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

func baseRefDefaultExecutor(resultJSON string) (*RelayExecutor, *fakeInfraFleetServiceClient) {
	fake := &fakeInfraFleetServiceClient{relayRespFunc: func(*infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
		return &infrafleetv1.RelayResponse{ResultJson: resultJSON}, nil
	}}
	return NewRelayExecutor(fake), fake
}

func TestRelayExecutor_BaseRefDefault_ForwardsPreferredBranchToTheAgent(t *testing.T) {
	r, fake := baseRefDefaultExecutor(`{"ref":"release/1.x"}`)
	ref, err := r.BaseRefDefault(ctxWithTenant(t), "/repo", "main")
	if err != nil || ref != "release/1.x" {
		t.Fatalf("ref=%q err=%v", ref, err)
	}
	req := fake.gotRelayRequests[0]
	if req.GetMethod() != "git.baseRefDefault" {
		t.Errorf("method %q", req.GetMethod())
	}
	if !strings.Contains(req.GetParamsJson(), `"preferredBranch":"main"`) {
		t.Errorf("preferredBranch must reach the agent: %s", req.GetParamsJson())
	}
}

func TestRelayExecutor_BaseRefDefault_NoPreferredBranch_KeepsTheOldWireShape(t *testing.T) {
	r, fake := baseRefDefaultExecutor(`{"ref":"main"}`)
	if _, err := r.BaseRefDefault(ctxWithTenant(t), "/repo", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fake.gotRelayRequests[0].GetParamsJson(), "preferredBranch") {
		t.Errorf("an agent that predates the field must see the old request: %s", fake.gotRelayRequests[0].GetParamsJson())
	}
}
