// Package providerregistry implements usecase.SsoExchangerRegistry as a
// simple in-memory lookup — mirrors scm-integration-service's
// providerregistry.OAuthRegistry shape/rationale.
package providerregistry

import (
	"fmt"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// SsoRegistry implements usecase.SsoExchangerRegistry, populated once by
// cmd/server/main.go's composition root with one internal/adapter/oauth
// exchanger per provider that has SSO_*_CLIENT_ID configured — a provider
// with no configured client_id is simply absent from the map. Resolve
// returns an error for it, which StartSsoLogin surfaces as
// AUTH_SSO_PROVIDER_UNSUPPORTED.
type SsoRegistry struct {
	exchangers map[domain.SsoProvider]usecase.SsoExchanger
}

// NewSsoRegistry returns an SsoRegistry backed by the given provider->exchanger map.
func NewSsoRegistry(exchangers map[domain.SsoProvider]usecase.SsoExchanger) *SsoRegistry {
	return &SsoRegistry{exchangers: exchangers}
}

var _ usecase.SsoExchangerRegistry = (*SsoRegistry)(nil)

func (r *SsoRegistry) Resolve(provider domain.SsoProvider) (usecase.SsoExchanger, error) {
	e, ok := r.exchangers[provider]
	if !ok {
		return nil, fmt.Errorf("providerregistry: no sso configuration registered for provider %q", provider)
	}
	return e, nil
}
