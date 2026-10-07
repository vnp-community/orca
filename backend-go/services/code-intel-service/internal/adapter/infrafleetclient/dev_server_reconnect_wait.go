package infrafleetclient

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// reconnectGate coalesces concurrent calls waiting for devServer reconnection per (tenantID, devServerID).
type reconnectGate struct {
	mu      sync.Mutex
	waiters map[string]*gateEntry
	client  infrafleetv1.InfraFleetServiceClient
	budget  time.Duration
}

type gateEntry struct {
	mu        sync.Mutex
	connected bool
	doneCh    chan struct{}
	err       error
	refCount  int
}

func newReconnectGate(client infrafleetv1.InfraFleetServiceClient, budget time.Duration) *reconnectGate {
	if budget <= 0 {
		budget = 20 * time.Second
	}
	return &reconnectGate{
		waiters: make(map[string]*gateEntry),
		client:  client,
		budget:  budget,
	}
}

func (g *reconnectGate) key(tenantID, devServerID string) string {
	return tenantID + "|" + devServerID
}

// waitForReconnect waits for devServerID under tenantID to reconnect, polling via IsDevServerConnected.
// It coalesces concurrent callers into a single polling goroutine per tenant+devServer.
func (g *reconnectGate) waitForReconnect(ctx context.Context, tenantID, devServerID string) error {
	k := g.key(tenantID, devServerID)

	g.mu.Lock()
	entry, exists := g.waiters[k]
	if !exists {
		entry = &gateEntry{
			doneCh: make(chan struct{}),
		}
		g.waiters[k] = entry
		go g.pollLoop(tenantID, devServerID, entry)
	}
	entry.refCount++
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		entry.refCount--
		if entry.refCount <= 0 && entry.isDone() {
			delete(g.waiters, k)
		}
		g.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-entry.doneCh:
		if entry.connected {
			return nil
		}
		if entry.err != nil {
			return entry.err
		}
		return apperrors.New(apperrors.KindUnavailable, "CODEINTEL_DEV_SERVER_OFFLINE", "dev server did not reconnect within budget", nil)
	}
}

func (e *gateEntry) isDone() bool {
	select {
	case <-e.doneCh:
		return true
	default:
		return false
	}
}

func (g *reconnectGate) pollLoop(tenantID, devServerID string, entry *gateEntry) {
	deadline := time.Now().Add(g.budget)
	intervals := []time.Duration{
		500 * time.Millisecond,
		1000 * time.Millisecond,
		2000 * time.Millisecond,
		2000 * time.Millisecond,
	}
	intervalIdx := 0

	for {
		if time.Now().After(deadline) {
			entry.mu.Lock()
			entry.connected = false
			entry.err = apperrors.New(apperrors.KindUnavailable, "CODEINTEL_DEV_SERVER_OFFLINE", "dev server did not reconnect within budget", nil)
			close(entry.doneCh)
			entry.mu.Unlock()
			return
		}

		// Base interval with +/- 20% jitter
		base := intervals[intervalIdx]
		if intervalIdx < len(intervals)-1 {
			intervalIdx++
		}
		jitterFactor := 0.8 + 0.4*rand.Float64()
		sleepDur := time.Duration(float64(base) * jitterFactor)

		remaining := time.Until(deadline)
		if sleepDur > remaining {
			sleepDur = remaining
		}

		time.Sleep(sleepDur)

		// Probe IsDevServerConnected using caller's tenant context
		probeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		callCtx, err := withTenantMetadata(tenant.WithTenantID(probeCtx, tenantID))
		if err == nil {
			resp, err := g.client.IsDevServerConnected(callCtx, &infrafleetv1.IsDevServerConnectedRequest{
				DevServerId: devServerID,
			})
			cancel()
			if err == nil && resp.GetConnected() {
				entry.mu.Lock()
				entry.connected = true
				close(entry.doneCh)
				entry.mu.Unlock()
				return
			}
		} else {
			cancel()
		}
	}
}
