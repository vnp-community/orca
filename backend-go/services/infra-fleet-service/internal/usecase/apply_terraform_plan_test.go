package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeTerraformRunner is an in-memory TerraformRunner.
type fakeTerraformRunner struct {
	outputJSON string
	applyErr   error
	calledWith domain.DevServer
}

func (f *fakeTerraformRunner) Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (string, error) {
	f.calledWith = controlDevServer
	if f.applyErr != nil {
		return "", f.applyErr
	}
	return f.outputJSON, nil
}

func TestApplyTerraformPlan_RequiresTenantContext(t *testing.T) {
	uc := NewApplyTerraformPlan(&fakeDevServerRepository{}, &fakeTerraformRunner{})
	_, err := uc.Execute(context.Background(), ApplyTerraformPlanInput{ControlDevServerID: "d1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestApplyTerraformPlan_UnknownControlDevServer_ReturnsInvalidArgument(t *testing.T) {
	devRepo := &fakeDevServerRepository{getErr: errors.New("not found")}
	uc := NewApplyTerraformPlan(devRepo, &fakeTerraformRunner{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ApplyTerraformPlanInput{ControlDevServerID: "unknown"})
	if err == nil {
		t.Fatal("expected an error for an unknown control dev server")
	}
}

func TestApplyTerraformPlan_RunnerError_WrappedAsInternal(t *testing.T) {
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"d1": {ID: "d1"}}}
	runner := &fakeTerraformRunner{applyErr: errors.New("terraform binary missing")}
	uc := NewApplyTerraformPlan(devRepo, runner)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ApplyTerraformPlanInput{ControlDevServerID: "d1"})
	if err == nil {
		t.Fatal("expected an error to propagate from the runner")
	}
}

func TestApplyTerraformPlan_Success_ParsesInstancesAndPassesControlDevServer(t *testing.T) {
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"d1": {ID: "d1", Host: "control-1"}}}
	runner := &fakeTerraformRunner{outputJSON: `{"instance_hosts":{"value":["10.0.0.1","10.0.0.2"]}}`}
	uc := NewApplyTerraformPlan(devRepo, runner)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, ApplyTerraformPlanInput{ControlDevServerID: "d1", WorkingDir: "/infra"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Instances) != 2 || result.Instances[0].Host != "10.0.0.1" || result.Instances[1].Host != "10.0.0.2" {
		t.Errorf("unexpected instances: %+v", result.Instances)
	}
	if runner.calledWith.ID != "d1" {
		t.Errorf("expected runner.Apply to be called with the resolved control dev server, got %+v", runner.calledWith)
	}
}

func TestParseTerraformOutput_ValidInstanceHosts_ReturnsInstances(t *testing.T) {
	instances, err := parseTerraformOutput(`{"instance_hosts":{"value":["h1","h2"]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 || instances[0].Host != "h1" || instances[1].Host != "h2" {
		t.Errorf("unexpected instances: %+v", instances)
	}
}

func TestParseTerraformOutput_MissingInstanceHostsKey_ReturnsEmptyNotError(t *testing.T) {
	instances, err := parseTerraformOutput(`{"other_output":{"value":"x"}}`)
	if err != nil {
		t.Fatalf("expected no error for a missing instance_hosts key, got: %v", err)
	}
	if len(instances) != 0 {
		t.Errorf("expected 0 instances, got %d", len(instances))
	}
}

func TestParseTerraformOutput_MalformedJSON_ReturnsError(t *testing.T) {
	_, err := parseTerraformOutput(`not json`)
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}
