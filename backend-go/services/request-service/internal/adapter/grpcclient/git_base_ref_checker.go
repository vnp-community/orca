package grpcclient

import (
	"context"
	"log/slog"
	"time"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// baseRefCheckTimeout bounds the whole check: PrefetchCreateBase fetches from the remote, which over SSH can be slow, and a
// person is waiting on the request that carries the ref.
const baseRefCheckTimeout = 10 * time.Second

// GitBaseRefChecker answers usecase.BaseRefChecker with git-gateway's PrefetchCreateBase on the project's first repo.
// Only an explicit "this ref does not exist" is NotFound. Everything else that stops it from asking (project or gateway down,
// dev server offline, timeout, an error git-gateway does not classify) is Unreachable, so a person is never refused for an
// outage; the worktree creation checks the ref again with the same words.
type GitBaseRefChecker struct {
	git      gitgatewayv1.GitGatewayServiceClient
	projects projectv1.ProjectServiceClient
	timeout  time.Duration
}

var _ usecase.BaseRefChecker = (*GitBaseRefChecker)(nil)

// NewGitBaseRefChecker accepts nil clients (a deployment without project-service or git-gateway): Check then fails with
// REQUEST_BASE_REF_CHECK_NOT_CONFIGURED, which the use cases surface instead of storing a ref nobody could check.
func NewGitBaseRefChecker(git gitgatewayv1.GitGatewayServiceClient, projects projectv1.ProjectServiceClient) *GitBaseRefChecker {
	return &GitBaseRefChecker{git: git, projects: projects, timeout: baseRefCheckTimeout}
}

func (c *GitBaseRefChecker) Check(ctx context.Context, projectID, baseRef string) (usecase.BaseRefCheck, error) {
	if c == nil || c.git == nil || c.projects == nil {
		return usecase.BaseRefCheck{}, domain.ErrRequestBaseRefCheckNotConfigured()
	}
	started := time.Now()
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res := c.ask(cctx, projectID, baseRef)
	if ctx.Err() != nil {
		return usecase.BaseRefCheck{}, ctx.Err() // the caller gave up; that is not an answer about the ref
	}
	slog.InfoContext(ctx, "base ref checked", slog.String("project_id", projectID), slog.String("result", string(res.Status)),
		slog.Int64("elapsed_ms", time.Since(started).Milliseconds()))
	return res, nil
}

func (c *GitBaseRefChecker) ask(ctx context.Context, projectID, baseRef string) usecase.BaseRefCheck {
	unreachable := func(why error) usecase.BaseRefCheck {
		slog.WarnContext(ctx, "base ref could not be checked", slog.String("project_id", projectID), slog.String("grpc_code", status.Code(why).String()))
		return usecase.BaseRefCheck{Status: usecase.BaseRefUnreachable}
	}
	repoID, err := defaultRepoID(ctx, c.projects, projectID)
	if err != nil {
		return unreachable(err)
	}
	gctx, err := withTenantMetadata(ctx)
	if err != nil {
		return unreachable(err)
	}
	_, err = c.git.PrefetchCreateBase(gctx, &gitgatewayv1.PrefetchCreateBaseRequest{RepoId: repoID, BaseRef: baseRef})
	if err == nil {
		return usecase.BaseRefCheck{Status: usecase.BaseRefFound}
	}
	if status.Code(err) == codes.NotFound {
		if suggestions, missing := gatewayBaseRefMissing(err, baseRef); missing {
			return usecase.BaseRefCheck{Status: usecase.BaseRefNotFound, Suggestions: suggestions}
		}
	}
	return unreachable(err)
}
