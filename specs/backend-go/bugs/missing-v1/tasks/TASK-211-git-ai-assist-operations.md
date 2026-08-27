# TASK-211: Add Group E — AI-assist RPCs to `git-gateway-service` (4 methods)

**From Solution:** SOL-032 (Part 2, Group E)
**Priority:** P2 — read-heavy/AI-assist, lowest urgency in SOL-032's phased rollout
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `internal/usecase/ports.go`, `internal/usecase/generate_pull_request_fields.go`, `internal/usecase/discover_commit_message_models.go` (new), `internal/adapter/grpcclient/aiprovider_client.go` (new), `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-209 (reuses `GetDiff`/`History` for context-gathering — `History`'s `GitExecutor` method must already exist)
**Status:** `[x]` DONE — `GeneratePullRequestFields` (reuses the new `gatherFullDiff` composition helper, same pattern as `GenerateCommitMessage`) and `DiscoverCommitMessageModels` (new `AIProviderResolver` port + `grpcclient.AIProviderResolver` calling `ai-provider-service`'s `ResolveProvider` directly, `ai-provider-service` dialed separately in `main.go` via new `AIProviderServiceAddr` config field) both fully implemented — proto, usecases, gRPC adapter, `main.go` wiring. `cancelGenerateCommitMessage`/`cancelGeneratePullRequestFields` correctly NOT implemented as new RPCs (both are synchronous unary calls with nothing server-side to cancel, per this task's own Context section — wiring the WS envelope's own cancellation signal is a separate `wscompat`-core follow-up). `go build`/`go vet`/`go test` clean.

---

## ✅ No contract correction needed

[SOL-032 §0](../solutions/SOL-032-git-channels.md#0--correction-pass-read-before-implementing-anything-below-real-agent-contract-vs-this-docs-original-design)
audited all 34 `git.*` methods against the real agent contract and found
this group needs **no correction at all**. Per its bottom-line summary:
"6 of 34 methods (`generateCommitMessage`, `remoteCommitUrl`,
`remoteFileUrl`, `discoverCommitMessageModels`, and the 2 `cancel*`
methods) are correctly designed as-is and need no fix" — this task's
entire 4-method group (`generatePullRequestFields`,
`discoverCommitMessageModels`, `cancelGenerateCommitMessage`,
`cancelGeneratePullRequestFields`) falls entirely within that
already-correct set:

- `generatePullRequestFields` relays via `ai.complete`, already reachable
  on Part A — no TASK-227 dependency, no shape fix.
- `discoverCommitMessageModels` calls `ai-provider-service` directly, not
  the agent at all — not affected by BUG-036's agent-reachability findings.
- The 2 `cancel*` methods make no agent call whatsoever (synchronous unary
  RPCs, nothing to cancel server-side).

A future reader does not need to re-audit this group — it was already
checked and is fine. The rest of this file is unmodified from its
original design.

## Context

Per SOL-032, this group's 4 methods split into two shapes:

1. **`generatePullRequestFields`** — a dispatch operation following
   `GenerateCommitMessage`'s already-established pattern exactly (gather
   diff context via the same dispatch path, relay to the Dev Server
   Agent's `ai.complete`).
2. **`discoverCommitMessageModels`** — **not** a dispatch operation; it
   lists available AI models/providers, which is `ai-provider-service`'s
   data, not git-gateway-service's. **Scope note not resolved by SOL-032**:
   `ai-provider-service`'s actual proto (`aiprovider.proto`) has no
   account-*listing* RPC today — only `CreateAccount`/`ResolveProvider`/
   `RotateKey`/`GetUsageToday`. This task implements
   `DiscoverCommitMessageModels` against `ResolveProvider` (tenant+user's
   single resolved account) rather than inventing a new
   `ai-provider-service` RPC out of scope for `git-gateway-service`'s own
   build task — it reports the one account `ResolveProvider`'s
   scope-cascade would pick, not every account tenant-wide. If the frontend
   actually needs a full multi-account list, file a follow-up to add
   `ListAccounts` to `ai-provider-service` and widen this usecase then;
   don't block this task on that larger change.
3. **`cancelGenerateCommitMessage`/`cancelGeneratePullRequestFields`** — no
   new RPC. Both `GenerateCommitMessage` and `GeneratePullRequestFields`
   are synchronous unary RPCs — the client blocks for the whole call, so
   there's no server-side job to cancel. Per SOL-032: wire the WS
   envelope's own cancellation signal to cancel the `wscompat` handler's
   `context.Context`, which then cancels the outbound gRPC call and,
   transitively, the `Relay` call to the Dev Server Agent — **no
   git-gateway-service change**, this is entirely a `wscompat`-side
   concern. If `Registry` has no cancellation signal today, that gap is the
   thing to close, and it is **out of scope for this task** — file it as
   its own follow-up rather than building a `Cancel` RPC against an
   operation that isn't actually async server-side.

## Changes to make

### Step 1: Proto

Add to the `GitGatewayService` service block:

```protobuf
  rpc GeneratePullRequestFields(GeneratePullRequestFieldsRequest) returns (GeneratePullRequestFieldsResponse);
  rpc DiscoverCommitMessageModels(DiscoverCommitMessageModelsRequest) returns (DiscoverCommitMessageModelsResponse);
```

Append messages:

```protobuf
message GeneratePullRequestFieldsRequest {
  string worktree_id = 1;
  string base_branch = 2;
}
message GeneratePullRequestFieldsResponse {
  string title = 1;
  string description = 2;
}

message ModelInfo {
  string provider_type = 1; // mirrors orca.aiprovider.v1.ProviderType's string name
  string account_id = 2;
  string status = 3; // active | rotating | revoked
}
message DiscoverCommitMessageModelsRequest {
  string tenant_id = 1;
  string user_id = 2;
}
message DiscoverCommitMessageModelsResponse {
  repeated ModelInfo models = 1; // 0 or 1 entries today — see this RPC's usecase doc comment
}
```

### Step 2: `internal/usecase/ports.go` — new `AIProviderResolver` port

```go
// AIProviderResolver resolves which AI provider account a tenant/user
// would use, by calling ai-provider-service's ResolveProvider RPC —
// DiscoverCommitMessageModels' only data source. Distinct from AICompleter
// (which relays the actual completion call through infra-fleet-service);
// this port talks to ai-provider-service directly, since account
// resolution is metadata, not an execution-plane call.
type AIProviderResolver interface {
	ResolveProvider(ctx context.Context, tenantID, userID string) (providerType, accountID, status string, err error)
}
```

`GeneratePullRequestFields` needs no new port — it reuses `ConnectionResolver`,
`GitExecutor` (via the already-built `GetDiff`/`History` usecases), and
`AICompleter`, exactly like `GenerateCommitMessage`.

### Step 3: Usecases — `internal/usecase/`

`generate_pull_request_fields.go` (same shape as `generate_commit_message.go`):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GeneratePullRequestFieldsInput struct {
	WorktreeID string
	BaseBranch string
}

type PRFields struct {
	Title       string
	Description string
}

const prFieldsPromptPrefix = "Write a pull request title and description for the following diff against the base branch. Reply with the title on the first line and the description on subsequent lines.\n\n"

type GeneratePullRequestFields struct {
	resolver  ConnectionResolver
	getDiff   *GetDiff
	completer AICompleter
}

func NewGeneratePullRequestFields(resolver ConnectionResolver, getDiff *GetDiff, completer AICompleter) *GeneratePullRequestFields {
	return &GeneratePullRequestFields{resolver: resolver, getDiff: getDiff, completer: completer}
}

func (uc *GeneratePullRequestFields) Execute(ctx context.Context, in GeneratePullRequestFieldsInput) (PRFields, error) {
	if in.WorktreeID == "" {
		return PRFields{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return PRFields{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	// Same posture as GenerateCommitMessage: no host-local AI inference
	// fallback exists, so a disconnected worktree is a clear
	// FailedPrecondition.
	if !conn.Connected {
		return PRFields{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_NO_AI_RELAY_CONNECTION", "AI-assisted PR field generation requires a connected dev server for this worktree", nil)
	}
	diff, err := uc.getDiff.Execute(ctx, GetDiffInput{WorktreeID: in.WorktreeID, Staged: false})
	if err != nil {
		return PRFields{}, err
	}
	content, err := uc.completer.Complete(ctx, conn.ConnectionID, prFieldsPromptPrefix+diff.UnifiedDiff)
	if err != nil {
		return PRFields{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_AI_COMPLETE_FAILED", "failed to generate PR fields via AI relay", err)
	}
	return parsePRFields(content), nil
}

// parsePRFields splits ai.complete's response on the first newline: title,
// then description. A response with no newline becomes {title, ""} rather
// than an error — a model that ignores the "title on first line" prompt
// instruction still yields a usable (if imperfect) title.
func parsePRFields(content string) PRFields {
	for i, r := range content {
		if r == '\n' {
			return PRFields{Title: content[:i], Description: content[i+1:]}
		}
	}
	return PRFields{Title: content}
}
```

`discover_commit_message_models.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type DiscoverCommitMessageModelsInput struct {
	TenantID string
	UserID   string
}

type ModelInfo struct {
	ProviderType string
	AccountID    string
	Status       string
}

// DiscoverCommitMessageModels is not a worktree-dispatch operation — it
// answers "what AI account would be used" by calling ai-provider-service
// directly, no ConnectionResolver/GitExecutor involved. See this task's
// Context note on why it returns at most one ModelInfo today (bounded by
// ResolveProvider's single-account answer, not a full account list).
type DiscoverCommitMessageModels struct {
	aiProviders AIProviderResolver
}

func NewDiscoverCommitMessageModels(aiProviders AIProviderResolver) *DiscoverCommitMessageModels {
	return &DiscoverCommitMessageModels{aiProviders: aiProviders}
}

func (uc *DiscoverCommitMessageModels) Execute(ctx context.Context, in DiscoverCommitMessageModelsInput) ([]ModelInfo, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_TENANT_ID", "tenant_id is required", nil)
	}
	providerType, accountID, status, err := uc.aiProviders.ResolveProvider(ctx, in.TenantID, in.UserID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_DISCOVER_MODELS_FAILED", "failed to resolve AI provider account", err)
	}
	if accountID == "" {
		return []ModelInfo{}, nil
	}
	return []ModelInfo{{ProviderType: providerType, AccountID: accountID, Status: status}}, nil
}
```

### Step 4: `AIProviderResolver` implementation — new file `internal/adapter/grpcclient/aiprovider_client.go`

```go
// Package grpcclient — this file adds a second gRPC client dependency
// (ai-provider-service) distinct from infra-fleet-service's Relay client
// RelayExecutor already wraps — DiscoverCommitMessageModels resolves
// account metadata directly, it does not go through the execution-plane
// relay.
package grpcclient

import (
	"context"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

type AIProviderResolver struct {
	client aiproviderv1.AiProviderServiceClient
}

func NewAIProviderResolver(client aiproviderv1.AiProviderServiceClient) *AIProviderResolver {
	return &AIProviderResolver{client: client}
}

func (a *AIProviderResolver) ResolveProvider(ctx context.Context, tenantID, userID string) (providerType, accountID, status string, err error) {
	resp, err := a.client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		TenantId: tenantID, UserId: userID,
	})
	if err != nil {
		return "", "", "", err
	}
	account := resp.GetAccount()
	if account == nil {
		return "", "", "", nil
	}
	return account.GetType().String(), account.GetId(), account.GetStatus(), nil
}
```

### Step 5: gRPC adapter — `internal/adapter/grpc/server.go`

```go
func (s *Server) GeneratePullRequestFields(ctx context.Context, req *gitgatewayv1.GeneratePullRequestFieldsRequest) (*gitgatewayv1.GeneratePullRequestFieldsResponse, error) {
	fields, err := s.generatePullRequestFields.Execute(ctx, usecase.GeneratePullRequestFieldsInput{
		WorktreeID: req.GetWorktreeId(), BaseBranch: req.GetBaseBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.GeneratePullRequestFieldsResponse{Title: fields.Title, Description: fields.Description}, nil
}

func (s *Server) DiscoverCommitMessageModels(ctx context.Context, req *gitgatewayv1.DiscoverCommitMessageModelsRequest) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error) {
	models, err := s.discoverCommitMessageModels.Execute(ctx, usecase.DiscoverCommitMessageModelsInput{
		TenantID: req.GetTenantId(), UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*gitgatewayv1.ModelInfo, 0, len(models))
	for _, m := range models {
		out = append(out, &gitgatewayv1.ModelInfo{ProviderType: m.ProviderType, AccountId: m.AccountID, Status: m.Status})
	}
	return &gitgatewayv1.DiscoverCommitMessageModelsResponse{Models: out}, nil
}
```

Add `generatePullRequestFields *usecase.GeneratePullRequestFields`,
`discoverCommitMessageModels *usecase.DiscoverCommitMessageModels` fields
to `Server` and 2 params to `New`.

### Step 6: Composition root — `cmd/server/main.go`

Dial `ai-provider-service` alongside the existing `infra-fleet-service`
dial:

```go
	aiProviderConn, err := grpcclient.Dial(cfg.AIProviderServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing ai-provider-service: %w", err)
	}
	defer func() { _ = aiProviderConn.Close() }()
	aiProviderClient := aiproviderv1.NewAiProviderServiceClient(aiProviderConn)
	aiProviderResolver := grpcclient.NewAIProviderResolver(aiProviderClient)

	generatePullRequestFieldsUC := usecase.NewGeneratePullRequestFields(resolver, getDiffUC, relay)
	discoverCommitMessageModelsUC := usecase.NewDiscoverCommitMessageModels(aiProviderResolver)
```

Add `AIProviderServiceAddr` to this service's `internal/config` struct
(mirroring `InfraFleetServiceAddr`'s existing field) and its
`deploy/`/`docker-compose.yml` env wiring. Extend `gitgatewaygrpc.New(...)`
with `generatePullRequestFieldsUC, discoverCommitMessageModelsUC`.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/git-gateway-service
go build ./... && go vet ./...
```
