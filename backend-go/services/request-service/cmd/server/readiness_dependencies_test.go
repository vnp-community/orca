package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/opaclient"
)

const repoPolicyBundle = "../../../../policy/orca-authz"

func readyzStatus(register func(h *health.Server)) (int, map[string]string) {
	h := health.New()
	register(h)
	return serveReadyz(h)
}

func TestReadyz_OPABundle(t *testing.T) {
	good := opaclient.NewRequestPolicy(repoPolicyBundle)
	code, body := readyzStatus(func(h *health.Server) { registerPolicyBundleReadiness(h, good) })
	if code != http.StatusOK || body["opa_bundle"] != "ok" {
		t.Fatalf("a loadable bundle: %d %v", code, body)
	}

	missing := opaclient.NewRequestPolicy("./no-such-bundle")
	code, body = readyzStatus(func(h *health.Server) { registerPolicyBundleReadiness(h, missing) })
	if code != http.StatusServiceUnavailable || body["opa_bundle"] == "ok" || body["opa_bundle"] == "" {
		t.Fatalf("a missing bundle must name itself as the failing check: %d %v", code, body)
	}
}

func TestProbeWithTimeout_AHungDependencyIsNotReady(t *testing.T) {
	old := readinessProbeTimeout
	readinessProbeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { readinessProbeTimeout = old })

	start := time.Now()
	err := probeWithTimeout(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(start))
	}
	if err := probeWithTimeout(func(context.Context) error { return nil }); err != nil {
		t.Fatalf("a healthy dependency: %v", err)
	}
}

func TestReadyz_DatabaseDownIsNotReady(t *testing.T) {
	code, body := readyzStatus(func(h *health.Server) {
		h.Register("postgres", func() error {
			return probeWithTimeout(func(context.Context) error { return errors.New("connection refused") })
		})
	})
	if code != http.StatusServiceUnavailable || body["postgres"] != "connection refused" {
		t.Fatalf("%d %v", code, body)
	}
}
