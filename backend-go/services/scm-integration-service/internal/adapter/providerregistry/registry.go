// Package providerregistry implements usecase.ProviderRegistry as a simple
// in-memory lookup, populated once by cmd/server/main.go's composition root
// with one entry per internal/adapter/{github,gitlab,bitbucket,azuredevops,
// gitea} implementation — see scm-integration-service.md §6.
package providerregistry

import (
	"fmt"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// Registry implements usecase.ProviderRegistry.
type Registry struct {
	providers map[domain.ScmProvider]usecase.ScmProvider
}

// New returns a Registry backed by the given provider->adapter map.
func New(providers map[domain.ScmProvider]usecase.ScmProvider) *Registry {
	return &Registry{providers: providers}
}

// Resolve returns the adapter registered for provider, or an error if none
// is registered — the usecase layer maps this to codes.InvalidArgument
// (see internal/usecase's apperrors.KindInvalidArgument usage).
func (r *Registry) Resolve(provider domain.ScmProvider) (usecase.ScmProvider, error) {
	p, ok := r.providers[provider]
	if !ok {
		return nil, fmt.Errorf("providerregistry: no adapter registered for provider %q", provider)
	}
	return p, nil
}
