package grpcclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/grpcmw"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// prefetchGit plays git-gateway's PrefetchCreateBase: an answer, a delay (honouring the caller's deadline like a real call) and
// a record of what it was asked.
type prefetchGit struct {
	gitgatewayv1.GitGatewayServiceClient
	err      error
	delay    time.Duration
	asked    []*gitgatewayv1.PrefetchCreateBaseRequest
	metadata metadata.MD
}

func (g *prefetchGit) PrefetchCreateBase(ctx context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
	g.asked = append(g.asked, in)
	g.metadata, _ = metadata.FromOutgoingContext(ctx)
	if g.delay > 0 {
		select {
		case <-time.After(g.delay):
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}
	if g.err != nil {
		return nil, g.err
	}
	return &gitgatewayv1.PrefetchCreateBaseResponse{ResolvedSha: "abc"}, nil
}

func checkerKit(g *prefetchGit, repos ...*projectv1.Repo) *GitBaseRefChecker {
	return NewGitBaseRefChecker(g, &fakeProjects{repos: repos})
}

func TestGitBaseRefChecker_Found(t *testing.T) {
	g := &prefetchGit{}
	got, err := checkerKit(g, &projectv1.Repo{Id: "repo-1"}, &projectv1.Repo{Id: "repo-2"}).Check(ctxWithIdentity(), "proj", "release/x")
	if err != nil || got.Status != usecase.BaseRefFound {
		t.Fatalf("%+v %v", got, err)
	}
	if len(g.asked) != 1 || g.asked[0].GetRepoId() != "repo-1" || g.asked[0].GetBaseRef() != "release/x" {
		t.Fatalf("the first repo of the project is asked about the ref: %+v", g.asked)
	}
	if got := g.metadata.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("the tenant must travel with the call: %v", g.metadata)
	}
}

func TestGitBaseRefChecker_NotFound_CarriesAtMostFiveRankedSuggestions(t *testing.T) {
	g := &prefetchGit{err: status.Error(codes.NotFound,
		`WORKTREE_BASE_REF_NOT_FOUND: branch "rel" not found; available: main, a, b, c, d, release/1.4, release/1.5`)}
	got, err := checkerKit(g, oneRepo).Check(ctxWithIdentity(), "proj", "rel")
	if err != nil || got.Status != usecase.BaseRefNotFound {
		t.Fatalf("%+v %v", got, err)
	}
	if len(got.Suggestions) != 5 || got.Suggestions[0] != "release/1.4" || got.Suggestions[1] != "release/1.5" {
		t.Fatalf("suggestions = %v", got.Suggestions)
	}
}

func TestGitBaseRefChecker_NotFoundWithoutAList(t *testing.T) {
	g := &prefetchGit{err: status.Error(codes.NotFound, `WORKTREE_BASE_REF_NOT_FOUND: branch "x" not found`)}
	got, err := checkerKit(g, oneRepo).Check(ctxWithIdentity(), "proj", "x")
	if err != nil || got.Status != usecase.BaseRefNotFound || len(got.Suggestions) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestGitBaseRefChecker_Unreachable(t *testing.T) {
	cases := map[string]error{
		"gateway unavailable":      status.Error(codes.Unavailable, "GITGATEWAY_DEV_SERVER_UNREACHABLE: dev server ds-1 is offline"),
		"deadline in the gateway":  status.Error(codes.DeadlineExceeded, "WORKTREE_PREFETCH_TIMEOUT: fetching the base ref timed out"),
		"remote could not be read": status.Error(codes.Unavailable, "WORKTREE_PREFETCH_UNAVAILABLE: the repository's remote could not be reached"),
		"dev server not bound":     status.Error(codes.FailedPrecondition, "GITGATEWAY_REPO_HOST_UNBOUND: repo has no host"),
		"unclassified git error":   status.Error(codes.Internal, "WORKTREE_PREFETCH_FAILED: failed to prefetch base ref"),
		"repo unknown to gateway":  status.Error(codes.NotFound, "WORKTREE_REPO_NOT_FOUND: repo does not exist"),
		"plain error":              errors.New("connection reset"),
	}
	for name, cause := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := checkerKit(&prefetchGit{err: cause}, oneRepo).Check(ctxWithIdentity(), "proj", "release/x")
			if err != nil || got.Status != usecase.BaseRefUnreachable || len(got.Suggestions) != 0 {
				t.Fatalf("an outage says nothing about the ref: %+v %v", got, err)
			}
		})
	}
}

func TestGitBaseRefChecker_SlowGatewayIsUnreachableAfterTheTimeout(t *testing.T) {
	g := &prefetchGit{delay: 5 * time.Second}
	c := checkerKit(g, oneRepo)
	c.timeout = 30 * time.Millisecond
	started := time.Now()
	got, err := c.Check(ctxWithIdentity(), "proj", "release/x")
	if err != nil || got.Status != usecase.BaseRefUnreachable {
		t.Fatalf("%+v %v", got, err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("the check must give up at its own timeout, took %v", elapsed)
	}
}

func TestGitBaseRefChecker_TheTimeoutIsTenSeconds(t *testing.T) {
	if got := checkerKit(&prefetchGit{}, oneRepo).timeout; got != 10*time.Second {
		t.Fatalf("timeout = %v", got)
	}
}

func TestGitBaseRefChecker_CallerGivingUpIsAnErrorNotAnAnswer(t *testing.T) {
	ctx, cancel := context.WithCancel(ctxWithIdentity())
	g := &prefetchGit{delay: 5 * time.Second}
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	got, err := checkerKit(g, oneRepo).Check(ctx, "proj", "release/x")
	if !errors.Is(err, context.Canceled) || got.Status != "" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestGitBaseRefChecker_ProjectWithoutRepoOrIdentity_IsUnreachable(t *testing.T) {
	g := &prefetchGit{}
	if got, err := checkerKit(g).Check(ctxWithIdentity(), "proj", "main"); err != nil || got.Status != usecase.BaseRefUnreachable || len(g.asked) != 0 {
		t.Fatalf("a project without a repo cannot be asked: %+v %v asked=%d", got, err, len(g.asked))
	}
	if got, err := checkerKit(g, oneRepo).Check(tenantOnly(), "proj", "main"); err != nil || got.Status != usecase.BaseRefUnreachable || len(g.asked) != 0 {
		t.Fatalf("without the acting user project-service refuses, which is an outage here: %+v %v", got, err)
	}
}

func TestGitBaseRefChecker_NotConfigured(t *testing.T) {
	for name, c := range map[string]*GitBaseRefChecker{
		"no git-gateway":     NewGitBaseRefChecker(nil, &fakeProjects{}),
		"no project-service": NewGitBaseRefChecker(&prefetchGit{}, nil),
		"neither":            NewGitBaseRefChecker(nil, nil),
		"nil checker":        nil,
	} {
		_, err := c.Check(ctxWithIdentity(), "proj", "main")
		if err == nil || !strings.Contains(err.Error(), "REQUEST_BASE_REF_CHECK_NOT_CONFIGURED") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
