package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// FleetServerInput mirrors ImportFleetInventoryRequest's FleetServerInput
// message 1:1, see CreateSshTargetInput's doc comment for the convention.
type FleetServerInput struct {
	Host, UserName, VaultSSHRole, Project string
	Tags                                  []string
}

// ImportFleetInventoryInput is BL-FLEET-01's batch YAML-import request —
// each server is upserted independently (skip-and-continue), so one
// malformed row never aborts an otherwise-valid batch.
type ImportFleetInventoryInput struct {
	Servers []FleetServerInput
	DryRun  bool
}

// ImportFleetInventoryError records why one server in the batch was
// skipped — Host/UserName identify the offending row since it may not have
// a domain.SshTarget (and therefore no ID) at all.
type ImportFleetInventoryError struct {
	Host, UserName, Reason string
}

// ImportFleetInventoryResult is BL-FLEET-01's {imported, updated, skipped,
// errors} contract.
type ImportFleetInventoryResult struct {
	Imported, Updated, Skipped int
	Errors                     []ImportFleetInventoryError
}

// ImportFleetInventory upserts SshTargets by (tenant_id, host, user_name) —
// see SshTargetRepository.Upsert's doc comment for the conflict target.
// DryRun reports what WOULD happen (imported vs. updated counts) without
// writing anything, via the non-mutating GetByHostUser probe.
type ImportFleetInventory struct {
	repo SshTargetRepository
}

func NewImportFleetInventory(repo SshTargetRepository) *ImportFleetInventory {
	return &ImportFleetInventory{repo: repo}
}

func (uc *ImportFleetInventory) Execute(ctx context.Context, in ImportFleetInventoryInput) (ImportFleetInventoryResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ImportFleetInventoryResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	var result ImportFleetInventoryResult
	for _, s := range in.Servers {
		target, err := domain.NewSshTarget(uuid.NewString(), tenantID, s.Host, s.UserName, s.VaultSSHRole, s.Project, s.Tags)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
			continue
		}
		if in.DryRun {
			_, found, err := uc.repo.GetByHostUser(ctx, tenantID, s.Host, s.UserName)
			if err != nil {
				result.Skipped++
				result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
				continue
			}
			if found {
				result.Updated++
			} else {
				result.Imported++
			}
			continue
		}
		_, updated, err := uc.repo.Upsert(ctx, target)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
			continue
		}
		if updated {
			result.Updated++
		} else {
			result.Imported++
		}
	}
	return result, nil
}
