package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type DeployFleetDefinitionInput struct {
	FleetDefinitionID  string
	ControlDevServerID string // required only when Provision != nil — validated in Execute
}

// deployProjectPrefix scopes the SSH targets this usecase upserts (see
// Execute) so BulkProvisionFleet's Project filter provisions exactly this
// definition's servers, not every SSH target the tenant has registered.
const deployProjectPrefix = "fleetdef:"

// DeployFleetDefinition reads a saved FleetDefinition and, when it has a
// Provision config, runs ApplyTerraformPlan first and merges the resulting
// instances into Servers — CR-FLEET-003's coordination point across
// CR-FLEET-001/002. Does not reimplement either usecase's logic, only
// sequences them.
//
// BulkProvisionFleet (CR-FLEET-001) provisions whatever SSH targets are
// already registered for a tenant/project, it does not accept an ad hoc
// server list — so this usecase first Upserts every one of the
// definition's servers as an SshTarget tagged with a project derived from
// the definition's ID (deployProjectPrefix+def.ID), then calls
// BulkProvisionFleet scoped to that project. This keeps BulkProvisionFleet
// itself unaware of FleetDefinition entirely, matching this codebase's
// existing "orchestrator composes, doesn't reach into other usecases'
// internals" convention.
//
// OPEN QUESTION RESOLVED (BE-FLEET-SOL-003 §4): ApplyTerraformPlan's
// TerraformInstance only carries Host — the upserted SshTarget needs
// UserName/VaultSSHRole too, and Terraform-created instances have no
// pre-existing Vault role. Chose "hướng 1":
// domain.ProvisionConfig.DefaultUserName/DefaultVaultSSHRole apply to every
// instance Terraform creates for a given definition — simpler than "hướng
// 2" (extending terraform output to carry per-instance vault_ssh_role,
// which would require reopening TASK-BE-FLEET-006's parseTerraformOutput
// convention) and sufficient for CR-FLEET-002's single-cloud-provider MVP
// scope, where one `apply` run typically serves one team/purpose sharing
// one Vault role.
type DeployFleetDefinition struct {
	repo               FleetDefinitionRepository
	sshTargets         SshTargetRepository
	applyTerraformPlan *ApplyTerraformPlan
	bulkProvisionFleet *BulkProvisionFleet
}

func NewDeployFleetDefinition(repo FleetDefinitionRepository, sshTargets SshTargetRepository, applyTerraformPlan *ApplyTerraformPlan, bulkProvisionFleet *BulkProvisionFleet) *DeployFleetDefinition {
	return &DeployFleetDefinition{repo: repo, sshTargets: sshTargets, applyTerraformPlan: applyTerraformPlan, bulkProvisionFleet: bulkProvisionFleet}
}

func (uc *DeployFleetDefinition) Execute(ctx context.Context, in DeployFleetDefinitionInput) (BulkProvisionFleetResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, in.FleetDefinitionID)
	if err != nil {
		if err == domain.ErrFleetDefinitionNotFound {
			return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
		}
		return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindInternal, "INFRA_GET_FLEET_DEFINITION_FAILED", "failed to get fleet definition", err)
	}

	servers := append([]domain.FleetSpecServer{}, def.Servers...) // copy — never mutate def.Servers itself

	if def.Provision != nil {
		if in.ControlDevServerID == "" {
			return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_MISSING_CONTROL_DEV_SERVER", "control_dev_server_id is required when definition has a provision config", nil)
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
			return BulkProvisionFleetResult{}, err // already apperrors-wrapped by ApplyTerraformPlan, don't wrap twice
		}
		for _, inst := range tfResult.Instances {
			servers = append(servers, domain.FleetSpecServer{
				Host:         inst.Host,
				UserName:     def.Provision.DefaultUserName,
				VaultSSHRole: def.Provision.DefaultVaultSSHRole,
			})
		}
	}

	project := deployProjectPrefix + def.ID
	for _, s := range servers {
		target, err := domain.NewSshTarget(uuid.NewString(), tenantID, s.Host, 0, s.UserName, s.VaultSSHRole, "", "", project, nil)
		if err != nil {
			return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindInternal, "INFRA_DEPLOY_INVALID_SSH_TARGET", fmt.Sprintf("invalid server %q in fleet definition", s.Host), err)
		}
		// Upsert (not Create) — a re-deploy of the same definition must not
		// fail on the (tenant_id, host, user_name) unique index a prior
		// deploy already satisfied.
		if _, _, err := uc.sshTargets.Upsert(ctx, target); err != nil {
			return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindInternal, "INFRA_DEPLOY_UPSERT_SSH_TARGET_FAILED", fmt.Sprintf("failed to register server %q for provisioning", s.Host), err)
		}
	}

	return uc.bulkProvisionFleet.Execute(ctx, BulkProvisionFleetInput{Project: project})
}
