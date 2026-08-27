# TASK-AG-01-04: [ARCHITECTURE BLOCKER] `agent/` has no Vault Transit decrypt — keyed-provider agent spawn cannot receive a plaintext API key

**From Solution:** SOL-AG-01
**Priority:** P0 — blocks any keyed-provider (non-`localInference`) agent spawn/resume/switch
**Service:** `agent/` (cross-repo, outside `backend-go`) — this task documents and isolates the gap for `infra-fleet-service`
**File:** `agent/src/relay/agent-credential-store.ts`, `agent/src/relay/agent-spawner.ts`
**Depends on:** none — this task should land (or at least be acknowledged/signed-off) before TASK-AG-01-08 and TASK-AG-04-03 are exercised against any keyed provider
**Status:** `[ ]` BLOCKED — needs `agent/` Vault Transit support first

---

## Context

`ai-provider-service.md` §9 and `credential-broker-service.md`'s
`ResolveCredential` contract both assert the target security model: the Dev
Server Agent decrypts an `AI_PROVIDER_KEY` **locally** via its own Vault
Transit identity — no backend-go service is ever allowed to hold or return a
plaintext key (`credential-broker-service.md:184`, `:437`). But the **real,
current** `agent/` implementation (`agent-credential-store.ts`) only
implements the old two-layer model (browser `SubtleCrypto` blob +
agent-local `scrypt`+AES-256-GCM re-encryption); `readDecryptedKey()` only
strips the second layer and still returns Layer-1 ciphertext the agent
cannot open. `buildAgentEnv()` in `agent-spawner.ts:246-318` requires a
**plaintext** `resolvedApiKey` in `agent.spawn`'s own params and **throws**
if it's absent for any keyed provider. There is no RPC anywhere in
`credential-broker-service`'s proto that can hand backend-go a plaintext key
to forward — this is not a missing wire-up, it is two correct-by-design
constraints that jointly make the field impossible to populate today.

This is not implementable as a backend-go change. It requires either:

1. **(Recommended)** Implement Vault Transit decrypt in
   `agent-credential-store.ts` and consume `credential-broker-service`'s
   already-existing `PushCiphertext` RPC agent-side (nothing currently
   receives it), or
2. Scope `StartAgentSession`/`ResumeAgentSession`/`SwitchAgentAccount` to
   `localInference` accounts only (e.g. Ollama, `apiKeyEnvVar == nil`) until
   (1) lands — this is what TASK-AG-01-08's usecase and its integration test
   actually exercise, since it requires zero credential plumbing.

## Changes to make

This task has no backend-go code change. Its job is to make the blocker
explicit and trackable rather than something a later task silently routes
around:

1. File (or link, if one already exists) a tracking issue in the `agent/`
   repo titled "Implement Vault Transit decrypt for AI provider keys
   (`agent-credential-store.ts`)" describing the gap above, and get
   product/security sign-off on option 1 vs. option 2 as the near-term
   scope.
2. In `backend-go/services/infra-fleet-service/internal/usecase/start_agent_session.go`
   (built in TASK-AG-01-08), add `translateAgentSpawnError` so a spawn
   failure caused by this gap surfaces as a distinguishable error rather
   than a generic internal failure:

```go
// translateAgentSpawnError maps agent.spawn's own credential-injection
// failure messages (agent-spawner.ts's buildAgentEnv, fixed strings) into a
// dedicated apperrors kind — an honest, distinguishable error rather than a
// generic internal failure, until the agent-side Vault Transit work
// (TASK-AG-01-04) lands.
func translateAgentSpawnError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no plaintext resolvedApiKey was provided") ||
		strings.Contains(msg, "no credential found for accountId=") {
		return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE",
			"credential injection for this provider account is not available yet — Dev Server Agent has no Vault Transit decrypt (see TASK-AG-01-04)", err)
	}
	return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SPAWN_AGENT_FAILED", "failed to spawn agent on dev server agent", err)
}
```

3. Add a code comment at `SpawnAgentInput` (TASK-AG-01-02) and at
   `StartAgentSessionRequest` (TASK-AG-01-01) — already present in both —
   cross-referencing this task id so a future contributor doesn't "fix" the
   missing field without reading this task first.

## Verify

```bash
# No independent build target — verified as part of TASK-AG-01-08's
# usecase test suite, which must include this case:
cd /opt/repos/orca/backend-go
go test ./services/infra-fleet-service/internal/usecase/... -run TestStartAgentSession_CredentialInjectionUnavailable -v
```

Expected: a fake `DevServerAgentClient.SpawnAgent` returning the agent's
"no plaintext resolvedApiKey was provided" error string maps to
`INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE`, not a generic internal
error. Sign-off tracked separately (see step 1) — this task's `Status`
should only flip to DONE once the `agent/` tracking issue exists and a
scope decision (option 1 vs. 2) has been recorded, even though the Go-side
error-mapping code can merge independently.
