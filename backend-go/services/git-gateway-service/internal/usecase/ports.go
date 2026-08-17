// Package usecase holds git-gateway-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// Per git-gateway-service.md §2/§6, this service's only owned logic is
// "resolve host -> dispatch -> translate": every usecase here follows the
// same shape — resolve which host owns the target worktree via
// ConnectionResolver, then dispatch the actual git operation to whichever
// GitExecutor answers for that host (local binary vs. relay to the Dev
// Server Agent), then return the result for the gRPC adapter to translate
// to wire types.
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ResolvedConnection is ConnectionResolver's answer for a worktree: whether
// its owning host is a remote dev server (Connected=true, ConnectionID
// populated) or the same host git-gateway-service itself runs on
// (Connected=false).
//
// RepoPath is the filesystem path GitExecutor operates against. In the full
// target architecture (git-gateway-service.md §7) this is resolved
// separately by project-service's WorktreeResolver and passed alongside the
// connection lookup; this scaffold folds it into ConnectionResolver's
// response to keep the port count to the two named in this service's build
// task (ConnectionResolver, GitExecutor) — see this service's README for the
// project-service integration this still needs.
type ResolvedConnection struct {
	Connected    bool
	ConnectionID string
	RepoPath     string
}

// ConnectionResolver resolves which host owns a worktree, by calling
// infra-fleet-service's ResolveConnection RPC (git-gateway-service.md §2
// step 2, §7). Implemented by internal/adapter/grpcclient in this scaffold
// as a stub that always answers Connected=false — see that package's doc
// comment for what real wiring needs.
type ConnectionResolver interface {
	ResolveConnection(ctx context.Context, worktreeID string) (ResolvedConnection, error)
}

// GitExecutor performs the actual git operation against a resolved worktree
// path. Two implementations exist per git-gateway-service.md §2:
//   - internal/adapter/localgit: a real os/exec-backed implementation used
//     when ConnectionResolver reports Connected=false (host-local case).
//   - internal/adapter/grpcclient: a stub relay-to-Dev-Server-Agent
//     implementation used when Connected=true, via infra-fleet-service's
//     provider-registry client (not wired in this scaffold).
//
// Each usecase is handed both implementations and selects between them
// based on ConnectionResolver's answer — that selection is the "dispatch"
// logic this service actually owns (§2), and is what this package's tests
// exercise with fakes, independent of which GitExecutor implementation is
// real vs. stubbed.
type GitExecutor interface {
	GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
	GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
}

// ErrGenerateCommitMessageNotImplemented is returned by GenerateCommitMessage's
// Execute — per git-gateway-service.md §3.1, this RPC relays to the Dev
// Server Agent's ai.complete rather than calling an LLM from this service,
// and that relay path is not implemented in this scaffold.
var ErrGenerateCommitMessageNotImplemented = errors.New("usecase: GenerateCommitMessage relays to AI inference on the Dev Server Agent; not implemented in this scaffold")

// dispatchExecutor is the resolve-and-dispatch logic every RPC-shaped
// usecase in this package shares: ask ConnectionResolver which host owns
// worktreeID, then return whichever GitExecutor answers for that host plus
// the resolved repo path to operate against. Centralized here so the
// routing behavior — connected=false -> local, connected=true -> relay — is
// implemented and tested exactly once.
func dispatchExecutor(ctx context.Context, resolver ConnectionResolver, local, relay GitExecutor, worktreeID string) (GitExecutor, string, error) {
	conn, err := resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return nil, "", err
	}
	if conn.Connected {
		return relay, conn.RepoPath, nil
	}
	return local, conn.RepoPath, nil
}
