package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CapabilitiesChangedSubject follows the service's orca.infrafleet.* outbox
// subject family (the CR's orca.infra.* prefix would not be captured by the
// INFRAFLEET stream).
const CapabilitiesChangedSubject = "orca.infrafleet.dev_server.capabilities_changed"

// capabilityProbeTimeout bounds the agent.capabilities call; the agent's own
// overall budget is 8s.
const capabilityProbeTimeout = 15 * time.Second

type capabilityFlight struct {
	done    chan struct{}
	profile domain.CapabilityProfile
	err     error
}

// RefreshDevServerCapabilities probes a connected agent with
// agent.capabilities and stores the result; an agent without the method
// (JSON-RPC -32601) gets a handshake_only profile built from its handshake.
type RefreshDevServerCapabilities struct {
	devServers  DevServerRepository
	agent       DevServerAgentClient
	store       CapabilityProfileStore
	events      OutboxEnqueuer
	clock       Clock
	minInterval time.Duration
	logger      *slog.Logger

	mu          sync.Mutex
	flights     map[string]*capabilityFlight
	lastAttempt map[string]time.Time
}

func NewRefreshDevServerCapabilities(
	devServers DevServerRepository,
	agent DevServerAgentClient,
	store CapabilityProfileStore,
	events OutboxEnqueuer,
	clock Clock,
	minInterval time.Duration,
) *RefreshDevServerCapabilities {
	return &RefreshDevServerCapabilities{
		devServers:  devServers,
		agent:       agent,
		store:       store,
		events:      events,
		clock:       clock,
		minInterval: minInterval,
		logger:      slog.Default(),
		flights:     make(map[string]*capabilityFlight),
		lastAttempt: make(map[string]time.Time),
	}
}

func (uc *RefreshDevServerCapabilities) WithLogger(l *slog.Logger) *RefreshDevServerCapabilities {
	if l != nil {
		uc.logger = l
	}
	return uc
}

// Execute returns the refreshed profile. Concurrent calls for one dev server
// share a single probe (even with force); without force, a call inside
// minInterval of the previous attempt returns the stored profile so a caller
// cannot turn refresh into a probe storm.
func (uc *RefreshDevServerCapabilities) Execute(ctx context.Context, tenantID, devServerID string, force bool) (domain.CapabilityProfile, error) {
	key := tenantID + ":" + devServerID

	uc.mu.Lock()
	if f, ok := uc.flights[key]; ok {
		uc.mu.Unlock()
		select {
		case <-f.done:
			return f.profile, f.err
		case <-ctx.Done():
			return domain.CapabilityProfile{}, ctx.Err()
		}
	}
	now := uc.clock.Now()
	if !force {
		if last, ok := uc.lastAttempt[key]; ok && now.Sub(last) < uc.minInterval {
			uc.mu.Unlock()
			if p, found, err := uc.store.Get(ctx, tenantID, devServerID); err != nil {
				return domain.CapabilityProfile{}, err
			} else if found {
				return p, nil
			}
			// Nothing stored yet: a first probe is always allowed.
			return uc.Execute(ctx, tenantID, devServerID, true)
		}
	}
	f := &capabilityFlight{done: make(chan struct{})}
	uc.flights[key] = f
	uc.lastAttempt[key] = now
	uc.mu.Unlock()

	// Detach from the leader's cancellation so followers are not failed by one
	// caller hanging up; tenant values in ctx are kept.
	probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), capabilityProbeTimeout)
	defer cancel()
	f.profile, f.err = uc.refresh(probeCtx, tenantID, devServerID, force, now)

	uc.mu.Lock()
	delete(uc.flights, key)
	uc.mu.Unlock()
	close(f.done)
	return f.profile, f.err
}

func (uc *RefreshDevServerCapabilities) refresh(ctx context.Context, tenantID, devServerID string, force bool, now time.Time) (domain.CapabilityProfile, error) {
	ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
	}

	stored, hasStored, err := uc.store.Get(ctx, tenantID, devServerID)
	if err != nil {
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindInternal, "INFRA_CAPABILITY_STORE_FAILED", "failed to read capability profile", err)
	}

	// Exec on a relay mode would dial; a refresh must never open a connection.
	info, handshaked := uc.agent.LastHandshakeInfo(devServerID)
	if !uc.agent.IsConnected(devServerID) || !handshaked {
		if hasStored {
			return stored, nil
		}
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED", "this dev server has no live agent connection right now", nil)
	}

	var params map[string]any
	if force {
		params = map[string]any{"refresh": true}
	}
	res, execErr := uc.agent.Exec(ctx, ds, "agent.capabilities", params)

	var p domain.CapabilityProfile
	switch {
	case errors.Is(execErr, domain.ErrAgentMethodNotFound):
		p, err = buildHandshakeOnlyProfile(info, ds, now)
	case execErr != nil:
		// Transient failure: keep serving the old profile instead of
		// downgrading a healthy agent to handshake_only.
		uc.logger.WarnContext(ctx, "capability probe failed", slog.String("devServerId", devServerID), slog.Any("error", execErr))
		if hasStored {
			return stored, nil
		}
		return domain.CapabilityProfile{}, MapAgentExecError("agent.capabilities", execErr)
	default:
		p, err = buildProbeProfile(res, info, ds, now)
	}
	if err != nil {
		return domain.CapabilityProfile{}, err
	}

	prev, existed, err := uc.store.Upsert(ctx, p)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.CapabilityProfile{}, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
		}
		if errors.Is(err, domain.ErrProfileTooLarge) {
			return domain.CapabilityProfile{}, apperrors.New(apperrors.KindResourceExhausted, "INFRA_CAPABILITY_PROFILE_TOO_LARGE", "capability report exceeds the size limit", err)
		}
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindInternal, "INFRA_CAPABILITY_STORE_FAILED", "failed to store capability profile", err)
	}

	if !existed || prev != p.Fingerprint {
		uc.emitChanged(ctx, p, now)
	}
	return p, nil
}

// emitChanged never fails the refresh: the profile is already durable and the
// next fingerprint change re-emits.
func (uc *RefreshDevServerCapabilities) emitChanged(ctx context.Context, p domain.CapabilityProfile, now time.Time) {
	payload, err := json.Marshal(map[string]any{
		"dev_server_id": p.DevServerID,
		"fingerprint":   p.Fingerprint,
		"source":        string(p.Source),
		"features":      p.Features,
	})
	if err != nil {
		return
	}
	if err := uc.events.EnqueueOutboxEvent(ctx, uuid.NewString(), p.TenantID, CapabilitiesChangedSubject, now, 1, payload); err != nil {
		uc.logger.WarnContext(ctx, "failed to enqueue capabilities_changed", slog.String("devServerId", p.DevServerID), slog.Any("error", err))
	}
}

func buildProbeProfile(res map[string]any, info HandshakeInfo, ds domain.DevServer, now time.Time) (domain.CapabilityProfile, error) {
	if len(res) == 0 {
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindInternal, "INFRA_CAPABILITY_EMPTY_REPORT", "agent.capabilities returned an empty report", nil)
	}
	b, err := json.Marshal(res)
	if err != nil {
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindInternal, "INFRA_CAPABILITY_BAD_REPORT", "agent.capabilities report is not encodable", err)
	}
	fp, err := domain.ComputeFingerprint(b)
	if err != nil {
		return domain.CapabilityProfile{}, apperrors.New(apperrors.KindInternal, "INFRA_CAPABILITY_BAD_REPORT", "agent.capabilities report is not a JSON object", err)
	}
	return domain.CapabilityProfile{
		DevServerID:       ds.ID,
		TenantID:          ds.TenantID,
		Source:            domain.ProfileSourceProbe,
		AgentBuildVersion: info.BuildVersion,
		ProtocolVersion:   info.EffectiveProtocolVersion(),
		Features:          domain.NormalizeFeatures(info.Features),
		ProfileJSON:       b,
		Fingerprint:       fp,
		ProbedAt:          now,
	}, nil
}

// buildHandshakeOnlyProfile covers agents that predate agent.capabilities
// (and the desktop bundle): only what the handshake reported is known.
func buildHandshakeOnlyProfile(info HandshakeInfo, ds domain.DevServer, now time.Time) (domain.CapabilityProfile, error) {
	handshakeCaps := info.Capabilities
	if handshakeCaps == nil {
		handshakeCaps = []string{}
	}
	b, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"agent": map[string]any{
			"buildVersion":    info.BuildVersion,
			"protocolVersion": info.EffectiveProtocolVersion(),
		},
		"host": map[string]any{
			"platform":    info.Platform,
			"arch":        info.Arch,
			"nodeVersion": info.NodeVersion,
		},
		"handshakeCapabilities": handshakeCaps,
	})
	if err != nil {
		return domain.CapabilityProfile{}, fmt.Errorf("encode handshake-only profile: %w", err)
	}
	fp, err := domain.ComputeFingerprint(b)
	if err != nil {
		return domain.CapabilityProfile{}, fmt.Errorf("fingerprint handshake-only profile: %w", err)
	}
	return domain.CapabilityProfile{
		DevServerID:       ds.ID,
		TenantID:          ds.TenantID,
		Source:            domain.ProfileSourceHandshakeOnly,
		AgentBuildVersion: info.BuildVersion,
		ProtocolVersion:   info.EffectiveProtocolVersion(),
		Features:          domain.NormalizeFeatures(info.Features),
		ProfileJSON:       b,
		Fingerprint:       fp,
		ProbedAt:          now,
	}, nil
}
