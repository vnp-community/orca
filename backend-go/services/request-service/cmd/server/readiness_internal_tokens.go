package main

import (
	"errors"

	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
)

// registerInternalTokenReadiness makes a missing internal token visible as "not ready". With an empty
// GATEWAY_INTERNAL_TOKEN or SERVICE_INTERNAL_TOKEN the guarded RPCs refuse every call, and from the outside that looks like
// REQUEST_FORBIDDEN (a permission problem) rather than a configuration problem.
func registerInternalTokenReadiness(h *health.Server, cfg config.Config) {
	h.Register("internal_tokens", func() error { return internalTokensConfigured(cfg) })
}

func internalTokensConfigured(cfg config.Config) error {
	switch {
	case cfg.GatewayInternalToken == "" && cfg.ServiceInternalToken == "":
		return errors.New("GATEWAY_INTERNAL_TOKEN and SERVICE_INTERNAL_TOKEN are empty: every guarded RPC is refused")
	case cfg.GatewayInternalToken == "":
		return errors.New("GATEWAY_INTERNAL_TOKEN is empty: the api-gateway cannot call this service")
	case cfg.ServiceInternalToken == "":
		return errors.New("SERVICE_INTERNAL_TOKEN is empty: internal callers (issue-status-sync) are refused")
	}
	return nil
}
