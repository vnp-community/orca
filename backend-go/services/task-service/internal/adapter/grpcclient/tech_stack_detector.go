package grpcclient

import (
	"context"
	"strings"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// techStackCandidate is one ecosystem marker file probed via ReadFile.
type techStackCandidate struct {
	path  string
	parse func(content []byte, out *domain.TechStack)
}

var techStackCandidates = []techStackCandidate{
	{"package.json", func(content []byte, out *domain.TechStack) {
		out.Languages = append(out.Languages, "JavaScript/TypeScript")
		if strings.Contains(string(content), `"react"`) {
			out.Frameworks = append(out.Frameworks, "React")
		}
	}},
	{"go.mod", func(_ []byte, out *domain.TechStack) { out.Languages = append(out.Languages, "Go") }},
	{"pom.xml", func(_ []byte, out *domain.TechStack) { out.Languages = append(out.Languages, "Java") }},
	{"pyproject.toml", func(_ []byte, out *domain.TechStack) { out.Languages = append(out.Languages, "Python") }},
	{"Cargo.toml", func(_ []byte, out *domain.TechStack) { out.Languages = append(out.Languages, "Rust") }},
}

// TechStackDetector implements usecase.TechStackDetector against
// git-gateway-service's ReadFile RPC — see SOL-TG-02 for why this is a
// git-gateway-service dependency, not a new file-I/O client. Resolves
// projectID -> its connection's worktree_id via resolver (infra-fleet-service's
// ResolveConnectionResponse.worktree_id — the SAME resolve call
// SimpleExecutor already makes for worktreePath, widened by TASK-TG-02-03
// to also surface worktree_id since git-gateway-service.ReadFile needs an
// ID, not a filesystem path), then probes a fixed candidate list, treating
// NOT_FOUND (and any other read error) as "this ecosystem isn't present"
// rather than a hard failure.
type TechStackDetector struct {
	git      gitgatewayv1.GitGatewayServiceClient
	resolver usecase.ProjectExecutionResolver
}

func NewTechStackDetector(git gitgatewayv1.GitGatewayServiceClient, resolver usecase.ProjectExecutionResolver) *TechStackDetector {
	return &TechStackDetector{git: git, resolver: resolver}
}

func (d *TechStackDetector) Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error) {
	var stack domain.TechStack
	_, _, worktreeID, connected, err := d.resolver.ResolveConnection(ctx, tenantID, projectID)
	if err != nil || !connected || worktreeID == "" {
		return stack, nil // best-effort: no worktree to probe is not an error
	}
	for _, candidate := range techStackCandidates {
		resp, err := d.git.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{WorktreeId: worktreeID, Path: candidate.path})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			continue // best-effort: any other read error also just skips this candidate, never fails Detect
		}
		candidate.parse(resp.GetContent(), &stack)
	}
	return stack, nil
}
