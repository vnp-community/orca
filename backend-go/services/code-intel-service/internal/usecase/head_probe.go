package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

// HeadProbeEntry stores cached head commit and index state in memory.
type HeadProbeEntry struct {
	HeadCommit     string
	Sources        []domain.ToolIndexStatus
	PendingChanges int
	CachedAt       time.Time
}

// HeadProbe caches worktree head commit and status probes for 15s to avoid repeated CLI/RPC calls.
type HeadProbe struct {
	gateway AgentCodeIntelGateway
	ttl     time.Duration
	mu      sync.RWMutex
	cache   map[string]HeadProbeEntry
	sf      singleflight.Group
}

// NewHeadProbe creates a new HeadProbe instance.
func NewHeadProbe(gateway AgentCodeIntelGateway, ttl time.Duration) *HeadProbe {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	return &HeadProbe{
		gateway: gateway,
		ttl:     ttl,
		cache:   make(map[string]HeadProbeEntry),
	}
}

func probeKey(tenant, binding string) string {
	return tenant + ":" + binding
}

// Get retrieves the head probe entry for a given target and binding.
func (p *HeadProbe) Get(ctx context.Context, target AgentTarget, binding string) (HeadProbeEntry, error) {
	if binding == "" {
		binding = target.WorkspaceRoot
	}
	key := probeKey(target.TenantID, binding)

	p.mu.RLock()
	entry, ok := p.cache[key]
	p.mu.RUnlock()

	if ok && time.Since(entry.CachedAt) < p.ttl {
		return entry, nil
	}

	res, err, _ := p.sf.Do(key, func() (any, error) {
		// Double-check under lock inside singleflight
		p.mu.RLock()
		cached, found := p.cache[key]
		p.mu.RUnlock()
		if found && time.Since(cached.CachedAt) < p.ttl {
			return cached, nil
		}

		if p.gateway == nil {
			return HeadProbeEntry{}, fmt.Errorf("gateway is nil")
		}

		raw, err := p.gateway.Status(ctx, target, StatusParams{})
		if err != nil {
			return HeadProbeEntry{}, err
		}

		headCommit := raw.HeadCommit
		var sources []domain.ToolIndexStatus
		var pendingChanges int

		if len(raw.Data) > 0 {
			var sd domain.AgentStatusData
			if err := json.Unmarshal(raw.Data, &sd); err == nil {
				if headCommit == "" {
					if sd.Indexes.GitNexus != nil && sd.Indexes.GitNexus.HeadCommit != "" {
						headCommit = sd.Indexes.GitNexus.HeadCommit
					} else if sd.Indexes.CodeGraph != nil && sd.Indexes.CodeGraph.HeadCommit != "" {
						headCommit = sd.Indexes.CodeGraph.HeadCommit
					}
				}

				if sd.Indexes.GitNexus != nil {
					idx := sd.Indexes.GitNexus
					st := domain.ToolIndexStatus{
						Tool:          "gitnexus",
						HeadCommit:    idx.HeadCommit,
						IndexedCommit: idx.IndexedCommit,
					}
					if idx.IndexedAt != nil {
						st.IndexedAt = *idx.IndexedAt
					}
					if idx.PendingChanges != nil {
						pendingChanges += *idx.PendingChanges
					}
					sources = append(sources, st)
				}
				if sd.Indexes.CodeGraph != nil {
					idx := sd.Indexes.CodeGraph
					st := domain.ToolIndexStatus{
						Tool:       "codegraph",
						HeadCommit: idx.HeadCommit,
					}
					if idx.IndexedAt != nil {
						st.IndexedAt = *idx.IndexedAt
					}
					if idx.PendingChanges != nil {
						pendingChanges += idx.PendingChanges.Added + idx.PendingChanges.Modified + idx.PendingChanges.Removed
					}
					sources = append(sources, st)
				}
			}
		}

		newEntry := HeadProbeEntry{
			HeadCommit:     headCommit,
			Sources:        sources,
			PendingChanges: pendingChanges,
			CachedAt:       time.Now(),
		}

		p.mu.Lock()
		p.cache[key] = newEntry
		p.mu.Unlock()

		return newEntry, nil
	})

	if err != nil {
		return HeadProbeEntry{}, err
	}
	return res.(HeadProbeEntry), nil
}

// Observe updates the probe entry from any response envelope that provides head commit or status.
func (p *HeadProbe) Observe(tenant, binding string, headCommit string, sources []domain.ToolIndexStatus, pendingChanges int) {
	if tenant == "" || binding == "" || headCommit == "" {
		return
	}
	key := probeKey(tenant, binding)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[key] = HeadProbeEntry{
		HeadCommit:     headCommit,
		Sources:        sources,
		PendingChanges: pendingChanges,
		CachedAt:       time.Now(),
	}
}

// Invalidate removes the in-memory entry for a tenant and binding.
func (p *HeadProbe) Invalidate(tenant, binding string) {
	key := probeKey(tenant, binding)
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.cache, key)
}

// StalenessEvaluator evaluates branch A and branch B staleness.
type StalenessEvaluator struct{}

// Evaluate checks:
// Branch A (stale):
//   - pendingChanges > 0
//   - Any source has Commit != headCommit
//   - Current headCommit != cachedHeadCommit
// Branch B (miss):
//   - Tool version or indexedAt in probe differs from cached snapshot
func (s StalenessEvaluator) Evaluate(
	probeSources []domain.ToolIndexStatus,
	probeHead string,
	cachedHead string,
	cachedSources []domain.ToolIndexStatus,
	pendingChanges int,
) (stale bool, miss bool) {
	// Branch B: tool version or indexedAt mismatch between probe and snapshot indicates a miss
	cachedSourceMap := make(map[string]domain.ToolIndexStatus)
	for _, cs := range cachedSources {
		cachedSourceMap[cs.Tool] = cs
	}

	for _, ps := range probeSources {
		if cs, exists := cachedSourceMap[ps.Tool]; exists {
			if ps.Version != "" && cs.Version != "" && ps.Version != cs.Version {
				miss = true
			}
			if !ps.IndexedAt.IsZero() && !cs.IndexedAt.IsZero() && !ps.IndexedAt.Equal(cs.IndexedAt) {
				miss = true
			}
		}
	}

	// Branch A: stale if commit differs or pending changes exist
	if pendingChanges > 0 {
		stale = true
	}
	if probeHead != "" && cachedHead != "" && probeHead != cachedHead {
		stale = true
	}
	for _, ps := range probeSources {
		commit := ps.IndexedCommit
		if commit == "" {
			commit = ps.HeadCommit
		}
		if commit != "" && probeHead != "" && commit != probeHead {
			stale = true
		}
	}

	return stale, miss
}
