package providerregistry

import (
	"fmt"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// OAuthRegistry implements usecase.OAuthExchangerRegistry as a simple
// in-memory lookup — mirrors Registry's shape/rationale (see its doc
// comment) for OAuth exchangers instead of ScmProvider adapters, populated
// once by cmd/server/main.go's composition root with one
// internal/adapter/oauth.Client per provider that has OAuth credentials
// configured (a provider with no configured client_id/secret is simply
// absent from the map — Resolve returns an error for it, which
// StartOAuthFlow surfaces as SCM_PROVIDER_UNSUPPORTED, same as an
// unregistered ScmProvider adapter).
type OAuthRegistry struct {
	exchangers map[domain.ScmProvider]usecase.OAuthExchanger
}

// NewOAuth returns an OAuthRegistry backed by the given provider->exchanger
// map.
func NewOAuth(exchangers map[domain.ScmProvider]usecase.OAuthExchanger) *OAuthRegistry {
	return &OAuthRegistry{exchangers: exchangers}
}

func (r *OAuthRegistry) Resolve(provider domain.ScmProvider) (usecase.OAuthExchanger, error) {
	e, ok := r.exchangers[provider]
	if !ok {
		return nil, fmt.Errorf("providerregistry: no oauth configuration registered for provider %q", provider)
	}
	return e, nil
}
