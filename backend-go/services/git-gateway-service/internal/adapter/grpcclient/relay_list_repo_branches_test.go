package grpcclient

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func relayBranchesFake(t *testing.T, stdout string, exitCode int, stderr string) *fakeInfraFleetServiceClient {
	t.Helper()
	resultJSON, err := json.Marshal(map[string]any{"stdout": stdout, "stderr": stderr, "exitCode": exitCode})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)}}
}

func TestRelayExecutor_ListRepoBranches_SendsAllowlistedBranchArgv(t *testing.T) {
	fake := relayBranchesFake(t, "refs/heads/main\nrefs/heads/feature/other\nrefs/remotes/origin/HEAD\nrefs/remotes/origin/main\nrefs/remotes/origin/hotfix/x\n", 0, "")

	branches, truncated, err := NewRelayExecutor(fake).ListRepoBranches(ctxWithTenant(t), "/srv/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []domain.RepoBranch{{Name: "feature/other"}, {Name: "main"}, {Name: "origin/hotfix/x", Remote: true}}
	if truncated || !reflect.DeepEqual(branches, want) {
		t.Errorf("got %+v truncated=%v, want %+v", branches, truncated, want)
	}

	if fake.gotRelay.GetMethod() != "git.exec" {
		t.Errorf("method = %q, want git.exec", fake.gotRelay.GetMethod())
	}
	var params struct {
		Args []string `json:"args"`
		Cwd  string   `json:"cwd"`
	}
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if !reflect.DeepEqual(params.Args, []string{"branch", "--list", "--all", "--format=%(refname)"}) || params.Cwd != "/srv/repo" {
		t.Errorf("unexpected params: %+v", params)
	}
}

func TestRelayExecutor_ListRepoBranches_GitFailureIsAnError(t *testing.T) {
	fake := relayBranchesFake(t, "", 128, "fatal: not a git repository")
	if _, _, err := NewRelayExecutor(fake).ListRepoBranches(ctxWithTenant(t), "/srv/repo"); err == nil {
		t.Fatal("expected an error when git exits non-zero")
	}
}

func TestRelayExecutor_ListRepoBranches_RelayErrorPassesThrough(t *testing.T) {
	wantErr := errors.New("relay down")
	fake := &fakeInfraFleetServiceClient{relayErr: wantErr}
	if _, _, err := NewRelayExecutor(fake).ListRepoBranches(ctxWithTenant(t), "/srv/repo"); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
