package grpcclient

import (
	"encoding/json"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// A path that is not there is an answer to Stat, as it is for the local executor. The proposal workspace of the Request service
// asks for files the agent is about to create, and read a relay failure as "the change could not be checked".
func TestRelayExecutor_StatOfAMissingPathIsExistsFalse(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayByDevServerErr: status.Error(codes.NotFound, "INFRA_AGENT_PATH_NOT_FOUND: path not found on the dev server")}
	r := NewRelayExecutor(fake)

	ctx := usecase.WithDevServerID(ctxWithTenant(t), "ds-1")
	got, err := r.Stat(ctx, "/repo", "backend/missing.go")
	if err != nil || got.Exists {
		t.Fatalf("a missing path must read as exists=false: %+v %v", got, err)
	}
}

func TestRelayExecutor_StatKeepsOtherRelayFailures(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayByDevServerErr: status.Error(codes.Internal, "INFRA_AGENT_EXEC_FAILED: boom")}
	r := NewRelayExecutor(fake)

	ctx := usecase.WithDevServerID(ctxWithTenant(t), "ds-1")
	if _, err := r.Stat(ctx, "/repo", "a.go"); err == nil {
		t.Fatal("a real relay failure must still be an error")
	}
}

// The agent's git.exec refuses show-ref, which made every branch look new: a worktree for a branch that already exists (a removed
// proposal worktree being made again) was asked to create it with -b and failed.
func TestRelayExecutor_BranchExistsIsAskedWithASubcommandTheAgentAllows(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayByDevServerResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"abc\n","stderr":"","exitCode":0}`}}
	r := NewRelayExecutor(fake)

	ctx := usecase.WithDevServerID(ctxWithTenant(t), "ds-1")
	if !r.branchExistsLocally(ctx, "/repo", "request/42-proposal") {
		t.Fatal("a successful rev-parse means the branch exists")
	}
	var sent struct {
		Args []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(fake.gotRelayByDevServer.GetParamsJson()), &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Args) != 4 || sent.Args[0] != "rev-parse" || sent.Args[3] != "refs/heads/request/42-proposal" {
		t.Fatalf("args %v", sent.Args)
	}
	for _, banned := range []string{"show-ref"} {
		if sent.Args[0] == banned {
			t.Fatalf("%s is not on the agent's allowlist", banned)
		}
	}
}
