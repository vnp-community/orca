package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ScanNestedInput struct {
	DevServerID string
	RootPath    string
}

// ScanNested relays a filesystem scan to the Dev Server Agent via
// infra-fleet-service's CreateConnection+Relay — never checked against
// project-service's own host (the "legacy desktop-app assumption" BUG-021
// flags). Requires only an authenticated tenant — there is no project yet
// to check membership against at this pre-import stage.
type ScanNested struct {
	relay DevServerRelay
}

func NewScanNested(relay DevServerRelay) *ScanNested {
	return &ScanNested{relay: relay}
}

func (uc *ScanNested) Execute(ctx context.Context, in ScanNestedInput) ([]domain.NestedRepoCandidate, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	connID, err := uc.relay.CreateConnection(ctx, in.DevServerID, in.RootPath, "")
	if err != nil {
		// An unknown/unreachable dev_server_id fails here — the only
		// validation this usecase performs, see this task's Context.
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED", "failed to connect to dev server", err)
	}
	params, err := json.Marshal(map[string]string{"path": in.RootPath})
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_SCAN_PARAMS_FAILED", "failed to encode scan params", err)
	}
	resultJSON, err := uc.relay.Relay(ctx, connID, "fs.scanNestedRepos", params)
	if err != nil {
		// Fails closed — no local-disk fallback, matching
		// infra-fleet-service.md §10's correctness bar for ScanWorkspacePorts.
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_SCAN_NESTED_FAILED", "failed to scan nested repos on dev server", err)
	}
	candidates, err := domain.ParseNestedRepoCandidates(resultJSON)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_SCAN_PARSE_FAILED", "failed to parse scan result", err)
	}
	return candidates, nil
}
