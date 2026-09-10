package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type DeployFleetDefinitionInput struct {
	FleetDefinitionID  string
	ControlDevServerID string // required only when Provision != nil — validated in Execute
}

// DeployFleetDefinition reads a saved FleetDefinition and, when it has a
// Provision config, runs ApplyTerraformPlan first and merges the resulting
// instances into Servers before calling BulkProvisionFleet for the whole
// list — CR-FLEET-003's coordination point across CR-FLEET-001/002. Does
// NOT reimplement either usecase's logic, only sequences them.
//
// OPEN QUESTION RESOLVED (BE-FLEET-SOL-003 §4): ApplyTerraformPlan's
// TerraformInstance only carries Host — CreateSshTarget (called inside
// BulkProvisionFleet) needs UserName/VaultSSHRole too, and Terraform-created
// instances have no pre-existing Vault role. Chose "hướng 1":
// domain.ProvisionConfig.DefaultUserName/DefaultVaultSSHRole apply to every
// instance Terraform creates for a given definition — simpler than "hướng
// 2" (extending terraform output to carry per-instance vault_ssh_role,
// which would require reopening TASK-BE-FLEET-006's parseTerraformOutput
// convention) and sufficient for CR-FLEET-002's single-cloud-provider MVP
// scope, where one `apply` run typically serves one team/purpose sharing
// one Vault role.
type DeployFleetDefinition struct {
	repo               FleetDefinitionRepository
	applyTerraformPlan *ApplyTerraformPlan
	bulkProvisionFleet *BulkProvisionFleet
}

func NewDeployFleetDefinition(repo FleetDefinitionRepository, applyTerraformPlan *ApplyTerraformPlan, bulkProvisionFleet *BulkProvisionFleet) *DeployFleetDefinition {
	return &DeployFleetDefinition{repo: repo, applyTerraformPlan: applyTerraformPlan, bulkProvisionFleet: bulkProvisionFleet}
}

func (uc *DeployFleetDefinition) Execute(ctx context.Context, in DeployFleetDefinitionInput, emit func(BulkProvisionServerResult)) (BulkProvisionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return BulkProvisionResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, in.FleetDefinitionID)
	if err != nil {
		if err == domain.ErrFleetDefinitionNotFound {
			return BulkProvisionResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
		}
		return BulkProvisionResult{}, apperrors.New(apperrors.KindInternal, "INFRA_GET_FLEET_DEFINITION_FAILED", "failed to get fleet definition", err)
	}

	servers := append([]domain.FleetSpecServer{}, def.Servers...) // copy — never mutate def.Servers itself

	if def.Provision != nil {
		if in.ControlDevServerID == "" {
			return BulkProvisionResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_MISSING_CONTROL_DEV_SERVER", "control_dev_server_id is required when definition has a provision config", nil)
		}
		tfResult, err := uc.applyTerraformPlan.Execute(ctx, ApplyTerraformPlanInput{
			ControlDevServerID: in.ControlDevServerID,
			WorkingDir:         def.Provision.WorkingDir,
			VarsFile:           def.Provision.VarsFile,
		})
		if err != nil {
			// A Terraform failure must stop here, before any
			// BulkProvisionFleet call — otherwise a partial/garbage
			// instance list could still get provisioned.
			return BulkProvisionResult{}, err // already apperrors-wrapped by ApplyTerraformPlan, don't wrap twice
		}
		for _, inst := range tfResult.Instances {
			servers = append(servers, domain.FleetSpecServer{
				Host:         inst.Host,
				UserName:     def.Provision.DefaultUserName,
				VaultSSHRole: def.Provision.DefaultVaultSSHRole,
			})
		}
	}

	return uc.bulkProvisionFleet.Execute(ctx, FleetSpec{Servers: servers}, 0, emit)
}
