package main

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// Child entities named in the RPC catalog resolve to their Request before authorization; an unregistered kind
// refuses non-admins, so every entity a handler serves must be listed here.
type clarificationRequestResolver struct {
	repo usecase.ClarificationRepository
}

func (r clarificationRequestResolver) RequestIDOf(ctx context.Context, id string) (string, error) {
	c, err := r.repo.Get(ctx, id)
	return c.RequestID, err
}

type decisionRequestResolver struct{ repo usecase.DecisionRepository }

func (r decisionRequestResolver) RequestIDOf(ctx context.Context, id string) (string, error) {
	d, err := r.repo.Get(ctx, id)
	return d.RequestID, err
}

type artifactRefRequestResolver struct {
	index usecase.ArtifactIndexRepository
}

func (r artifactRefRequestResolver) RequestIDOf(ctx context.Context, ref string) (string, error) {
	e, err := r.index.Resolve(ctx, ref)
	return e.RequestID, err
}

func registerEntityResolvers(authorize *usecase.AuthorizeRequestAction, a artifactStores) {
	authorize.RegisterEntity("clarification", clarificationRequestResolver{a.clarifications})
	authorize.RegisterEntity("decision", decisionRequestResolver{a.decisions})
	authorize.RegisterEntity("artifact_ref", artifactRefRequestResolver{a.index})
}
