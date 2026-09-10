# TASK-WF-002-02: `ProviderResolver` — explicit-pin vs. `ai-provider-service` priority chain

**From Solution:** BE-SOL-002
**Priority:** P1
**Service:** `workflow-service` (calls out to `ai-provider-service`)
**File:** `backend-go/services/workflow-service/internal/domain/step.go` (`AgentStepConfig.Provider` field, new), `backend-go/services/workflow-service/internal/usecase/resolve_provider.go` (new), `backend-go/services/workflow-service/internal/usecase/ports.go` (client port addition), `backend-go/services/workflow-service/cmd/server/main.go` (wire `ai-provider-service` client)
**Depends on:** None (independent of TASK-WF-002-01; both feed TASK-WF-002-03)
**Status:** `[x]` DONE

## Execution notes (2026-09-09)

Re-verified live before implementing: `ResolveProvider` RPC confirmed
exact shape (task's own correction of BE-SOL-002's nonexistent
`ResolveForContext` was accurate); `AgentStepConfig` confirmed to have
exactly `{ConnectionID, Prompt, WorktreePath, TrustPreset}` with no
`Provider` field before this change.

**One additional real-code finding beyond what the task flagged:**
`aiprovider.proto` has **no single-account-lookup RPC** (`GetAccount` does
not exist) — full RPC list is `CreateAccount/ResolveProvider/RotateKey/
GetUsageToday/ListAccounts/UpdateAccount/DeleteAccount/WriteCredential/
TestConnection`. `GetAccountStatus` (needed to validate an explicit pin)
is implemented via `ListAccounts` (tenant-scoped server-side) filtered
client-side by account id — the task's own step 4 anticipated needing to
check for this and pick the right existing RPC; `ListAccounts` was it.

Also note (documented in the new adapter's doc comment, not fixed —
different service, out of this task's scope): the real generated Go
interface is `aiproviderv1.AiProviderServiceClient` (lowercase "i" in
"Ai") — confirmed against generated code before using it, since it's easy
to typo as "AIProviderServiceClient". Separately: an existing
git-gateway-service client (`grpcclient/aiprovider_client.go`) calls
`ResolveProvider` without forwarding tenant via outbound gRPC metadata,
relying only on the request's `TenantId` field — but `ai-provider-service`'s
real handler reads tenant via `tenant.RequireTenantID(ctx)` (context
metadata), not the request field. This looks like a pre-existing latent
bug in a different service; out of scope here, not touched, but worth a
human follow-up ticket.

**Changes made (per the task's sketch, one deviation noted above):**
1. `internal/domain/step.go`: added `ProviderPin` + `AgentStepConfig.Provider *ProviderPin`.
2. `internal/usecase/ports.go`: added `AIProviderClient` port.
3. `internal/usecase/resolve_provider.go` (new): `ProviderResolver` verbatim from the task.
4. `internal/adapter/aiproviderclient/` (new package): `client.go`
   (`Dial` + `Client.ResolveForProject`/`GetAccountStatus`) +
   `tenant_forwarding.go` (own `withTenantMetadata` copy, same
   per-package-duplication convention as `projectclient`/`infrafleetclient`).
5. `internal/config/config.go`: added `AIProviderServiceAddr` (env
   `AI_PROVIDER_SERVICE_ADDR`, default `ai-provider-service:9090`,
   matching git-gateway-service's identically-named field).
6. `cmd/server/main.go`: dials ai-provider-service, constructs
   `ProviderResolver` alongside `ServerResolver` (both left
   intentionally-unused-for-now — `_, _ = serverResolver, providerResolver` —
   TASK-WF-002-03 threads both into the step executors' construction).

**Verify output:**
```
go build ./services/workflow-service/...   # clean
go vet   ./services/workflow-service/...   # clean
go test  ./services/workflow-service/internal/usecase/... -run TestProviderResolver -v
  # 6/6 PASS (active pin wins, inactive pin hard-errors, no-pin falls back,
  #           empty-AccountID pin falls back, chain error propagates,
  #           status lookup error propagates)
go test  ./services/workflow-service/...   # full suite, all ok
```

---

## Context — one real divergence from BE-SOL-002's sketch, re-verified against the live proto

BE-SOL-002 sketches `r.aiProvider.ResolveForContext(ctx,
&aiproviderv1.ResolveForContextRequest{UserId: triggeredBy, ProjectId:
projectID})`. Reading `proto/orca/aiprovider/v1/aiprovider.proto` directly:
**there is no `ResolveForContext` RPC.** The real, already-built
priority-chain RPC (confirmed present, matching the "already built for F35"
claim) is:

```protobuf
rpc ResolveProvider(ResolveProviderRequest) returns (ResolveProviderResponse);

message ResolveProviderRequest {
  string tenant_id = 1;
  string user_id = 2;
  string project_id = 3;
}
message ResolveProviderResponse {
  ProviderAccount account = 1;
}
message ProviderAccount {
  string id = 1;
  string tenant_id = 2;
  ProviderType type = 3;
  string status = 4; // active | rotating | revoked
  string credential_ref = 5;
  string dev_server_id = 6;
}
```

This task uses `ResolveProvider`, not the nonexistent `ResolveForContext`
name. Note also that `ProviderAccount` has no `Model` field at all — the
priority-chain RPC resolves which *account* to use, not which model. This
task's `ResolvedProvider.Model` is therefore populated ONLY from an
explicit `cfg.Provider.Model` pin; when resolution falls through to the
`ai-provider-service` chain, `Model` stays empty and `agent.execPrompt`'s
existing "absent model defaults to claude" fallback applies (same
fallback TASK-WF-001-01 already documents).

`AgentStepConfig` (`internal/domain/step.go:64-69`) was re-read directly
and confirmed to have exactly `{ConnectionID, Prompt, WorktreePath,
TrustPreset}` — no `Provider` field exists yet, confirming BE-SOL-002's
claim.

## Changes to make

**1. `internal/domain/step.go`** — add the pin type and field to
`AgentStepConfig`:

```go
// ProviderPin is an AgentStepConfig's optional explicit AI provider
// choice — set, it beats ai-provider-service's own priority-chain
// resolution (user > project > server); unset, ProviderResolver falls
// back to that chain. See usecase.ProviderResolver.
type ProviderPin struct {
	AccountID string `json:"accountId,omitempty"`
	Model     string `json:"model,omitempty"`
}

type AgentStepConfig struct {
	ConnectionID string       `json:"connectionId"`
	Prompt       string       `json:"prompt"`
	WorktreePath string       `json:"worktreePath,omitempty"`
	TrustPreset  string       `json:"trustPreset,omitempty"`
	Provider     *ProviderPin `json:"provider,omitempty"` // NEW
}
```

`Provider` is a pointer so "field entirely absent" (fall back to the
chain) is distinguishable from "present with an empty `AccountID`" (also
falls back — see below) without a second boolean.

**2. `internal/usecase/ports.go`** — add the client port:

```go
// AIProviderClient resolves which provider account a project/user
// context should use, and validates an explicit account pin is still
// active. Backs ProviderResolver.
type AIProviderClient interface {
	// ResolveForProject runs ai-provider-service's existing
	// user > project > server priority chain.
	ResolveForProject(ctx context.Context, userID, projectID string) (accountID string, err error)
	// GetAccountStatus returns the named account's current status
	// ("active" | "rotating" | "revoked") — used to validate an explicit
	// pin before trusting it.
	GetAccountStatus(ctx context.Context, accountID string) (status string, err error)
}
```

**3. `internal/usecase/resolve_provider.go`** (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ResolvedProvider is what ProviderResolver hands back to a step
// executor — Model is only ever populated from an explicit pin (see
// TASK-WF-002-02's Context: ai-provider-service's ResolveProvider RPC
// resolves an account, not a model).
type ResolvedProvider struct {
	AccountID string
	Model     string
}

// ProviderResolver implements the priority BE-SOL-002/workflow-service.md
// §7 specify: an explicit, still-active step.config.provider.accountId
// pin beats ai-provider-service's own priority-chain resolution
// (user > project > server). A pin that fails validation (inactive
// account) is a hard error, never a silent fall-back — a step author who
// pinned a specific account almost certainly wants to know that account
// stopped working, not have a different one silently substituted.
type ProviderResolver struct {
	aiProvider AIProviderClient
}

func NewProviderResolver(aiProvider AIProviderClient) *ProviderResolver {
	return &ProviderResolver{aiProvider: aiProvider}
}

func (r *ProviderResolver) Resolve(ctx context.Context, cfg domain.AgentStepConfig, projectID, triggeredBy string) (ResolvedProvider, error) {
	if cfg.Provider != nil && cfg.Provider.AccountID != "" {
		status, err := r.aiProvider.GetAccountStatus(ctx, cfg.Provider.AccountID)
		if err != nil {
			return ResolvedProvider{}, err
		}
		if status != "active" {
			return ResolvedProvider{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_PROVIDER_PIN_INACTIVE", "pinned provider account is not active", nil)
		}
		return ResolvedProvider{AccountID: cfg.Provider.AccountID, Model: cfg.Provider.Model}, nil
	}
	accountID, err := r.aiProvider.ResolveForProject(ctx, triggeredBy, projectID)
	if err != nil {
		return ResolvedProvider{}, err
	}
	return ResolvedProvider{AccountID: accountID}, nil
}
```

**4. Real `AIProviderClient` adapter** (e.g.
`internal/adapter/aiproviderclient/client.go`, new) wraps
`aiproviderv1.AIProviderServiceClient.ResolveProvider` for both methods —
`ResolveForProject` calls it and returns `resp.GetAccount().GetId()`;
`GetAccountStatus` needs its own lookup. Re-check `aiprovider.proto`'s RPC
list for a single-account-lookup RPC (e.g. `ListAccounts` filtered, or a
dedicated `GetAccount`) before inventing a new one — confirm at
implementation time which of the existing RPCs already returns
`ProviderAccount.status` for one account ID.

**5. `cmd/server/main.go`** — dial `ai-provider-service` (new config
field) and construct `ProviderResolver`, following the same
`infrafleetclient.Dial`-style pattern as TASK-WF-002-01's `project-service`
dial.

## Test plan

- Explicit pin, account active → returns `{AccountID, Model}` from the
  pin, `ai-provider-service`'s chain RPC never called.
- Explicit pin, account inactive/revoked → hard error, chain RPC never
  called (no silent fallback).
- No pin → falls back to `ResolveForProject`, `Model` comes back empty.
- `Provider == nil` and `Provider != nil but AccountID == ""` behave
  identically (both fall back).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestProviderResolver -v
```

Expected: clean build; pin-active/pin-inactive/no-pin cases all pass
against fakes, no real `ai-provider-service` dependency needed for the
unit suite.
