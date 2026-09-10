// This file implements usecase.TechStackDetector by calling
// git-gateway-service's ReadFile RPC directly on candidate manifest files.
//
// Grounding correction versus BE-SOL-002's own sketch: it assumed a
// connection_id/absolute-path/string-content shaped call, by analogy with
// SimpleExecutor's infra-fleet-service Relay request. The real
// ReadFileRequest/ReadFileResponse (proto/orca/gitgateway/v1/gitgateway.proto:444-451,
// confirmed by direct read) is worktree_id + a worktree-RELATIVE path, and
// the response carries `bytes content` + an `encoding` field ("utf8" |
// "base64"), not a plain string. worktree_id is a logical FK into
// project-service's worktree registry per
// specs/backend-go/tdd/services/git-gateway-service.md:121 — but this
// codebase's established convention (see ProjectExecutionResolver's own doc
// comment: "Like git-gateway-service's worktreeID, task-service's projectID
// IS the infra-fleet-service connectionId") is that this external
// identifier is passed through verbatim across every resource-scoped
// service's RPC, regardless of the field's local name. This file follows
// that same convention for worktree_id rather than adding a project-service
// client to resolve one.
package grpcclient

import (
	"context"
	"path"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// candidateManifests maps a manifest filename to the tech-stack hint it
// implies — deliberately small and coarse (BE-SOL-002 doesn't ask for
// exhaustive framework detection, just enough of a hint to steer the AI
// decompose prompt).
var candidateManifests = map[string]string{
	"package.json":     "node",
	"go.mod":           "go",
	"requirements.txt": "python",
	"Cargo.toml":       "rust",
	"pom.xml":          "java",
	"Gemfile":          "ruby",
}

// TechStackDetector implements usecase.TechStackDetector against
// git-gateway-service's ReadFile RPC. Best-effort throughout: Detect never
// returns an error a caller must handle specially — AIDecompose must never
// fail because tech-stack detection failed or found nothing (see that
// usecase's doc comment).
type TechStackDetector struct {
	gitGateway gitgatewayv1.GitGatewayServiceClient
}

func NewTechStackDetector(gitGateway gitgatewayv1.GitGatewayServiceClient) *TechStackDetector {
	return &TechStackDetector{gitGateway: gitGateway}
}

// Detect probes id (see this file's header comment for why this is the same
// verbatim identifier task-service already calls "project_id" elsewhere,
// not a separately-resolved git-gateway worktree entity) for a small set of
// common manifest files. A ReadFile error (not-found, no such worktree, RPC
// failure) for any single candidate is swallowed — that candidate is simply
// absent from the result, never a Detect-level error. An entirely
// unreachable/unregistered worktree yields an empty (nil) result, not an
// error, matching this port's "AIDecompose must never fail because
// detection failed" contract.
func (d *TechStackDetector) Detect(ctx context.Context, id string) ([]string, error) {
	if id == "" {
		return nil, nil
	}
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, nil //nolint:nilerr // best-effort: see doc comment
	}
	var stack []string
	for filename, hint := range candidateManifests {
		resp, err := d.gitGateway.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{
			WorktreeId: id,
			Path:       path.Base(filename), // worktree-root-relative
		})
		if err != nil || len(resp.GetContent()) == 0 {
			continue
		}
		stack = append(stack, hint)
	}
	return stack, nil
}
