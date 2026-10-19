package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Mã lỗi khi git-gateway không xác định được nơi chạy git của một repo.
// Frontend nhận dạng bằng chuỗi con (REPO_HOST_UNBOUND / DEV_SERVER_UNREACHABLE).
const (
	CodeRepoHostUnbound      = "GITGATEWAY_REPO_HOST_UNBOUND"
	CodeDevServerUnreachable = "GITGATEWAY_DEV_SERVER_UNREACHABLE"
	CodeHostCheckFailed      = "GITGATEWAY_HOST_CHECK_FAILED"
)

// RepoHostRouting is DevServerReachability plus the deployment's answer to
// "may a repo with no dev server run on this process's own filesystem?".
// Why not fold the flag into DevServerReachability: reachability is a pure
// infra-fleet read; local-execution is a deployment policy.
type RepoHostRouting interface {
	DevServerReachability
	LocalReposAllowed() bool
}

// RepoHostRouter composes a reachability adapter with the local-repo flag.
// Docker deployments leave the flag off: git-gateway's container cannot see
// the user's disk, so running git "locally" there only yields chdir errors.
type RepoHostRouter struct {
	reachability DevServerReachability
	allowLocal   bool
}

func NewRepoHostRouter(reachability DevServerReachability, allowLocalRepos bool) *RepoHostRouter {
	return &RepoHostRouter{reachability: reachability, allowLocal: allowLocalRepos}
}

func (r *RepoHostRouter) IsReachable(ctx context.Context, devServerID string) (bool, error) {
	return r.reachability.IsReachable(ctx, devServerID)
}

func (r *RepoHostRouter) LocalReposAllowed() bool { return r.allowLocal }

// ErrRepoHostUnbound: repo has no dev server and local repos are not allowed.
func ErrRepoHostUnbound() *apperrors.AppError {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeRepoHostUnbound,
		"this repository is not bound to a dev server, and this deployment cannot run git on its own filesystem; bind it to the dev server where it lives (repo.rebindDevServer) and retry", nil)
}

// ErrDevServerUnreachable: the repo's dev server has no live agent. Retryable.
func ErrDevServerUnreachable(devServerID string) *apperrors.AppError {
	return apperrors.New(apperrors.KindUnavailable, CodeDevServerUnreachable,
		"dev server "+devServerID+" is offline or its agent just disconnected; check that the dev server agent is running and retry", nil)
}

// ErrHostCheckFailed: the host could not be determined (transient lookup error).
func ErrHostCheckFailed(cause error) *apperrors.AppError {
	return apperrors.New(apperrors.KindUnavailable, CodeHostCheckFailed,
		"could not determine which host this repository lives on; retry shortly", cause)
}

// HostDispatchError returns the host-routing AppError found anywhere in err's
// chain, or nil. Usecases wrap dispatch failures in generic *_FAILED codes;
// the gRPC adapter uses this to surface the actionable inner code instead.
func HostDispatchError(err error) *apperrors.AppError {
	for e := err; e != nil; {
		var ae *apperrors.AppError
		if !errors.As(e, &ae) {
			return nil
		}
		switch ae.Code {
		case CodeRepoHostUnbound, CodeDevServerUnreachable, CodeHostCheckFailed:
			return ae
		}
		e = ae.Err
	}
	return nil
}

// RouteRepoHost is the single place deciding where a repo-scoped operation
// runs: relay (true) or local (false). Never falls back silently — an
// unreachable dev server must not turn into git running in the container.
func RouteRepoHost(ctx context.Context, routing RepoHostRouting, devServerID string) (relay bool, err error) {
	if devServerID == "" {
		if routing.LocalReposAllowed() {
			return false, nil
		}
		return false, ErrRepoHostUnbound()
	}
	reachable, err := routing.IsReachable(ctx, devServerID)
	if err != nil {
		return false, ErrHostCheckFailed(err)
	}
	if !reachable {
		return false, ErrDevServerUnreachable(devServerID)
	}
	return true, nil
}
