# SOL-AG-04: Compose the already-correct `ResolveProvider` cascade with SOL-AG-01/02's spawn/kill into one switch-account saga, driven by SOL-AG-05's rate-limit signal

**Resolves:** [BUG-AG-04](../BUG-AG-04-switch-account-partial.md)
**Service:** `infra-fleet-service` (new orchestrating usecase) + `ai-provider-service` (caller only, no code change)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new `SwitchAgentAccount` RPC)
- `backend-go/services/infra-fleet-service/internal/usecase/switch_agent_account.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (reuses SOL-AG-01's `AIProviderResolverClient`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` (extended: `agent.switchAccount`, plus the `agent:rateLimited` push wired in SOL-AG-05)
**Status:** 📋 Proposed — not yet implemented; **inherits SOL-AG-01's credential-injection blocker** (a switch to any keyed provider hits the exact same structural gap, see below)

---

## Design rationale (grounded in TDD)

### This is a saga over three already-designed pieces, not new domain logic

BL-AG-04's flow is explicitly a composition: step 4 cites "BL-AG-02:
conn.call('agent.kill'...)", "BL-AG-01: conn.call('agent.spawn'...)" and
"BL-AG-03: resume session nếu compatible" by name. `05-data-architecture.md`'s
**synchronous saga** pattern is the exact right shape: "service A needs
service B to also succeed before A's operation can be considered complete,
and the caller is waiting" — here all three steps live in the same service
(`infra-fleet-service`) plus one read-only cross-service call
(`ai-provider-service.Resolve`, already sub-20ms per
`ai-provider-service.md` §8), so this is a saga **within** one service's
usecase layer, not a cross-service compensable-steps saga in the stricter
sense — but the same "if a later step fails, the earlier step's effect
should be handled explicitly, not left dangling" discipline applies.

```go
type SwitchAgentAccount struct {
	sessions AgentSessionRepository
	kill     *KillAgentSession        // SOL-AG-02
	resolve  AIProviderResolverClient // SOL-AG-01 — ai-provider-service.Resolve, already correct/unmodified
	start    *StartAgentSession       // SOL-AG-01
	resume   *ResumeAgentSession      // SOL-AG-03
}

func (uc *SwitchAgentAccount) Execute(ctx context.Context, in SwitchAgentAccountInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	found, current, err := uc.sessions.LatestForWorktree(ctx, tenantID, in.WorktreeID)
	if err != nil || !found {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "no running agent session for this worktree", err)
	}

	// Step (b): BL-AG-02, force kill — a rate-limited agent is not writing
	// a file (BR-AG-06 doesn't apply to a rate-limit-triggered switch the
	// same way it does to a user-initiated kill, but this usecase still
	// goes through KillAgentSession rather than calling KillAgent directly,
	// so the write-lock check — if/when SOL-AG-02's open question resolves
	// to build it — applies uniformly, not just to manual kills).
	if err := uc.kill.Execute(ctx, current.ID, "SIGKILL"); err != nil {
		return domain.AgentSession{}, err // kill failure aborts the switch — do not spawn a second agent on top of a possibly-still-alive one
	}

	// Step (c): BL-AG-04 step 4c — priority cascade, already-correct,
	// unmodified. AccountID explicitly EXCLUDES current.AccountID so a
	// same-provider rate limit doesn't resolve back to the same rate-limited
	// account — see excludeAccountID below.
	accountID, modelID, status, err := uc.resolve.Resolve(ctx, in.UserID, in.ProjectID)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_SWITCH_RESOLVE_FAILED", "failed to resolve a replacement provider account", err)
	}
	if accountID == current.AccountID {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SWITCH_NO_ALTERNATE_ACCOUNT", "provider resolution returned the same account that was just rate limited", nil)
	}

	// Step (d)/(e): BL-AG-04 step 4d/4e — spawn with the new account;
	// resume iff BR-AG-09 compatibility holds (delegated entirely to
	// ResumeAgentSession's own check, not duplicated here).
	if uc.resumeCompatible(ctx, tenantID, current) {
		return uc.resume.Execute(ctx, ResumeAgentSessionInput{
			ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID, Cwd: in.Cwd,
		})
	}
	return uc.start.Execute(ctx, StartAgentSessionInput{
		ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID,
		Cwd: in.Cwd, ModelID: modelID, AccountID: accountID,
	})
}
```

`resumeCompatible` is a thin existence check (does
`current.ResumeProviderSessionID` exist and is it non-expired) — reuses
`ResumeAgentSession`'s own BR-AG-08/09 checks rather than re-implementing
them, so the two bugs' logic can't drift independently.

**`ai-provider-service.Resolve` needs no code change** — BUG-AG-04's own
"What backend-go has" section already confirms the cascade
(`resolve_provider.go:42-89`) is fully correct. The only wiring this
solution adds is a **new caller** with an "exclude this account" nuance
BL-AG-04 implies (switching away from a rate-limited account must not
immediately re-resolve to it) but `ResolveProviderInput`
(`resolve_provider.go:11-17`) has no such field today — a small, additive
extension to that input struct (`ExcludeAccountID string`, threaded into
`ListAccountsFilter`), **not** a change to the cascade order/logic itself.

### Rate-limit detection is SOL-AG-05's job, not duplicated here

BL-AG-04 step 1 ("AgentHookParser parse pattern: rate-limit detected →
emit: agent:rateLimited") is the exact same PTY-output classification
pipeline BUG-AG-05 needs for `waiting`/`completed`/`error` detection —
same input stream (`StreamPty`'s `pty.data`-sourced `agent.output`
notifications), same text-pattern-matching mechanism, different pattern
set. This solution does not build a second classifier; SOL-AG-05's
`AgentOutputClassifier` emits `agent:rateLimited{sessionID, resetAt}` as one
of its recognized signals (BL-AG-04's own `RATE_LIMIT_PATTERNS`, ported
1:1), and `channels_agent.go`'s push wiring (SOL-AG-05) surfaces it to the
renderer, which is what actually drives a user's "switch account" click —
`SwitchAgentAccount` itself is invoked by that explicit user action, not by
the classifier directly (BL-AG-04 step 3 makes the switch an explicit user
choice among three options — switch/switch-provider/wait — never an
automatic action).

### BR-AG-11 — this solution inherits SOL-AG-01's credential blocker unchanged

Every path through `SwitchAgentAccount` ends in a call to `StartAgentSession`
or `ResumeAgentSession`, both of which route through `StartAgentSession`'s
`SpawnAgentInput` (SOL-AG-01) — which, per that solution's "Genuine
architecture gap," cannot populate `agent.spawn`'s `resolvedApiKey` for any
keyed provider today, by design of `credential-broker-service`'s
`ResolveCredential` contract (`credential-broker-service.md:184`: `AI_PROVIDER_KEY`
"never plaintext"). **A switch to a second Anthropic/OpenAI/etc. account is
exactly as blocked as a fresh spawn** — this is not a new instance of the
gap, but it's worth restating here because BL-AG-04's whole premise (switch
*because* the current provider is rate limited) makes this the single
highest-value path to unblock once the `agent/`-side Vault Transit work
(SOL-AG-01) lands: rate-limit recovery is the scenario credential injection
matters most for. Switching between two `localInference` (Ollama) accounts
is unaffected and works today.

## Design — proto

```protobuf
service InfraFleetService {
  // ... existing + SOL-AG-01/02/03's RPCs ...
  rpc SwitchAgentAccount(SwitchAgentAccountRequest) returns (AgentSession);
}

message SwitchAgentAccountRequest {
  string connection_id = 1;
  string worktree_id   = 2;
  string user_id       = 3;
  string project_id    = 4; // threaded into ResolveProvider's cascade
  string cwd            = 5;
}
```

## Design — `ai-provider-service` extension (additive, non-breaking)

```go
// resolve_provider.go — additive field, cascade logic unchanged.
type ResolveProviderInput struct {
	UserID           string
	ProjectID        string
	ExcludeAccountID string // "" = no exclusion (existing callers unaffected)
}
```

`Resolve`'s three-tier loop (`resolve_provider.go:50-82`) gains one
`acc.ID != in.ExcludeAccountID` check inside `firstResolvable`'s caller —
additive, does not touch cascade ORDER, which is the part BUG-AG-04
confirms is already correct and this solution must not regress.

## Design — BR-AG-13 (usage counter reset)

No code needed. `ai_provider.usage_daily` is already keyed by
`(account_id, usage_date)` (`ai-provider-service.md:165-173`) — switching
to a **different** `account_id` inherently starts from that account's own
existing rollup (zero, for an account never used before), never carries
over the rate-limited account's counters. "Reset" in BL-AG-04's sense is
already structurally satisfied by the schema; flagged here so it isn't
mistaken for a missing RPC.

## Test plan

- `usecase/switch_agent_account_test.go`:
  - happy path, no resumable prior session → `kill` then `start.Execute`
    called with a **different** `accountID` than `current.AccountID`.
  - `Resolve` returns the same `accountID` as the one being switched away
    from → `INFRA_SWITCH_NO_ALTERNATE_ACCOUNT`, `start`/`resume` never called.
  - resumable prior session → `resume.Execute` called instead of `start.Execute`.
  - `kill` fails → the saga aborts before calling `Resolve`/`start`
    (regression guard against spawning a second agent on top of a
    possibly-still-alive one).
- `usecase/resolve_provider_test.go` (extended): `ExcludeAccountID` set →
  the excluded account is skipped even when otherwise `Resolvable()`;
  existing cascade-order tests unchanged (regression guard the extension
  didn't reorder tiers).

## References

- `specs/backend-go/bugs/logic-v1/BUG-AG-04-switch-account-partial.md`
- `docs/logic/agent-orchestration/BL-AG-04-switch-account.md` — full flow, BR-AG-11/12/13, `RATE_LIMIT_PATTERNS`
- `specs/backend-go/tdd/services/ai-provider-service.md:224-227` (§7 — `Resolve` makes no cross-service call, still true after this extension), `:250-317` (§9 credential gap, inherited unchanged)
- `specs/backend-go/tdd/services/credential-broker-service.md:184` (`AI_PROVIDER_KEY` never plaintext — the shared blocker)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:100-112` (synchronous saga pattern this usecase follows)
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:1-100` — cascade this solution extends additively
- `specs/backend-go/bugs/logic-v1/solutions/SOL-AG-01-khoi-dong-agent.md`, `SOL-AG-02-dung-agent.md`, `SOL-AG-03-resume-session.md`, `SOL-AG-05-monitor-status.md` — the four pieces this solution composes
