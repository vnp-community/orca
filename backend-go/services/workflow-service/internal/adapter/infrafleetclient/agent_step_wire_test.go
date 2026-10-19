package infrafleetclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/agentrelay"
	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// wireGoldenDir holds the JSON agentrelay froze from the struct workflow-service used to send.
var wireGoldenDir = filepath.Join("..", "..", "..", "..", "..", "common", "agentrelay", "testdata", "wire")

func canonicalWire(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, '\n')
}

func goldenMap(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(wireGoldenDir, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// legacyAgentParams is agentExecPromptParams as it stood before the move to agentrelay, tags unchanged.
type legacyAgentParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	AccountID    string            `json:"accountId,omitempty"`
	Model        string            `json:"model,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	InitFile     string            `json:"initFile,omitempty"`
}

type capturedRelay struct {
	req *infrafleetv1.RelayRequest
	ctx context.Context
}

func executorAnswering(result string, cap *capturedRelay, profiles usecase.ProfileResolver) *AgentExecutor {
	fake := &fakeInfraFleetClient{
		relayFunc: func(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			cap.req, cap.ctx = in, ctx
			return &infrafleetv1.RelayResponse{ResultJson: result}, nil
		},
	}
	projects := &fakeAgentProjectContextResolver{ctx: usecase.ProjectContext{ProjectName: "demo", Description: "d", RepoURL: "git@example.com:a/b.git", DevServerHostname: "dev1"}}
	return NewAgentExecutor(fake, testResolver(), &fakeProviderResolver{accountID: "acc-1"}, profiles, projects)
}

func runAgentStep(t *testing.T, exec *AgentExecutor, cfg domain.AgentStepConfig) (domain.StepResult, error) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return exec.Execute(withTenantContext(context.Background(), "tenant-1"), string(raw))
}

func TestAgentWire_BareStepMatchesGolden(t *testing.T) {
	var cap capturedRelay
	exec := executorAnswering(`{"exitCode":0}`, &cap, &fakeAgentProfileResolver{})
	cfg := domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "Review", WorktreePath: "/srv/wt", TrustPreset: "default", Model: "claude-opus"}
	if _, err := runAgentStep(t, exec, cfg); err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(goldenMap(t, "workflow_bare"))
	if got := canonicalWire(t, []byte(cap.req.GetParamsJson())); !bytes.Equal(got, canonicalWire(t, want)) {
		t.Errorf("bare step changed on the wire:\n got %s\nwant %s", got, want)
	}
}

func TestAgentWire_ProfileStepMatchesGoldenAndTheOldStruct(t *testing.T) {
	var cap capturedRelay
	profiles := &fakeAgentProfileResolver{settings: map[string]any{"agent": map[string]any{"preferredModel": "claude-opus"}}}
	exec := executorAnswering(`{"exitCode":0}`, &cap, profiles)
	cfg := domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "Review", WorktreePath: "/srv/wt", TrustPreset: "default", UserID: "u-1", ProjectID: "p-1"}
	if _, err := runAgentStep(t, exec, cfg); err != nil {
		t.Fatal(err)
	}
	sent := []byte(cap.req.GetParamsJson())

	// Keys whose values come from profile and project resolution are checked against the old struct; the rest against
	// the golden file.
	var got map[string]any
	if err := json.Unmarshal(sent, &got); err != nil {
		t.Fatal(err)
	}
	want := goldenMap(t, "workflow_profile")
	for _, k := range []string{"env", "initFile"} {
		if _, ok := got[k]; !ok {
			t.Errorf("a step with a user must send %q: %s", k, sent)
		}
		delete(got, k)
		delete(want, k)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if !bytes.Equal(canonicalWire(t, gotJSON), canonicalWire(t, wantJSON)) {
		t.Errorf("differs from golden workflow_profile:\n got %s\nwant %s", gotJSON, wantJSON)
	}

	var decoded agentrelay.ExecParams
	if err := json.Unmarshal(sent, &decoded); err != nil {
		t.Fatal(err)
	}
	old, err := json.Marshal(legacyAgentParams{
		Prompt: "Review", WorktreePath: "/srv/wt", TrustPreset: "default", AccountID: "acc-1", Model: "claude-opus",
		Env: decoded.Env, InitFile: decoded.InitFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalWire(t, sent), canonicalWire(t, old)) {
		t.Errorf("differs from what the old struct sent:\n got %s\nold %s", sent, old)
	}
	if decoded.Env["ORCA_PROJECT_ID"] != "p-1" || decoded.Env["ORCA_PROJECT_NAME"] != "demo" || decoded.InitFile == "" {
		t.Errorf("service-specific env or init file lost: %+v", decoded)
	}
}

func TestAgentWire_NoRPCDeadlineAndOneTenantEntry(t *testing.T) {
	var cap capturedRelay
	exec := executorAnswering(`{"exitCode":0}`, &cap, &fakeAgentProfileResolver{})
	if _, err := runAgentStep(t, exec, domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, has := cap.ctx.Deadline(); has {
		t.Error("a workflow agent step had no RPC deadline of ours before and must not get one now")
	}
	md, _ := metadata.FromOutgoingContext(cap.ctx)
	if got := md.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "tenant-1" {
		t.Errorf("tenant metadata = %v", got)
	}
}

func TestAgentWire_StepResultFollowsTheAgentsAnswer(t *testing.T) {
	cases := []struct {
		name, result string
		want         domain.ResultStatus
		wantExit     int
	}{
		{"exit 0", `{"stdout":"ok","exitCode":0}`, domain.ResultStatusCompleted, 0},
		{"non-zero exit", `{"stderr":"boom","exitCode":2}`, domain.ResultStatusFailed, 2},
		{"older agent without an exit code", `{"stdout":"ok"}`, domain.ResultStatusCompleted, 0},
		// The agent kills a run at its timeout and reports no exit code; that used to read as a success.
		{"timed out", `{"exitCode":null,"timedOut":true}`, domain.ResultStatusFailed, 0},
		{"output cut by the agent", `{"stdout":"tail","exitCode":0,"truncated":{"stdout":true,"stderr":false}}`, domain.ResultStatusCompleted, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cap capturedRelay
			res, err := runAgentStep(t, executorAnswering(tc.result, &cap, &fakeAgentProfileResolver{}), domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "p"})
			if err != nil {
				t.Fatal(err)
			}
			var out execResultOutput
			if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
				t.Fatal(err)
			}
			if res.Status != tc.want || out.ExitCode != tc.wantExit {
				t.Errorf("status=%v exit=%d, want %v/%d (output %s)", res.Status, out.ExitCode, tc.want, tc.wantExit, res.OutputJSON)
			}
		})
	}
}

func TestAgentWire_AnUnreadableResultIsAnError(t *testing.T) {
	var cap capturedRelay
	_, err := runAgentStep(t, executorAnswering(`not json`, &cap, &fakeAgentProfileResolver{}), domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "p"})
	if !errors.Is(err, agentrelay.ErrMalformedResult) {
		t.Fatalf("err = %v", err)
	}
}
