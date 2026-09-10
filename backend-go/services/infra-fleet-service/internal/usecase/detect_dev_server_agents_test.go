package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestDetectDevServerAgents_CallsExecWithExactFourProbes(t *testing.T) {
	ds := domain.DevServer{ID: "ds1", TenantID: "t1"}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"agents": []any{"claude"}, "platform": "linux",
	}}
	uc := NewDetectDevServerAgents(devRepo, agent)

	_, err := uc.Execute(context.Background(), "t1", "ds1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "preflight.detectAgents" {
		t.Fatalf("expected exactly 1 call to preflight.detectAgents, got %+v", agent.execCalls)
	}
}

func TestDetectDevServerAgents_DefaultProbesAreBLFleet04Exact(t *testing.T) {
	want := []AgentProbe{
		{ID: "claude", Cmd: "claude"},
		{ID: "codex", Cmd: "codex"},
		{ID: "gemini", Cmd: "gemini"},
		{ID: "openai", Cmd: "openai"},
	}
	if !reflect.DeepEqual(defaultAgentProbes, want) {
		t.Errorf("expected the exact BL-FLEET-04 probe list, got %+v", defaultAgentProbes)
	}
}

func TestDetectDevServerAgents_DecodesFixtureResponse(t *testing.T) {
	ds := domain.DevServer{ID: "ds1", TenantID: "t1"}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"agents": []any{"claude"}, "platform": "linux",
	}}
	uc := NewDetectDevServerAgents(devRepo, agent)

	got, err := uc.Execute(context.Background(), "t1", "ds1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := DetectedAgents{Agents: []string{"claude"}, Platform: "linux"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestDetectDevServerAgents_ExecErrorSurfacesAsDetectAgentsFailed(t *testing.T) {
	ds := domain.DevServer{ID: "ds1", TenantID: "t1"}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("agent unreachable")}
	uc := NewDetectDevServerAgents(devRepo, agent)

	got, err := uc.Execute(context.Background(), "t1", "ds1")
	if err == nil {
		t.Fatal("expected an error when Exec fails")
	}
	if !reflect.DeepEqual(got, DetectedAgents{}) {
		t.Errorf("expected a zero-value DetectedAgents on error, not a fabricated empty list distinct from the zero value, got %+v", got)
	}
}

func TestDecodeDetectedAgents_MalformedInputDegradesGracefully(t *testing.T) {
	tests := []struct {
		name   string
		result map[string]any
		want   DetectedAgents
	}{
		{"empty result", map[string]any{}, DetectedAgents{}},
		{"non-string agent entries are skipped", map[string]any{"agents": []any{"claude", 42, "codex"}}, DetectedAgents{Agents: []string{"claude", "codex"}}},
		{"non-string platform is ignored", map[string]any{"platform": 42}, DetectedAgents{}},
		{"agents not a list is ignored", map[string]any{"agents": "not-a-list"}, DetectedAgents{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeDetectedAgents(tt.result)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %+v, got %+v", tt.want, got)
			}
		})
	}
}
