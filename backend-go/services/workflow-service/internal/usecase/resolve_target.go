package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ServerResolver resolves a parsed domain.TargetSpec down to a concrete
// infra-fleet-service connection ID — the missing layer step.go's
// ConnectionID doc comment names. One Resolve call per step dispatch (not
// cached across steps): a fleet:tag: target may deliberately return a
// different connection on each call once PickByTag's load-balancing is
// live.
type ServerResolver struct {
	project    ProjectClient
	infraFleet InfraFleetPicker
}

func NewServerResolver(project ProjectClient, infraFleet InfraFleetPicker) *ServerResolver {
	return &ServerResolver{project: project, infraFleet: infraFleet}
}

func (r *ServerResolver) Resolve(ctx context.Context, spec domain.TargetSpec) (string, error) {
	switch spec.Kind {
	case domain.TargetKindProject:
		return r.project.GetProject(ctx, spec.ID)
	case domain.TargetKindServer:
		return spec.ID, nil
	case domain.TargetKindFleetTag:
		return r.infraFleet.PickByTag(ctx, spec.Tag)
	default:
		return "", domain.ErrUnknownTargetKind
	}
}
