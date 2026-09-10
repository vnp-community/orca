package infrafleetclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetPicker implements usecase.InfraFleetPicker against
// infra-fleet-service's PickByTag RPC (TASK-WF-002-04) — used by
// ServerResolver to resolve TargetKindFleetTag ("fleet:tag:<tag>") targets.
type InfraFleetPicker struct {
	client infrafleetv1.InfraFleetServiceClient
}

// NewInfraFleetPicker wraps an already-constructed infrafleetv1 client —
// same already-dialed client the Agent/Shell/Notification executors in this
// package use, per TASK-WF-002-01's wiring note.
func NewInfraFleetPicker(client infrafleetv1.InfraFleetServiceClient) *InfraFleetPicker {
	return &InfraFleetPicker{client: client}
}

func (p *InfraFleetPicker) PickByTag(ctx context.Context, tag string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("infrafleetclient: PickByTag: %w", err)
	}
	resp, err := p.client.PickByTag(ctx, &infrafleetv1.PickByTagRequest{Tag: tag})
	if err != nil {
		return "", err
	}
	return resp.GetConnectionId(), nil
}
