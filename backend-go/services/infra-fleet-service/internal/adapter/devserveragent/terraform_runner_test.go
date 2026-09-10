package devserveragent

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeAgentClient implements just enough of usecase.DevServerAgentClient for
// TerraformRunner's tests — Exec is the only method TerraformRunner calls.
type fakeAgentClient struct {
	execResult   map[string]any
	execErr      error
	calledMethod string
	calledParams map[string]any
}

func (f *fakeAgentClient) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	f.calledMethod = method
	f.calledParams = params
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execResult, nil
}
func (f *fakeAgentClient) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	return true, nil
}
func (f *fakeAgentClient) IsConnected(devServerID string) bool { return true }

func TestTerraformRunner_Apply_CallsExecWithTerraformApplyMethod(t *testing.T) {
	agent := &fakeAgentClient{execResult: map[string]any{"outputJson": `{"instance_hosts":{"value":[]}}`}}
	runner := NewTerraformRunner(agent)

	_, err := runner.Apply(context.Background(), domain.DevServer{ID: "d1"}, "/infra", "prod.tfvars")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.calledMethod != "terraform.apply" {
		t.Errorf("expected method terraform.apply, got %q", agent.calledMethod)
	}
	if agent.calledParams["workingDir"] != "/infra" || agent.calledParams["varsFile"] != "prod.tfvars" {
		t.Errorf("unexpected params: %+v", agent.calledParams)
	}
}

func TestTerraformRunner_Apply_OmitsVarsFileWhenEmpty(t *testing.T) {
	agent := &fakeAgentClient{execResult: map[string]any{"outputJson": "{}"}}
	runner := NewTerraformRunner(agent)

	_, err := runner.Apply(context.Background(), domain.DevServer{ID: "d1"}, "/infra", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := agent.calledParams["varsFile"]; ok {
		t.Error("expected varsFile to be omitted from params when empty")
	}
}

func TestTerraformRunner_Apply_ReturnsOutputJSON(t *testing.T) {
	agent := &fakeAgentClient{execResult: map[string]any{"outputJson": `{"instance_hosts":{"value":["h1"]}}`}}
	runner := NewTerraformRunner(agent)

	got, err := runner.Apply(context.Background(), domain.DevServer{ID: "d1"}, "/infra", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"instance_hosts":{"value":["h1"]}}` {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestTerraformRunner_Apply_ExecErrorPropagates(t *testing.T) {
	agent := &fakeAgentClient{execErr: errors.New("agent unreachable")}
	runner := NewTerraformRunner(agent)

	_, err := runner.Apply(context.Background(), domain.DevServer{ID: "d1"}, "/infra", "")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestTerraformRunner_Apply_MissingOutputJSONField_ReturnsError(t *testing.T) {
	agent := &fakeAgentClient{execResult: map[string]any{"somethingElse": "x"}}
	runner := NewTerraformRunner(agent)

	_, err := runner.Apply(context.Background(), domain.DevServer{ID: "d1"}, "/infra", "")
	if err == nil {
		t.Fatal("expected an error when the agent response is missing outputJson")
	}
}
