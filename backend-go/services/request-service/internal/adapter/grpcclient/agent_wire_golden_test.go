package grpcclient

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// wireGoldenDir holds the JSON agentrelay froze from the structs request-service used to send. Reading it from here
// proves the migration did not change what the agent receives.
var wireGoldenDir = filepath.Join("..", "..", "..", "..", "..", "common", "agentrelay", "testdata", "wire")

// canonicalWire re-encodes with sorted keys and the indentation the golden files use.
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

func assertMatchesGolden(t *testing.T, name string, sent string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join(wireGoldenDir, name+".json"))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if got := canonicalWire(t, []byte(sent)); !bytes.Equal(got, want) {
		t.Errorf("%s: JSON sent to the agent changed:\n got %s\nwant %s", name, got, want)
	}
}

func TestAgentPromptRelay_SendsTheJSONRequestServiceAlwaysSent(t *testing.T) {
	base := usecase.AgentPromptInput{StepID: "step-1", Prompt: "Analyse the repo", RepoPath: "/srv/repo", RequestID: "req-1", ProjectID: "proj-1", TimeoutMS: 600000}
	with := func(edit func(*usecase.AgentPromptInput)) usecase.AgentPromptInput {
		in := base
		edit(&in)
		return in
	}
	cases := map[string]usecase.AgentPromptInput{
		"request_readonly": with(func(in *usecase.AgentPromptInput) { in.ReadOnlyEnforced = true }),
		"request_readonly_knowledge": with(func(in *usecase.AgentPromptInput) {
			in.ReadOnlyEnforced, in.KnowledgeRoot, in.ProposeChangesets = true, "/srv/repo", true
		}),
		"request_write_profile": with(func(in *usecase.AgentPromptInput) {
			in.WriteProfile, in.RepoPath = "spec-author", "/srv/wt/req-1"
		}),
		"request_write_profile_knowledge": with(func(in *usecase.AgentPromptInput) {
			in.WriteProfile, in.RepoPath, in.KnowledgeRoot = "spec-author", "/srv/wt/req-1", "/srv/repo"
		}),
		"request_contract": with(func(in *usecase.AgentPromptInput) { in.ResultNonce = "AbCdEfGh01234567" }),
		"request_plain":    with(func(in *usecase.AgentPromptInput) { in.TimeoutMS = 0 }),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			infra := &fakeAgentInfra{result: `{"exitCode":0}`}
			if _, err := NewAgentPromptRelay(infra).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"}, in); err != nil {
				t.Fatal(err)
			}
			if len(infra.relay) != 1 || infra.relay[0].GetMethod() != "agent.execPrompt" {
				t.Fatalf("calls = %+v", infra.relay)
			}
			assertMatchesGolden(t, name, infra.relay[0].GetParamsJson())
		})
	}
}

func TestAICompletionRelay_SendsTheJSONRequestServiceAlwaysSent(t *testing.T) {
	cases := map[string]struct{ accountID, prompt string }{
		"complete_request_account": {"acc-7", "Classify this"},
		"complete_request_plain":   {"", "Write a solution"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			infra := &fakeInfra{replies: []string{"ok"}}
			got, err := relayAIComplete(ctxWithIdentity(), newTenantRelayer(infra), AIConnection{ConnectionID: "c"}, tc.accountID, tc.prompt, time.Second)
			if err != nil || got != "ok" {
				t.Fatalf("got %q err %v", got, err)
			}
			if len(infra.relayCalls) != 1 || infra.relayCalls[0].GetMethod() != "ai.complete" {
				t.Fatalf("calls = %+v", infra.relayCalls)
			}
			assertMatchesGolden(t, name, infra.relayCalls[0].GetParamsJson())
		})
	}
}
