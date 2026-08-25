// Package policypublisher implements usecase.PolicyDataPublisher — the port
// UpdateAccessPolicy uses to push a newly-versioned AccessPolicy to the OPA
// bundle registry (auth-service.md:194). No real OPA bundle-registry
// integration exists in this codebase yet (see internal/adapter/opaclient,
// which only ever evaluates the embedded admin.rego decision — it never
// publishes bundle data), so NoopPublisher is a logging-only stand-in kept
// behind the same interface UpdateAccessPolicy already depends on, so
// swapping in a real bundle-registry client later touches only
// cmd/server/main.go's wiring, not usecase code.
package policypublisher

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// NoopPublisher logs that a policy changed but does not push it anywhere —
// see package doc comment for why this is a deliberate stub, not an
// oversight.
type NoopPublisher struct {
	Logger *slog.Logger
}

func New(logger *slog.Logger) *NoopPublisher {
	return &NoopPublisher{Logger: logger}
}

func (p *NoopPublisher) PublishPolicyChange(ctx context.Context, policy domain.AccessPolicy) error {
	if p.Logger != nil {
		p.Logger.WarnContext(ctx, "policypublisher: OPA bundle registry not wired yet — policy change was persisted but NOT published",
			slog.String("policy_id", policy.ID), slog.Int("version", int(policy.Version)))
	}
	return nil
}
