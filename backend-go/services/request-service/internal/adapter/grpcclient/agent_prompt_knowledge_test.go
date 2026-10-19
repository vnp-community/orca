package grpcclient

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func paramsJSON(t *testing.T, in usecase.AgentPromptInput) map[string]any {
	t.Helper()
	raw, err := json.Marshal(buildAgentParams(in))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildAgentParams_SendsNoKnowledgeKeyWithoutARoot(t *testing.T) {
	for _, ro := range []bool{false, true} {
		m := paramsJSON(t, usecase.AgentPromptInput{Prompt: "p", RepoPath: "/r", ReadOnlyEnforced: ro, ProposeChangesets: true})
		if _, ok := m["knowledgeRoot"]; ok {
			t.Fatalf("ro=%v: knowledgeRoot must be absent without a root: %v", ro, m)
		}
		if _, ok := m["proposeChangesets"]; ok {
			t.Fatalf("ro=%v: proposeChangesets must not travel without a root (the agent rejects it): %v", ro, m)
		}
	}
}

func TestBuildAgentParams_SendsTheRootInBothModes(t *testing.T) {
	for _, ro := range []bool{false, true} {
		m := paramsJSON(t, usecase.AgentPromptInput{Prompt: "p", RepoPath: "/r", ReadOnlyEnforced: ro, KnowledgeRoot: "/proj"})
		if m["knowledgeRoot"] != "/proj" {
			t.Fatalf("ro=%v: %v", ro, m)
		}
		if _, ok := m["proposeChangesets"]; ok {
			t.Fatalf("ro=%v: proposeChangesets defaults to absent: %v", ro, m)
		}
	}
	m := paramsJSON(t, usecase.AgentPromptInput{Prompt: "p", KnowledgeRoot: "/proj", ProposeChangesets: true})
	if m["proposeChangesets"] != true {
		t.Fatalf("%v", m)
	}
}

func TestBuildAgentParams_KnowledgeRootNeverRaisesTrust(t *testing.T) {
	m := paramsJSON(t, usecase.AgentPromptInput{Prompt: "p", ReadOnlyEnforced: true, KnowledgeRoot: "/proj", ProposeChangesets: true})
	if m["accessMode"] != "readonly" {
		t.Fatalf("accessMode: %v", m)
	}
	if _, ok := m["trustPreset"]; ok {
		t.Fatalf("readonly must not carry a trust preset: %v", m)
	}
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), `"full"`) {
		t.Fatalf("full trust: %s", raw)
	}
}

func TestClassifyAgentRelayError_KnowledgeHub(t *testing.T) {
	for _, reason := range []string{"KNOWLEDGE_HUB_UNSUPPORTED", "KNOWLEDGE_ROOT_NOT_ABSOLUTE", "KNOWLEDGE_ROOT_MISSING", "KNOWLEDGE_CONFIG_MISSING"} {
		err := classifyAgentRelayError(status.Error(codes.InvalidArgument, "agent error: agent.execPrompt: refused ("+reason+")"))
		if !errors.Is(err, usecase.ErrAgentKnowledgeHubUnavailable) {
			t.Fatalf("%s: %v", reason, err)
		}
		if errors.Is(err, usecase.ErrAgentReadonlyUnsupported) {
			t.Fatalf("%s must not read as a readonly refusal", reason)
		}
	}
	if errors.Is(classifyAgentRelayError(errors.New("boom")), usecase.ErrAgentKnowledgeHubUnavailable) {
		t.Fatal("an unrelated error must stay unrelated")
	}
}

func TestBuildAgentParams_WriteProfileRunsInAWorktreeWithoutFullTrust(t *testing.T) {
	m := paramsJSON(t, usecase.AgentPromptInput{Prompt: "p", RepoPath: "/wt", WriteProfile: "openspec-author", ResultNonce: "abcdefghijklmnop", KnowledgeRoot: "/proj"})
	if m["accessMode"] != "write" || m["workspaceKind"] != "worktree" || m["reportChanges"] != true || m["permissionProfile"] != "openspec-author" {
		t.Fatalf("%v", m)
	}
	if m["trustPreset"] != "default" {
		t.Fatalf("the profile run must never ask for full trust: %v", m)
	}
	rb, _ := m["resultBlock"].(map[string]any)
	if rb["nonce"] != "abcdefghijklmnop" || m["knowledgeRoot"] != "/proj" || m["maxOutputBytes"] == nil {
		t.Fatalf("%v", m)
	}
}

func TestBuildAgentParams_NoProfileNoResultBlockMeansNoSuchKeys(t *testing.T) {
	m := paramsJSON(t, usecase.AgentPromptInput{Prompt: "p"})
	for _, k := range []string{"permissionProfile", "resultBlock", "accessMode", "workspaceKind"} {
		if _, ok := m[k]; ok {
			t.Fatalf("%s must be absent for an ordinary run: %v", k, m)
		}
	}
}

func TestAgentPromptRelay_ReadOnlyAndWriteProfileCannotBeCombined(t *testing.T) {
	infra := &fakeAgentInfra{result: `{"exitCode":0}`}
	_, err := NewAgentPromptRelay(infra).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"},
		usecase.AgentPromptInput{Prompt: "p", ReadOnlyEnforced: true, WriteProfile: "openspec-author"})
	if err == nil || len(infra.relay) != 0 {
		t.Fatalf("must refuse before any call: %v", err)
	}
}

func TestParseAgentResult_ResultBlock(t *testing.T) {
	ok, err := parseAgentResult(`{"exitCode":0,"parsed":{"ok":true,"value":{"changeId":"req-1","phases":2}}}`)
	if err != nil || string(ok.ResultBlock) != `{"changeId":"req-1","phases":2}` || ok.ResultBlockCode != "" {
		t.Fatalf("%+v %v", ok, err)
	}
	bad, err := parseAgentResult(`{"exitCode":0,"parsed":{"ok":false,"code":"RESULT_BLOCK_MISSING"}}`)
	if err != nil || len(bad.ResultBlock) != 0 || bad.ResultBlockCode != "RESULT_BLOCK_MISSING" {
		t.Fatalf("%+v %v", bad, err)
	}
	none, _ := parseAgentResult(`{"exitCode":0}`)
	if len(none.ResultBlock) != 0 || none.ResultBlockCode != "" {
		t.Fatalf("an agent that sent no parsed field leaves both empty: %+v", none)
	}
}
