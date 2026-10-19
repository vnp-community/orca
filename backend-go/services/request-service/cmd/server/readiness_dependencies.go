package main

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/opaclient"
)

// readinessProbeTimeout keeps a hung dependency from hanging /readyz itself: the probe must answer "not ready"
// before the container healthcheck gives up.
var readinessProbeTimeout = 3 * time.Second

// probeWithTimeout runs one dependency check (a database ping) under readinessProbeTimeout.
func probeWithTimeout(check func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), readinessProbeTimeout)
	defer cancel()
	return check(ctx)
}

// registerPolicyBundleReadiness makes a missing or broken OPA bundle visible as "not ready": every guarded RPC runs
// through it, so without the bundle the service is up but refuses all work.
func registerPolicyBundleReadiness(h *health.Server, policy *opaclient.RequestPolicy) {
	h.Register("opa_bundle", func() error { return probeWithTimeout(policy.Ready) })
}
