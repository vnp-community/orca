package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

// WorktreeAuthorizer checks user permissions on a given worktree.
type WorktreeAuthorizer interface {
	AuthorizeWorktree(ctx context.Context, tenantID, projectID, worktreeRef, action string) (bool, error)
}

type authCacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

// PushAuthorization enforces worktree read permissions for event streaming (TASK-024-08).
// Caches decisions for 10s. Any lookup error results in access denial.
type PushAuthorization struct {
	authz    WorktreeAuthorizer
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[string]authCacheEntry
}

// NewPushAuthorization creates a new PushAuthorization instance.
func NewPushAuthorization(authz WorktreeAuthorizer, cacheTTL time.Duration) *PushAuthorization {
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Second
	}
	return &PushAuthorization{
		authz:    authz,
		cacheTTL: cacheTTL,
		cache:    make(map[string]authCacheEntry),
	}
}

// AuthorizeSelectors validates all requested selectors upfront.
// If any selector is unauthorized, returns PermissionDenied for the entire request.
func (p *PushAuthorization) AuthorizeSelectors(ctx context.Context, tenantID string, selectors []*codeintelv1.WorktreeSelector) error {
	if p.authz == nil {
		return nil
	}
	for _, sel := range selectors {
		if !p.CanRead(ctx, tenantID, sel.ProjectId, sel.WorktreeRef) {
			return apperrors.New(apperrors.KindPermissionDenied, "PERMISSION_DENIED", "not authorized to subscribe to worktree", nil)
		}
	}
	return nil
}

// CanRead checks read access for a specific worktree, using 10s cache. Errors result in denial.
func (p *PushAuthorization) CanRead(ctx context.Context, tenantID, projectID, worktreeRef string) bool {
	if p.authz == nil {
		return true
	}
	key := tenantID + ":" + projectID + ":" + worktreeRef

	p.mu.RLock()
	entry, ok := p.cache[key]
	p.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.allowed
	}

	allowed, err := p.authz.AuthorizeWorktree(ctx, tenantID, projectID, worktreeRef, "read")
	if err != nil {
		// Lookup error = deny access per PQ-11 / TASK-024-08
		return false
	}

	p.mu.Lock()
	p.cache[key] = authCacheEntry{
		allowed:   allowed,
		expiresAt: time.Now().Add(p.cacheTTL),
	}
	p.mu.Unlock()

	return allowed
}
