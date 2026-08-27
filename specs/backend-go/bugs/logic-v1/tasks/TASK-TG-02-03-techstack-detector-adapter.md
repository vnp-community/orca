# TASK-TG-02-03: `TechStackDetector` adapter — via `git-gateway-service.ReadFile`

**From Solution:** SOL-TG-02
**Priority:** P2 — enrichment only, `AIDecompose` must never fail because of this
**Service:** `task-service` (client) + `git-gateway-service` (existing `ReadFile` RPC, reused as-is)
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/tech_stack_detector.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

The spec's `collectProjectContext()` reads `package.json`/`go.mod`/etc. off
the target host's filesystem. `git-gateway-service`'s `ReadFile` RPC already
exists (`gitgateway.proto:58-61`) and is the correct port for this — this is
a genuine scope addition (a new `task-service -> git-gateway-service`
dependency edge, absent from `02-microservices-decomposition.md`'s
dependency graph) flagged explicitly, same as `SOL-009` flagged its own
proto extension.

## Changes to make

Add a `TechStackDetector` port to
`backend-go/services/task-service/internal/usecase/ports.go`:

```go
// TechStackDetector probes a project's worktree for common ecosystem
// marker files (package.json, go.mod, ...) to enrich AIDecompose's prompt
// — best-effort by design: a detection failure must never fail Execute
// outright, since this is an enrichment, not a precondition for producing
// a plan at all.
type TechStackDetector interface {
	Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error)
}
```

Add `TechStack` to `backend-go/services/task-service/internal/domain/tech_stack.go` (new):

```go
package domain

// TechStack is TechStackDetector's best-effort result — languages/
// frameworks inferred from marker-file presence, never validated further.
type TechStack struct {
	Languages  []string
	Frameworks []string
}
```

Create `backend-go/services/task-service/internal/adapter/grpcclient/tech_stack_detector.go`:

```go
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
// git-gateway-service's ReadFile RPC — see this file's doc comment in
// SOL-TG-02 for why this is a git-gateway-service dependency, not a new
// file-I/O client. Resolves projectID -> its default worktree_id via
// resolver (same lookup pattern SimpleExecutor uses for worktreePath),
// then probes a fixed candidate list, treating NOT_FOUND as "this
// ecosystem isn't present" rather than an error.
type TechStackDetector struct {
	git      gitgatewayv1.GitGatewayServiceClient
	resolver usecase.ProjectExecutionResolver // reused: worktreePath resolution is the same shape SimpleExecutor already needs
}

func NewTechStackDetector(git gitgatewayv1.GitGatewayServiceClient, resolver usecase.ProjectExecutionResolver) *TechStackDetector {
	return &TechStackDetector{git: git, resolver: resolver}
}

func (d *TechStackDetector) Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error) {
	var stack domain.TechStack
	_, worktreeID, connected, err := d.resolver.ResolveConnection(ctx, tenantID, projectID)
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
```

Note: `ProjectExecutionResolver.ResolveConnection`'s second return value is
currently named `worktreePath` (a filesystem path, not a worktree ID) — a
`git-gateway-service.ReadFile` call needs a `worktree_id`, not a path.
Confirm at implementation time whether `infra-fleet-service`'s
`ResolveConnectionResponse` also carries a worktree ID, or whether this
adapter needs its own `project-service` lookup for it (`WorktreeProvisioner`
in `SOL-TG-04` has the identical open question — resolve both the same way).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestTechStackDetector -v
```

Expected: fake `GitGatewayServiceClient` — `NOT_FOUND` on `package.json`
continues to probe `go.mod`; a real read populates the expected `TechStack`
fields; any detection error still returns a zero-value `TechStack` with a
nil error (never fails the call).
