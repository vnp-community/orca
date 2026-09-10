package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type ApplyTerraformPlanInput struct {
	ControlDevServerID string
	WorkingDir         string
	VarsFile           string
}

type TerraformInstance struct {
	Host string
}

type ApplyTerraformPlanResult struct {
	OutputJSON string
	Instances  []TerraformInstance
}

// ApplyTerraformPlan orchestrates `terraform apply` on a registered control
// dev server (via TerraformRunner/the agent) and parses its output into a
// minimal instance list DeployFleetDefinition (TASK-BE-FLEET-014) consumes.
//
// SCOPE: this usecase does NOT handle cloud-provider credentials — see
// TASK-BE-FLEET-009 (design + security review, not yet implemented).
// TerraformRunner's real implementation (agent RPC terraform.apply,
// TASK-BE-FLEET-007) currently has no way to inject such credentials; until
// TASK-BE-FLEET-009 lands, `terraform apply` runs with whatever the agent
// process's own environment already has (e.g. operator-provisioned), never
// something this usecase reads or stores itself.
type ApplyTerraformPlan struct {
	devServers DevServerRepository
	runner     TerraformRunner
}

func NewApplyTerraformPlan(devServers DevServerRepository, runner TerraformRunner) *ApplyTerraformPlan {
	return &ApplyTerraformPlan{devServers: devServers, runner: runner}
}

func (uc *ApplyTerraformPlan) Execute(ctx context.Context, in ApplyTerraformPlanInput) (ApplyTerraformPlanResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	controlDevServer, err := uc.devServers.Get(ctx, tenantID, in.ControlDevServerID)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_UNKNOWN_CONTROL_HOST", "control dev server not found", err)
	}

	outputJSON, err := uc.runner.Apply(ctx, controlDevServer, in.WorkingDir, in.VarsFile)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInternal, "INFRA_TERRAFORM_APPLY_FAILED", "terraform apply failed", err)
	}

	instances, err := parseTerraformOutput(outputJSON)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInternal, "INFRA_TERRAFORM_OUTPUT_PARSE_FAILED", "failed to parse terraform output", err)
	}
	return ApplyTerraformPlanResult{OutputJSON: outputJSON, Instances: instances}, nil
}

// parseTerraformOutput expects a `terraform output -json` document with a
// top-level output named "instance_hosts" of type list(string) — this is
// an Orca-imposed convention on the user-authored .tf module (CR-FLEET-002
// §"Không thuộc phạm vi": Orca does not generate HCL, but it must agree
// with the user on ONE output name to consume). Document this convention
// wherever the fleet YAML's provision.workingDir field is documented for
// end users — a .tf module missing this output fails ApplyTerraformPlan
// with INFRA_TERRAFORM_OUTPUT_PARSE_FAILED, not a silent empty result.
func parseTerraformOutput(outputJSON string) ([]TerraformInstance, error) {
	var raw struct {
		InstanceHosts struct {
			Value []string `json:"value"`
		} `json:"instance_hosts"`
	}
	if err := json.Unmarshal([]byte(outputJSON), &raw); err != nil {
		return nil, err
	}
	instances := make([]TerraformInstance, 0, len(raw.InstanceHosts.Value))
	for _, host := range raw.InstanceHosts.Value {
		instances = append(instances, TerraformInstance{Host: host})
	}
	return instances, nil
}
