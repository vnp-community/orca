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
