package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type getHarness struct {
	*refreshHarness
	get      *GetDevServerCapabilities
	resolver *fakeConnectionResolver
}

func newGetHarness(t *testing.T, ttl time.Duration) *getHarness {
	t.Helper()
	r := newRefreshHarness(t, time.Nanosecond)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": r.devSrvs.byID[capDevServer]}}
	return &getHarness{
		refreshHarness: r,
		resolver:       resolver,
		get:            NewGetDevServerCapabilities(resolver, r.devSrvs, r.store, r.uc, r.agent, ttl, r.clock),
	}
}

func capCtx() context.Context { return tenant.WithTenantID(context.Background(), capTenant) }

func appErrKind(t *testing.T, err error) apperrors.Kind {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want AppError, got %v", err)
	}
	return ae.Kind
}

func TestGet_ExactlyOneID(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	for name, in := range map[string]GetCapabilitiesInput{
		"neither": {},
		"both":    {ConnectionID: "conn-1", DevServerID: capDevServer},
	} {
		_, err := h.get.Execute(capCtx(), in)
		if appErrKind(t, err) != apperrors.KindInvalidArgument {
			t.Errorf("%s: want InvalidArgument, got %v", name, err)
		}
	}
	if h.agent.execCount.Load() != 0 {
		t.Errorf("invalid input must not probe")
	}
}

func TestGet_RequiresTenant(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	_, err := h.get.Execute(context.Background(), GetCapabilitiesInput{DevServerID: capDevServer})
	if appErrKind(t, err) != apperrors.KindUnauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestGet_NoProfileConnected_ProbesOnce(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	res, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.Found || !res.Connected || res.Profile.Source != domain.ProfileSourceProbe {
		t.Errorf("unexpected result %+v", res)
	}
	if h.agent.execCount.Load() != 1 {
		t.Errorf("exec count=%d, want 1", h.agent.execCount.Load())
	}
}

func TestGet_FreshProfileNoRefresh(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(10 * time.Minute)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("second: %v", err)
	}
	if n := h.agent.execCount.Load(); n != 1 {
		t.Errorf("exec count=%d, a fresh profile must not re-probe", n)
	}
}

func TestGet_TTLExpiredRefreshes(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(2 * time.Hour)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("second: %v", err)
	}
	if n := h.agent.execCount.Load(); n != 2 {
		t.Errorf("exec count=%d, want 2 after TTL", n)
	}
}

func TestGet_RefreshFlagForcesProbe(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer, Refresh: true}); err != nil {
		t.Fatalf("second: %v", err)
	}
	if n := h.agent.execCount.Load(); n != 2 {
		t.Errorf("exec count=%d, want 2", n)
	}
}

func TestGet_NotConnectedNoProfile_NotFound(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	h.agent.connected = false
	_, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound || ae.Code != "INFRA_CAPABILITY_PROFILE_NOT_FOUND" {
		t.Fatalf("want INFRA_CAPABILITY_PROFILE_NOT_FOUND, got %v", err)
	}
	if h.agent.execCount.Load() != 0 {
		t.Errorf("disconnected dev server must never be probed")
	}
}

func TestGet_NotConnectedWithProfile_ServesStoredDisconnected(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("first: %v", err)
	}
	h.agent.set(func(a *capabilityFakeAgent) { a.connected = false })
	h.clock.advance(48 * time.Hour)
	res, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer, Refresh: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Connected || !res.Found {
		t.Errorf("want stored profile with connected=false, got %+v", res)
	}
	if h.agent.execCount.Load() != 1 {
		t.Errorf("no new probe expected while disconnected")
	}
}

func TestGet_RefreshFailureServesStoredProfile(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	first, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	h.clock.advance(2 * time.Hour)
	// Get's own read is call 1 (succeeds); the refresh's read (call 2) fails.
	h.store.getCalls = 0
	h.store.failGetAfter = 1
	res, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer})
	if err != nil {
		t.Fatalf("a failed refresh must fall back to the stored profile: %v", err)
	}
	if res.Profile.Fingerprint != first.Profile.Fingerprint || !res.Found {
		t.Errorf("stored profile not served: %+v", res)
	}
}

func TestGet_UnknownDevServerForTenant_NotFound(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	h.devSrvs.byID = map[string]domain.DevServer{}
	_, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer})
	if appErrKind(t, err) != apperrors.KindNotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestGet_ConnectionIDResolves(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	res, err := h.get.Execute(capCtx(), GetCapabilitiesInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Profile.DevServerID != capDevServer {
		t.Errorf("dev server id = %q", res.Profile.DevServerID)
	}
	_, err = h.get.Execute(capCtx(), GetCapabilitiesInput{ConnectionID: "unknown"})
	if appErrKind(t, err) != apperrors.KindNotFound {
		t.Errorf("unknown connection: want NotFound, got %v", err)
	}
}

func TestGet_TenantFromContextOnly(t *testing.T) {
	h := newGetHarness(t, time.Hour)
	if _, err := h.get.Execute(capCtx(), GetCapabilitiesInput{DevServerID: capDevServer}); err != nil {
		t.Fatalf("owner: %v", err)
	}
	other := tenant.WithTenantID(context.Background(), "tenant-2")
	_, err := h.get.Execute(other, GetCapabilitiesInput{DevServerID: capDevServer})
	if appErrKind(t, err) != apperrors.KindNotFound {
		t.Fatalf("another tenant must not see the profile, got %v", err)
	}
}
