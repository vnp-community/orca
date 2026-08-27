# TASK-AG-04-05: [ARCHITECTURE BLOCKER] Switching to a keyed provider account inherits TASK-AG-01-04's Vault Transit gap unchanged

**From Solution:** SOL-AG-04
**Priority:** P0 — this is BL-AG-04's single highest-value path to unblock once the `agent/`-side Vault Transit work lands, since rate-limit recovery is the scenario credential injection matters most for
**Service:** `agent/` (cross-repo) — same root cause as TASK-AG-01-04, restated here because BL-AG-04's whole premise is switching *because* the current provider is rate limited
**File:** none (documentation/verification task — no new backend-go code beyond what TASK-AG-04-03 already ships)
**Depends on:** TASK-AG-01-04, TASK-AG-04-03
**Status:** `[ ]` BLOCKED — needs `agent/` Vault Transit support first (same blocker as TASK-AG-01-04, not a second instance to fix independently)

---

## Context

Every path through `SwitchAgentAccount` (TASK-AG-04-03) ends in a call to `StartAgentSession` or `ResumeAgentSession`, both of which route through `SpawnAgentInput` (TASK-AG-01-02) — which, per TASK-AG-01-04's architecture gap, cannot populate `agent.spawn`'s `resolvedApiKey` for any keyed provider today. **A switch to a second Anthropic/OpenAI/etc. account is exactly as blocked as a fresh spawn.** This is not a new gap to fix — it is the same one, restated at the one call site where it will actually be hit most often in production (a user's whole reason to invoke `SwitchAgentAccount` is that their current keyed provider just rate-limited them).

Switching between two `localInference` (e.g. Ollama) accounts is unaffected — `AccountID` stays empty end to end, so `agent.spawn` never needs a `resolvedApiKey` for that path.

## Changes to make

No code change beyond what TASK-AG-04-03 already ships (its
`translateAgentSpawnError` mapping is inherited transitively through
`StartAgentSession`/`ResumeAgentSession`, since `SwitchAgentAccount` never
calls `agent.spawn` directly). This task's job is to make the inheritance
explicit in the integration test suite rather than something discovered
later in production:

```go
// switch_agent_account_test.go — add alongside TASK-AG-04-03's other cases.
func TestSwitchAgentAccount_InheritsCredentialInjectionBlocker(t *testing.T) {
	// Arrange: current session used a keyed (non-localInference) account;
	// resolve returns a different keyed account; the fake DevServerAgentClient's
	// SpawnAgent returns the agent's "no plaintext resolvedApiKey was
	// provided" error string (same fixture TASK-AG-01-07's test uses).
	//
	// Act: uc.Execute(...)
	//
	// Assert: the returned error is INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE
	// (TASK-AG-01-04's mapping), not a generic internal error — proving the
	// blocker surfaces cleanly through the saga, not as an opaque failure.
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/infra-fleet-service/internal/usecase/... -run TestSwitchAgentAccount_InheritsCredentialInjectionBlocker -v
```

This task's `Status` should only flip to DONE once TASK-AG-01-04's `agent/`
tracking issue and scope decision (option 1 vs. 2) exist — same
sign-off gate, not a separate one.
