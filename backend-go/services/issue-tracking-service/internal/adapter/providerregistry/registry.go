// Package providerregistry implements usecase.ProviderRegistry — a static,
// composition-root-populated map from domain.Provider to the concrete
// adapter (internal/adapter/jira or internal/adapter/linear) that handles
// it. No I/O of its own; this is wiring, not an external-system adapter.
package providerregistry

import (
	"fmt"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

type Registry struct {
	providers map[domain.Provider]usecase.IssueTrackerProvider
}

// New returns an empty Registry — populate it with Register calls from
// cmd/server/main.go.
func New() *Registry {
	return &Registry{providers: make(map[domain.Provider]usecase.IssueTrackerProvider)}
}

var _ usecase.ProviderRegistry = (*Registry)(nil)

// Register associates provider with impl and returns the Registry, so
// main.go can chain calls at composition time.
func (r *Registry) Register(provider domain.Provider, impl usecase.IssueTrackerProvider) *Registry {
	r.providers[provider] = impl
	return r
}

func (r *Registry) Resolve(provider domain.Provider) (usecase.IssueTrackerProvider, error) {
	impl, ok := r.providers[provider]
	if !ok {
		return nil, fmt.Errorf("providerregistry: no adapter registered for provider %q", provider)
	}
	return impl, nil
}
