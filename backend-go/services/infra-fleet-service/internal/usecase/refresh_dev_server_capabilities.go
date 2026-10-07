package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type OutboxEnqueuer interface {
	EnqueueOutboxEvent(ctx context.Context, id, tenantID, subject string, now time.Time, version int, payload json.RawMessage) error
}

type RefreshDevServerCapabilities struct {
	resolver    ConnectionResolver
	devServers  DevServerRepository
	agent       DevServerAgentClient
	store       CapabilityProfileStore
	events      OutboxEnqueuer
	clock       Clock
	inflight    sync.Map // tenantID+devServerID -> *sync.Mutex
	lastAttempt sync.Map // tenantID+devServerID -> time.Time
	lastMu      sync.Mutex
	minInterval time.Duration
}

func NewRefreshDevServerCapabilities(
	resolver ConnectionResolver,
	devServers DevServerRepository,
	agent DevServerAgentClient,
	store CapabilityProfileStore,
	events OutboxEnqueuer,
	clock Clock,
	minInterval time.Duration,
) *RefreshDevServerCapabilities {
	return &RefreshDevServerCapabilities{
		resolver:    resolver,
		devServers:  devServers,
		agent:       agent,
		store:       store,
		events:      events,
		clock:       clock,
		minInterval: minInterval,
	}
}

func (uc *RefreshDevServerCapabilities) Execute(ctx context.Context, tenantID, devServerID string, force bool) (domain.CapabilityProfile, error) {
	key := tenantID + ":" + devServerID

	muIface, _ := uc.inflight.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	now := uc.clock.Now()

	if !force {
		uc.lastMu.Lock()
		last, ok := uc.lastAttempt.Load(key)
		uc.lastMu.Unlock()
		if ok && now.Sub(last.(time.Time)) < uc.minInterval {
			// Skip and return stored
			p, ok, err := uc.store.Get(ctx, tenantID, devServerID)
			if err != nil {
				return domain.CapabilityProfile{}, err
			}
			if ok {
				return p, nil
			}
		}
	}

	uc.lastMu.Lock()
	uc.lastAttempt.Store(key, now)
	uc.lastMu.Unlock()

	ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return domain.CapabilityProfile{}, err
	}

	info, _ := uc.agent.LastHandshakeInfo(devServerID)
	
	res, execErr := uc.agent.Exec(ctx, ds, "agent.capabilities", nil)
	
	var p domain.CapabilityProfile

	if errors.Is(execErr, domain.ErrAgentMethodNotFound) || execErr != nil {
		// Handshake only fallback
		p = uc.buildHandshakeOnlyProfile(info, ds, now)
	} else {
		// Process probe result
		b, _ := json.Marshal(res)
		fingerprint, _ := domain.ComputeFingerprint(b)
		p = domain.CapabilityProfile{
			DevServerID:       devServerID,
			TenantID:          tenantID,
			Source:            domain.ProfileSourceProbe,
			AgentBuildVersion: info.BuildVersion,
			ProtocolVersion:   info.EffectiveProtocolVersion(),
			Features:          info.Features,
			ProfileJSON:       b,
			Fingerprint:       fingerprint,
			ProbedAt:          now,
		}
	}

	prev, existed, err := uc.store.Upsert(ctx, p)
	if err != nil {
		return domain.CapabilityProfile{}, err
	}

	if (!existed) || (existed && prev != p.Fingerprint) {
		payload, _ := json.Marshal(map[string]any{
			"dev_server_id": devServerID,
			"fingerprint":   p.Fingerprint,
			"source":        p.Source,
			"features":      p.Features,
		})
		uc.events.EnqueueOutboxEvent(ctx, devServerID, tenantID, "orca.infra.dev_server.capabilities_changed", now, 1, payload)
	}

	return p, nil
}

func (uc *RefreshDevServerCapabilities) buildHandshakeOnlyProfile(info HandshakeInfo, ds domain.DevServer, now time.Time) domain.CapabilityProfile {
	profile := map[string]any{
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
		"handshakeCapabilities": info.Capabilities,
	}
	b, _ := json.Marshal(profile)
	fingerprint, _ := domain.ComputeFingerprint(b)
	
	return domain.CapabilityProfile{
		DevServerID:       ds.ID,
		TenantID:          ds.TenantID, // Assuming ds has TenantID
		Source:            domain.ProfileSourceHandshakeOnly,
		AgentBuildVersion: info.BuildVersion,
		ProtocolVersion:   info.EffectiveProtocolVersion(),
		Features:          info.Features,
		ProfileJSON:       b,
		Fingerprint:       fingerprint,
		ProbedAt:          now,
	}
}
