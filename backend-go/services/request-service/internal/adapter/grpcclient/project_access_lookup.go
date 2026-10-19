package grpcclient

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

const projectAccessTTL = 30 * time.Second

type cachedAccess struct {
	exists  bool
	role    string
	expires time.Time
}

// ProjectAccessLookup asks project-service (ResolveProjectAccess, an internal RPC) whether a project exists and what role
// a user has. The old way, ListMembers as the user, could not tell a missing project from a project the user is not in.
type ProjectAccessLookup struct {
	client projectv1.ProjectServiceClient
	ttl    time.Duration
	now    func() time.Time

	mu    sync.Mutex
	cache map[roleKey]cachedAccess
}

func NewProjectAccessLookup(client projectv1.ProjectServiceClient) *ProjectAccessLookup {
	return &ProjectAccessLookup{client: client, ttl: projectAccessTTL, now: time.Now, cache: map[roleKey]cachedAccess{}}
}

// Lookup returns supported=false when project-service predates the RPC, so callers keep their previous behaviour during
// a staggered rollout. Any other failure is an error: an unverifiable project is never treated as found.
func (l *ProjectAccessLookup) Lookup(ctx context.Context, tenantID, projectID, userID string) (exists bool, role string, supported bool, err error) {
	k := roleKey{tenantID, projectID, userID}
	l.mu.Lock()
	if c, ok := l.cache[k]; ok && l.now().Before(c.expires) {
		l.mu.Unlock()
		return c.exists, c.role, true, nil
	}
	l.mu.Unlock()

	resp, err := l.client.ResolveProjectAccess(asUser(ctx, tenantID, userID), &projectv1.ResolveProjectAccessRequest{ProjectId: projectID, UserId: userID})
	if status.Code(err) == codes.Unimplemented {
		return false, "", false, nil
	}
	if err != nil {
		return false, "", true, err
	}
	l.mu.Lock()
	l.sweepLocked()
	l.cache[k] = cachedAccess{exists: resp.GetExists(), role: resp.GetRole(), expires: l.now().Add(l.ttl)}
	l.mu.Unlock()
	return resp.GetExists(), resp.GetRole(), true, nil
}

// Exists implements usecase.ProjectExistence. An older project-service cannot answer, so the project is assumed to exist
// (the legacy behaviour) rather than blocking every Request during the rollout.
func (l *ProjectAccessLookup) Exists(ctx context.Context, tenantID, projectID string) (bool, error) {
	exists, _, supported, err := l.Lookup(ctx, tenantID, projectID, "")
	if err != nil {
		return false, err
	}
	if !supported {
		return true, nil
	}
	return exists, nil
}

func (l *ProjectAccessLookup) sweepLocked() {
	now := l.now()
	for k, c := range l.cache {
		if !now.Before(c.expires) {
			delete(l.cache, k)
		}
	}
}
