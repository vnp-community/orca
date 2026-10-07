package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

type lruSymbolEntry struct {
	result    ViewResult
	expiresAt time.Time
}

type lruSymbolCache struct {
	mu      sync.RWMutex
	entries map[string]lruSymbolEntry
	maxLen  int
	ttl     time.Duration
}

func newLRUSymbolCache(maxLen int, ttl time.Duration) *lruSymbolCache {
	if maxLen <= 0 {
		maxLen = 64
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &lruSymbolCache{
		entries: make(map[string]lruSymbolEntry),
		maxLen:  maxLen,
		ttl:     ttl,
	}
}

func (c *lruSymbolCache) get(key string) (ViewResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return ViewResult{}, false
	}
	return entry.result, true
}

func (c *lruSymbolCache) put(key string, result ViewResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxLen {
		// Evict an arbitrary expired item or first entry
		for k, e := range c.entries {
			if time.Now().After(e.expiresAt) {
				delete(c.entries, k)
				break
			}
		}
		if len(c.entries) >= c.maxLen {
			for k := range c.entries {
				delete(c.entries, k)
				break
			}
		}
	}
	c.entries[key] = lruSymbolEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *lruSymbolCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]lruSymbolEntry)
}

// CachedViewReader wraps ViewReader with snapshot caching, staleness checks,
// singleflight orchestration, ETag validation, and graceful degradation.
type CachedViewReader struct {
	underlying     ViewReader
	store          SnapshotStore
	probe          *HeadProbe
	evaluator      StalenessEvaluator
	quota          *SnapshotQuotaManager
	collectTimeout time.Duration
	waitTimeout    time.Duration
	symbolCache    *lruSymbolCache
	sf             singleflight.Group
}

// NewCachedViewReader creates a new CachedViewReader instance.
func NewCachedViewReader(
	underlying ViewReader,
	store SnapshotStore,
	probe *HeadProbe,
	quota *SnapshotQuotaManager,
	collectTimeout time.Duration,
	waitTimeout time.Duration,
) *CachedViewReader {
	if collectTimeout <= 0 {
		collectTimeout = 100 * time.Second
	}
	if waitTimeout <= 0 {
		waitTimeout = 20 * time.Second
	}
	if quota == nil && store != nil {
		quota = NewSnapshotQuotaManager(store, DefaultSnapshotMaxBytes, DefaultMaxSnapshotBytesPerBinding, DefaultMaxSnapshotBytesPerTenant, DefaultSnapshotKeepCommits)
	}
	return &CachedViewReader{
		underlying:     underlying,
		store:          store,
		probe:          probe,
		evaluator:      StalenessEvaluator{},
		quota:          quota,
		collectTimeout: collectTimeout,
		waitTimeout:    waitTimeout,
		symbolCache:    newLRUSymbolCache(64, 5*time.Minute),
	}
}

// InvalidateLRU clears the symbol LRU cache.
func (c *CachedViewReader) InvalidateLRU() {
	c.symbolCache.clear()
}

// Get retrieves view results, consulting cache, probe, and falling back gracefully.
func (c *CachedViewReader) Get(ctx context.Context, target AgentTarget, view domain.ViewKind, params any) (ViewResult, error) {
	// 1. STATUS is not cached in DB
	if view == domain.ViewKindStatus {
		return c.underlying.Get(ctx, target, view, params)
	}

	// 2. SYMBOL uses in-memory LRU only
	if view == domain.ViewKindSymbol {
		paramsHash, _ := domain.CanonicalParamsHash(params)
		lruKey := fmt.Sprintf("%s:%s:%s", target.TenantID, target.WorkspaceRoot, paramsHash)
		if cached, ok := c.symbolCache.get(lruKey); ok {
			cached.Meta.Cached = true
			return cached, nil
		}
		res, err := c.underlying.Get(ctx, target, view, params)
		if err == nil {
			c.symbolCache.put(lruKey, res)
		}
		return res, err
	}

	// 3. Graph views
	binding := target.WorkspaceRoot
	paramsHash, err := domain.CanonicalParamsHash(params)
	if err != nil {
		return ViewResult{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", "failed to canonicalize params", err)
	}

	ifNoneMatch := extractIfNoneMatch(params)
	if ifNoneMatch != "" {
		if err := domain.ValidateIfNoneMatch(ifNoneMatch); err != nil {
			return ViewResult{}, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", err.Error(), nil)
		}
	}

	// Probe worktree head
	var probeEntry HeadProbeEntry
	var probeErr error
	if c.probe != nil {
		probeEntry, probeErr = c.probe.Get(ctx, target, binding)
	}

	// Check if probe failed due to dev server offline
	if probeErr != nil && isOfflineError(probeErr) {
		return c.serveOfflineFallback(ctx, target.TenantID, binding, view, paramsHash, probeErr)
	}

	headCommit := probeEntry.HeadCommit

	// Try reading snapshot from store if store is available and headCommit is known
	if c.store != nil && headCommit != "" {
		key := domain.SnapshotKey{
			Tenant:     target.TenantID,
			Binding:    binding,
			View:       view,
			HeadCommit: headCommit,
			ParamsHash: paramsHash,
		}
		snap, err := c.store.Get(ctx, key)
		if err == nil && (snap.ExpiresAt.IsZero() || time.Now().Before(snap.ExpiresAt)) {
			stale, miss := c.evaluator.Evaluate(probeEntry.Sources, probeEntry.HeadCommit, snap.Key.HeadCommit, nil, probeEntry.PendingChanges)
			if !miss {
				clientETag := domain.ClientETag(snap.ContentETag, probeEntry.HeadCommit, stale)
				if ifNoneMatch != "" && ifNoneMatch == clientETag {
					return ViewResult{
						Meta: domain.ResultMeta{
							Cached:      true,
							NotModified: true,
							ETag:        clientETag,
							Commit:      probeEntry.HeadCommit,
							Stale:       stale,
							TotalCount:  int64(snap.TotalCount),
						},
						Data: nil,
					}, nil
				}

				return ViewResult{
					Meta: domain.ResultMeta{
						Cached:     true,
						ETag:       clientETag,
						Commit:     probeEntry.HeadCommit,
						Stale:      stale,
						TotalCount: int64(snap.TotalCount),
					},
					Data: snap.Data,
				}, nil
			}
		}
	}

	// Cache Miss: Singleflight fetch
	sfKey := fmt.Sprintf("%s:%s:%d:%s:%s", target.TenantID, binding, view, headCommit, paramsHash)
	ch := c.sf.DoChan(sfKey, func() (any, error) {
		// Run with detached context and background timeout
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.collectTimeout)
		defer cancel()

		res, err := c.underlying.Get(bgCtx, target, view, params)
		if err != nil {
			return nil, err
		}

		// Estimate size and store snapshot if within limit
		if c.store != nil {
			payloadBytes, errBytes := json.Marshal(res.Data)
			size := int64(len(payloadBytes))
			if errBytes == nil && (c.quota == nil || c.quota.IsPayloadWithinLimit(size)) {
				contentHash := sha256.Sum256(payloadBytes)
				contentETag := domain.ContentETag(hex.EncodeToString(contentHash[:16]))

				snapHead := headCommit
				if snapHead == "" && res.Meta.Commit != "" {
					snapHead = res.Meta.Commit
				}

				snap := domain.Snapshot{
					Key: domain.SnapshotKey{
						Tenant:     target.TenantID,
						Binding:    binding,
						View:       view,
						HeadCommit: snapHead,
						ParamsHash: paramsHash,
					},
					ContentETag: contentETag,
					TotalCount:  int(res.Meta.TotalCount),
					Data:        res.Data,
					CreatedAt:   time.Now(),
					ExpiresAt:   time.Now().Add(7 * 24 * time.Hour), // 7 days TTL per PQ-14
				}
				_ = c.store.Put(bgCtx, snap)
				if c.quota != nil {
					_ = c.quota.CheckAndEvict(bgCtx, target.TenantID, binding)
				}
				res.Meta.ETag = domain.ClientETag(contentETag, snapHead, false)
			}
		}

		if c.probe != nil && res.Meta.Commit != "" {
			c.probe.Observe(target.TenantID, binding, res.Meta.Commit, nil, 0)
		}

		return res, nil
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			if isOfflineError(res.Err) {
				return c.serveOfflineFallback(ctx, target.TenantID, binding, view, paramsHash, res.Err)
			}
			return ViewResult{}, res.Err
		}
		vr := res.Val.(ViewResult)
		if ifNoneMatch != "" && vr.Meta.ETag != "" && vr.Meta.ETag == ifNoneMatch {
			vr.Meta.NotModified = true
			vr.Data = nil
		}
		return vr, nil

	case <-time.After(c.waitTimeout):
		// PQ-13: Return timeout error with retryAfterMs while collection continues in background
		return ViewResult{}, apperrors.New(
			apperrors.KindDeadlineExceeded,
			"CODEINTEL_TIMEOUT",
			"{\"retryAfterMs\":3000,\"inProgress\":true}",
			nil,
		)

	case <-ctx.Done():
		return ViewResult{}, ctx.Err()
	}
}

func (c *CachedViewReader) serveOfflineFallback(
	ctx context.Context,
	tenant string,
	binding string,
	view domain.ViewKind,
	paramsHash string,
	originalErr error,
) (ViewResult, error) {
	if c.store == nil {
		return ViewResult{}, originalErr
	}
	latest, err := c.store.GetLatest(ctx, tenant, binding, view, paramsHash)
	if err != nil {
		return ViewResult{}, originalErr
	}

	clientETag := domain.ClientETag(latest.ContentETag, latest.Key.HeadCommit, true)
	return ViewResult{
		Meta: domain.ResultMeta{
			Cached:     true,
			Stale:      true,
			ETag:       clientETag,
			Commit:     latest.Key.HeadCommit,
			TotalCount: int64(latest.TotalCount),
		},
		Data: latest.Data,
	}, nil
}

func isOfflineError(err error) bool {
	if err == nil {
		return false
	}
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		if ae.Kind == apperrors.KindUnavailable || ae.Code == "CODEINTEL_DEV_SERVER_OFFLINE" || ae.Code == "DEV_SERVER_OFFLINE" {
			return true
		}
	}
	return false
}

func extractIfNoneMatch(params any) string {
	if params == nil {
		return ""
	}
	v := reflect.ValueOf(params)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName("IfNoneMatch")
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
	} else if m, ok := params.(map[string]any); ok {
		if s, ok := m["if_none_match"].(string); ok {
			return s
		}
		if s, ok := m["ifNoneMatch"].(string); ok {
			return s
		}
	}
	return ""
}
