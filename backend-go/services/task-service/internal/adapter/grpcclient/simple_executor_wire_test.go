package grpcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/agentrelay"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// wireGoldenDir holds the JSON agentrelay froze from the structs task-service used to send, so reading it here proves the
// migration did not change what the agent receives.
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

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(wireGoldenDir, name+".json"))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return b
}

// legacyTaskParams is agentExecPromptParams as it stood before the move to agentrelay, tags unchanged. It is the
// reference for flows the golden files cannot reproduce through the real executor (the env and init file come from
// profile and project resolution).
type legacyTaskParams struct {
	Prompt         string             `json:"prompt"`
	WorktreePath   string             `json:"worktreePath"`
	StepID         string             `json:"stepId,omitempty"`
	TrustPreset    string             `json:"trustPreset,omitempty"`
	Model          string             `json:"model,omitempty"`
	Env            map[string]string  `json:"env,omitempty"`
	InitFile       string             `json:"initFile,omitempty"`
	ResultBlock    *legacyResultBlock `json:"resultBlock,omitempty"`
	ReportChanges  bool               `json:"reportChanges,omitempty"`
	MaxOutputBytes int                `json:"maxOutputBytes,omitempty"`
	TimeoutMs      int                `json:"timeoutMs,omitempty"`
	KnowledgeRoot  string             `json:"knowledgeRoot,omitempty"`
}

type legacyResultBlock struct {
	Nonce string `json:"nonce"`
}

// legacyContract is contractRunOptions.apply and its constants, before the move.
func (p *legacyTaskParams) legacyContract(nonce string) {
	p.ResultBlock = &legacyResultBlock{Nonce: nonce}
	p.ReportChanges = true
	p.MaxOutputBytes = 1 << 20
	p.TimeoutMs = 25 * 60 * 1000
}

const wireResult = `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`

type wireRun struct {
	relay *fakeInfraFleetServiceClient
	exec  *SimpleExecutor
	ctx   context.Context
}

// newWireRun wires task t-1 of project p-1; withUser adds a signed-in user with a profile, a preferred model and a
// project context, which is what turns on model, env and initFile.
func newWireRun(t *testing.T, withUser bool) wireRun {
	t.Helper()
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t-1": {ID: "t-1", ProjectID: "p-1", Title: "T"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/repo", connected: true}
	relay := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: wireResult}}
	profiles := &fakeSimpleExecutorProfileResolver{settings: map[string]any{"agent": map[string]any{"preferredModel": "claude-sonnet"}}}
	projects := &fakeSimpleExecutorProjectContextResolver{ctx: usecase.ProjectContext{ProjectName: "demo", Description: "d", RepoURL: "git@example.com:a/b.git", DevServerHostname: "dev1"}}
	ctx := ctxWithTenant(t)
	if withUser {
		ctx = tenant.WithUserID(ctx, "u-1")
	}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, profiles, projects, &fakeOutboxWriter{}, &fakeAgentExecOutputStreamer{}).WithExecutionRecords(&memoryRecords{})
	return wireRun{relay: relay, exec: exec, ctx: ctx}
}

func (w wireRun) sent() []byte { return []byte(w.relay.gotRelay.GetParamsJson()) }

func TestTaskWire_PlainRunMatchesGolden(t *testing.T) {
	w := newWireRun(t, false)
	if _, err := w.exec.Execute(w.ctx, "tenant-1", "t-1", "req:1", "/srv/wt", "Do task"); err != nil {
		t.Fatal(err)
	}
	// The golden has two env keys and no model or init file: exactly what a run with no signed-in user sends.
	if got, want := canonicalWire(t, w.sent()), readGolden(t, "task_plain"); !bytes.Equal(got, want) {
		t.Errorf("plain run changed on the wire:\n got %s\nwant %s", got, want)
	}
}

// goldenWithoutProfile drops the three keys whose values come from profile and project resolution, which the golden
// files fill with fixed sample data.
func goldenWithoutProfile(t *testing.T, name string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(readGolden(t, name), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"env", "initFile", "model"} {
		delete(m, k)
	}
	return m
}

func withoutProfile(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"env", "initFile", "model"} {
		if _, ok := m[k]; !ok {
			t.Errorf("a signed-in run must send %q: %s", k, raw)
		}
		delete(m, k)
	}
	return m
}

func TestTaskWire_ProfileAndContractRunsMatchGoldenAndTheOldStruct(t *testing.T) {
	cases := []struct {
		golden   string
		contract bool
		hub      bool
	}{
		{"task_profile", false, false},
		{"task_contract", true, false},
		{"task_contract_knowledge", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			w := newWireRun(t, true)
			var err error
			if tc.contract {
				_, err = w.exec.ExecuteWithContract(w.ctx, usecase.ContractExecuteInput{
					TenantID: "tenant-1", TaskID: "t-1", RequestID: "req:1", WorktreePath: "/srv/wt", Prompt: "Do task",
					ResultNonce: "Nonce0123456789ab", UseKnowledgeHub: tc.hub, Attempt: 1,
				})
				var failure *usecase.ExecutionFailure
				if !errors.As(err, &failure) {
					t.Fatalf("the stub answer has no result block, so the run must fail as a defect: %v", err)
				}
			} else {
				_, err = w.exec.Execute(w.ctx, "tenant-1", "t-1", "req:1", "/srv/wt", "Do task")
				if err != nil {
					t.Fatal(err)
				}
			}
			sent := w.sent()

			// 1. Every key of the golden that does not depend on resolution is identical.
			gotShape, _ := json.Marshal(withoutProfile(t, sent))
			wantShape, _ := json.Marshal(goldenWithoutProfile(t, tc.golden))
			if !bytes.Equal(canonicalWire(t, gotShape), canonicalWire(t, wantShape)) {
				t.Errorf("wire shape differs from golden %s:\n got %s\nwant %s", tc.golden, gotShape, wantShape)
			}

			// 2. The old struct filled with the same resolved values serialises to the same document.
			var got agentrelay.ExecParams
			if err := json.Unmarshal(sent, &got); err != nil {
				t.Fatal(err)
			}
			old := legacyTaskParams{
				Prompt: "Do task", WorktreePath: "/srv/wt", StepID: "req:1", TrustPreset: "full",
				Model: got.Model, Env: got.Env, InitFile: got.InitFile,
			}
			if tc.contract {
				old.legacyContract("Nonce0123456789ab")
			}
			if tc.hub {
				old.KnowledgeRoot = "/srv/repo"
			}
			oldJSON, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canonicalWire(t, sent), canonicalWire(t, oldJSON)) {
				t.Errorf("differs from what the old struct sent:\n got %s\nold %s", sent, oldJSON)
			}
			if got.Env["ORCA_TASK_ID"] != "t-1" || got.Env["ORCA_PROJECT_ID"] != "p-1" || got.Env["ORCA_PROJECT_NAME"] != "demo" {
				t.Errorf("service-specific env lost: %v", got.Env)
			}
		})
	}
}

// A task run is bounded by the agent's own timeout and the caller's context, never by an RPC deadline of ours: a contract
// run asks for 25 minutes, and a derived deadline would cut a healthy run that the agent still reports as running.
func TestTaskWire_TheRelayCallCarriesNoDeadlineAndOneTenantEntry(t *testing.T) {
	w := newWireRun(t, false)
	_, err := w.exec.ExecuteWithContract(w.ctx, usecase.ContractExecuteInput{
		TenantID: "tenant-1", TaskID: "t-1", RequestID: "req:1", WorktreePath: "/srv/wt", Prompt: "p", ResultNonce: "Nonce0123456789ab",
	})
	var failure *usecase.ExecutionFailure
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v", err)
	}
	if _, has := w.relay.gotRelayCtx.Deadline(); has {
		t.Error("a task run must not get an RPC deadline from its timeoutMs")
	}
	md, _ := metadata.FromOutgoingContext(w.relay.gotRelayCtx)
	if got := md.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "tenant-1" {
		t.Errorf("tenant metadata = %v, want exactly one entry (the live-output stream shares this context)", got)
	}
}

func TestTaskWire_ADevServerWithoutAConnectionIsReachedByItsID(t *testing.T) {
	w := newWireRun(t, false)
	w.relay.relayByDevServerResp = &infrafleetv1.RelayResponse{ResultJson: wireResult}
	w.exec.resolver = &fakeProjectExecutionResolver{connectionID: "", devServerID: "ds-1", worktreePath: "/srv/repo", connected: true}
	if _, err := w.exec.Execute(w.ctx, "tenant-1", "t-1", "req:1", "/srv/wt", "Do task"); err != nil {
		t.Fatal(err)
	}
	if w.relay.gotRelay != nil || w.relay.gotRelayByDevServer.GetDevServerId() != "ds-1" || w.relay.gotRelayByDevServer.GetMethod() != "agent.execPrompt" {
		t.Fatalf("relay=%v byDev=%v", w.relay.gotRelay, w.relay.gotRelayByDevServer)
	}
}

// The agent reports which streams it cut as an object; task-service used to type it as a bool, so any run whose output
// hit the budget failed to decode and was reported as a failed task.
func TestTaskWire_ATruncatedRunDecodes(t *testing.T) {
	const truncated = `{"stdout":"tail","stderr":"","exitCode":0,"timedOut":false,"truncated":{"stdout":true,"stderr":false}}`
	var old struct {
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(truncated), &old); err == nil {
		t.Fatal("the old bool field was expected to fail on the agent's object; the premise of this test is gone")
	}

	w := newWireRun(t, false)
	w.relay.relayResp = &infrafleetv1.RelayResponse{ResultJson: truncated}
	if _, err := w.exec.Execute(w.ctx, "tenant-1", "t-1", "req:1", "/srv/wt", "Do task"); err != nil {
		t.Fatalf("a run with truncated output is still a successful run: %v", err)
	}
	w = newWireRun(t, false)
	w.relay.relayResp = &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","exitCode":0,"truncated":{"stdout":true,"stderr":true}}`}
	_, err := w.exec.ExecuteWithContract(w.ctx, usecase.ContractExecuteInput{
		TenantID: "tenant-1", TaskID: "t-1", RequestID: "req:1", WorktreePath: "/srv/wt", Prompt: "p", ResultNonce: "Nonce0123456789ab",
	})
	if failureOf(t, err).Class != domain.FailureAgentDefect {
		t.Fatalf("a cut run without its result block is an agent defect, not a decode error: %v", err)
	}
}

// A result the agent sent but we cannot read is a protocol fault, not a relay outage: it must not be retried as one.
func TestTaskWire_AMalformedResultIsNotARelayFailure(t *testing.T) {
	w := newWireRun(t, false)
	w.relay.relayResp = &infrafleetv1.RelayResponse{ResultJson: `not json`}
	_, err := w.exec.Execute(w.ctx, "tenant-1", "t-1", "req:1", "/srv/wt", "Do task")
	var relayErr *relayError
	if err == nil || errors.As(err, &relayErr) || !errors.Is(err, agentrelay.ErrMalformedResult) {
		t.Fatalf("err = %v", err)
	}
}

func TestTaskWire_ADeadlineFromTheRelayIsTransient(t *testing.T) {
	w := newWireRun(t, false)
	w.relay.relayErr = context.DeadlineExceeded
	_, err := w.exec.Execute(w.ctx, "tenant-1", "t-1", "req:1", "/srv/wt", "Do task")
	var relayErr *relayError
	if !errors.As(err, &relayErr) || !relayErr.Transient() {
		t.Fatalf("err = %v", err)
	}
}

func TestAICompleterWire_SendsOnlyThePrompt(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"content":"hello","model":"m","usage":{"totalTokens":3}}`}}
	got, err := NewAICompleter(fake).Complete(ctxWithTenant(t), "conn-1", "Write a commit message")
	if err != nil || got != "hello" {
		t.Fatalf("got %q err %v", got, err)
	}
	if want := readGolden(t, "complete_git"); !bytes.Equal(canonicalWire(t, []byte(fake.gotRelay.GetParamsJson())), want) {
		t.Errorf("ai.complete params changed:\n got %s\nwant %s", fake.gotRelay.GetParamsJson(), want)
	}
	if fake.gotRelay.GetConnectionId() != "conn-1" {
		t.Errorf("connection = %q", fake.gotRelay.GetConnectionId())
	}
	if _, has := fake.gotRelayCtx.Deadline(); has {
		t.Error("ai.complete had no deadline of ours before and must not get one now")
	}
	md, _ := metadata.FromOutgoingContext(fake.gotRelayCtx)
	if got := md.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "tenant-1" {
		t.Errorf("tenant metadata = %v", got)
	}
}
