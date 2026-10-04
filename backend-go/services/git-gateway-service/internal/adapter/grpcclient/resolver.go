// Package grpcclient contains git-gateway-service's outbound gRPC client
// adapters for infra-fleet-service — this service's dependency for
// connection resolution and relay dispatch (git-gateway-service.md §7: "the
// only two Go services that talk to the execution plane"). Both adapters
// here (ConnectionResolver in this file, RelayExecutor in
// relay_executor.go) wrap the same *grpc.ClientConn dialed once in
// cmd/server/main.go and forward the caller's tenant identity on every RPC
// via withTenantMetadata (tenant_forwarding.go).
package grpcclient

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// repoDispatchPrefix marks a dispatchExecutor key as a repo id rather than a
// worktree id — MergeWorktreeIntoBase's genuine extension beyond what
// git-gateway-service.md documents (SOL-WT-05): AbortMerge/ConflictOperation/
// ResolveConflict's request messages all name their dispatch field
// worktree_id, but a conflicted MergeBranch happens against the repo's own
// checkout, which has no worktree_id in project-service's bookkeeping. See
// usecase.dispatchKeyForRepo's doc comment.
const repoDispatchPrefix = "repo:"

// ConnectionResolver implements usecase.ConnectionResolver by calling
// infra-fleet-service's ResolveConnection RPC. projects resolves a real
// filesystem path when infra-fleet-service reports no connection (SOL-013,
// see ResolveConnection's doc comment) — added because that branch used to
// echo the dispatch id itself back as RepoPath. reachability additionally
// lets that same branch relay via the repo's own dev server when one is
// bound and reachable (SOL-014/CR-PW-011), instead of always falling back
// to host-local dispatch — see ResolveConnection's doc comment.
type ConnectionResolver struct {
	client       infrafleetv1.InfraFleetServiceClient
	projects     usecase.ProjectClient
	reachability usecase.DevServerReachability
}

// Dial opens a gRPC client connection to infra-fleet-service at addr — the
// same lazy-dial pattern api-gateway's internal/adapter/grpc/dial.go uses
// (grpc.NewClient doesn't block on connect, so infra-fleet-service being
// down doesn't fail this service's startup). Shared by both
// NewConnectionResolver and NewRelayExecutor's callers in cmd/server/main.go
// so a single connection backs both adapters.
//
// Insecure transport credentials — acceptable for local dev only; see
// api-gateway's Dial doc comment for the production mTLS gap this mirrors.
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return conn, nil
}

// NewConnectionResolver wraps an already-constructed infrafleetv1 client —
// used with Dial's connection in cmd/server/main.go, and with a fake client
// in tests. projects backs the !Connected path-resolution fallback (SOL-013);
// reachability backs the !Connected relay-via-dev-server fallback (SOL-014)
// — see ResolveConnection's doc comment.
func NewConnectionResolver(client infrafleetv1.InfraFleetServiceClient, projects usecase.ProjectClient, reachability usecase.DevServerReachability) *ConnectionResolver {
	return &ConnectionResolver{client: client, projects: projects, reachability: reachability}
}

// ResolveConnection asks infra-fleet-service which host owns worktreeID.
//
// worktreeID is NOT the infra-fleet-service connectionId — infra.connections
// has its own UUID primary key (migrations/0002_connections.up.sql), separate
// from the TEXT worktree_id column that stores git-gateway-service's
// "repoId::path" worktree IDs. Sending worktreeID as ConnectionId (this
// method's prior, incorrect behavior) hits infra.connections.id's uuid
// column type and fails with "invalid input syntax for type uuid" for every
// non-UUID worktreeID — live-confirmed as GITGATEWAY_RESOLVE_FAILED's root
// cause on every dev-server-backed worktree's Git panel. WorktreeId is the
// correct request key (mirrors api-gateway's channels_browser.go), routing
// to ResolveConnectionByWorktree's worktree_id-keyed lookup instead. The
// resolved ConnectionID must come from the response (resp.GetConnectionId())
// — infra.connections.id — not be echoed back as worktreeID, since
// RelayExecutor.relay's Relay RPC also requires that real UUID as its own
// ConnectionId (see relay_executor.go's Complete doc comment).
//
// !resp.GetConnected() (BUG-012/CR-PW-010/SOL-013; BUG-015/CR-PW-011/SOL-014):
// this is the "no infra.connections row for this worktree" branch, confirmed
// live to be the system-wide, zero-row norm — not a rare edge case. It used
// to (a) return dispatchID itself as RepoPath (SOL-013 fixed this — see
// resolveLocal) and (b) always dispatch to the LOCAL executor even when the
// worktree's repo has a real, reachable dev server (SOL-014 fixes this):
// resolveLocal now also resolves the repo's DevServerID, and when one is
// bound and DevServerReachability confirms it's reachable, this method
// returns Connected=true with NO real ConnectionID (there is still no
// infra.connections row) but a ctx carrying WithDevServerID/
// WithHiddenTargetID — RelayExecutor.relay()'s shared chokepoint
// (relay_executor.go) already reads DevServerIDFromContext(ctx) FIRST and
// calls RelayByDevServer instead of the ConnectionID-keyed Relay RPC when
// present, exactly the mechanism dispatchExecutorForRepo (ports.go) already
// uses successfully — no new relay mechanism, just reusing it here too.
//
// Returns ctx alongside the answer (interface change, SOL-014) precisely so
// that WithDevServerID/WithHiddenTargetID's effect isn't thrown away —
// context.Context is immutable, so a value added inside this method is
// invisible to the caller unless returned. Every caller (dispatchExecutor
// and ~15 inline callers, ports.go/usecase/*.go) must use the returned ctx
// for any subsequent relay-dependent call.
func (r *ConnectionResolver) ResolveConnection(ctx context.Context, worktreeID string) (context.Context, usecase.ResolvedConnection, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return ctx, usecase.ResolvedConnection{}, err
	}

	// A "repo:"-prefixed key is a repo id, not a worktree id — strip it
	// before resolving; infra-fleet-service's ResolveConnection takes
	// whatever id git-gateway-service is currently dispatching against, so
	// the underlying repo id round-trips through this prefix scheme
	// unchanged. See repoDispatchPrefix's doc comment.
	isRepoScoped := strings.HasPrefix(worktreeID, repoDispatchPrefix)
	dispatchID := strings.TrimPrefix(worktreeID, repoDispatchPrefix)

	resp, err := r.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{
		WorktreeId: dispatchID,
	})
	if err != nil {
		return ctx, usecase.ResolvedConnection{}, fmt.Errorf("grpcclient: ResolveConnection(%q): %w", worktreeID, err)
	}

	if !resp.GetConnected() {
		local, err := r.resolveLocal(ctx, dispatchID, isRepoScoped)
		if err != nil {
			return ctx, usecase.ResolvedConnection{}, fmt.Errorf("grpcclient: resolving local path for %q: %w", worktreeID, err)
		}
		if local.DevServerID != "" {
			reachable, err := r.reachability.IsReachable(ctx, local.DevServerID)
			if err != nil {
				return ctx, usecase.ResolvedConnection{}, fmt.Errorf("grpcclient: checking dev server reachability for %q: %w", worktreeID, err)
			}
			if reachable {
				relayCtx := usecase.WithDevServerID(ctx, local.DevServerID)
				if local.HiddenTargetID != "" {
					relayCtx = usecase.WithHiddenTargetID(relayCtx, local.HiddenTargetID)
				}
				// ConnectionID/Mode deliberately left zero-value here — see
				// SOL-014's "Not fixed by this pass" for what that means for
				// ConnectionID-keyed callers (WatchWorktreeFiles,
				// RemoveWorktree's BR-WT-10 check) and Mode-gated callers
				// (MergeBranch et al.'s ssh-relay-unsupported check).
				return relayCtx, usecase.ResolvedConnection{Connected: true, RepoPath: local.Path, HiddenTargetID: local.HiddenTargetID}, nil
			}
		}
		return ctx, usecase.ResolvedConnection{Connected: false, RepoPath: local.Path}, nil
	}
	return ctx, usecase.ResolvedConnection{
		Connected:    true,
		ConnectionID: resp.GetConnectionId(),
		RepoPath:     resp.GetRepoPath(),
		Mode:         resp.GetDevServer().GetMode(),
		// HiddenTargetID (TASK-BE-EVM-018, BE-SOL-EVM-004 §6c — closes
		// TASK-BE-EVM-015's gap #1): infrafleetv1.ResolveConnectionResponse
		// now carries this field for real.
		HiddenTargetID: resp.GetHiddenTargetId(),
	}, nil
}

// localResolution is resolveLocal's answer: the real filesystem path plus
// (best-effort) the repo's dev server binding, so ResolveConnection can
// decide relay-via-reachability vs. genuinely-local dispatch (SOL-014).
type localResolution struct {
	Path           string
	DevServerID    string
	HiddenTargetID string
}

// resolveLocal resolves dispatchID into a real filesystem path (SOL-013)
// plus its repo's dev server binding, if any (SOL-014), for
// ResolveConnection's !Connected branch. dispatchID is one of three shapes,
// each resolved differently:
//
//  1. An "external" (not project-service-bookkept) worktree's synthesized
//     "repoId::path" composite id — api-gateway's mergeDetectedWorktrees
//     (channels_worktree.go) mints these for `git worktree add`-created
//     worktrees Orca never recorded. The path is already embedded in the id
//     itself (no project.worktrees row to look up for it — that's exactly
//     why this shape exists), but the embedded repoID is still real, so a
//     GetRepo call resolves DevServerID/HiddenTargetID for it. A GetRepo
//     failure here degrades to Path-only (no error) — the path is still
//     valid and locally usable even if the dev-server enrichment fails;
//     losing that enrichment must not regress SOL-013's already-shipped
//     guarantee of a real path.
//  2. isRepoScoped (the "repo:" dispatch prefix, MergeWorktreeIntoBase's
//     repo-level dispatch — see repoDispatchPrefix's doc comment): dispatchID
//     is a repo id, resolved via ONE GetRepo call that provides Path
//     (=Repo.URL, which doubles as an absolute filesystem path for these
//     repos — see domain.RepoInfo's doc comment), DevServerID and
//     HiddenTargetID together. A failure here IS fatal (unlike case 1 and
//     3's second lookup) — GetRepo is this case's ONLY source of Path.
//  3. Otherwise, a real project.worktrees.id — resolved via GetWorktree for
//     Path (fatal on failure — this is the only source of Path for this
//     case), then a SECOND GetRepo(wt.RepoID) call for DevServerID/
//     HiddenTargetID. Same degrade-gracefully-on-failure posture as case 1:
//     a GetRepo failure here must not invalidate the already-resolved Path.
func (r *ConnectionResolver) resolveLocal(ctx context.Context, dispatchID string, isRepoScoped bool) (localResolution, error) {
	if repoID, path, ok := strings.Cut(dispatchID, "::"); ok {
		repo, err := r.projects.GetRepo(ctx, repoID)
		if err != nil {
			return localResolution{Path: path}, nil
		}
		return localResolution{Path: path, DevServerID: repo.DevServerID, HiddenTargetID: repo.HiddenTargetID}, nil
	}
	if isRepoScoped {
		repo, err := r.projects.GetRepo(ctx, dispatchID)
		if err != nil {
			return localResolution{}, fmt.Errorf("resolving repo %q: %w", dispatchID, err)
		}
		return localResolution{Path: repo.URL, DevServerID: repo.DevServerID, HiddenTargetID: repo.HiddenTargetID}, nil
	}
	wt, err := r.projects.GetWorktree(ctx, dispatchID)
	if err != nil {
		return localResolution{}, fmt.Errorf("resolving worktree %q: %w", dispatchID, err)
	}
	repo, err := r.projects.GetRepo(ctx, wt.RepoID)
	if err != nil {
		return localResolution{Path: wt.Path}, nil
	}
	return localResolution{Path: wt.Path, DevServerID: repo.DevServerID, HiddenTargetID: repo.HiddenTargetID}, nil
}
