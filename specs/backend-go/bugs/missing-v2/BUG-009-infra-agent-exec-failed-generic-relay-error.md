# BUG-009: `INFRA_AGENT_EXEC_FAILED` — selecting "System default" Claude/Codex account on a Remote Dev Server always fails

**Service:** `api-gateway` (root cause), surfaced through `infra-fleet-service`
**File:** `internal/adapter/wscompat/channels_accounts.go` (`accountsRelayArgs`, `registerAccountsRelay`) — root cause; `internal/usecase/relay_by_dev_server.go` (`RelayByDevServer.Execute`) — where the generic error code is raised
**Severity:** **High** (revised — see "Update 2026-09-14" below; originally filed as Low/isolated, that was wrong)
**Symptom:**
```
rpc error: code = Internal desc = INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent
```
**Status:** ✅ **Root cause CONFIRMED and FIXED** (2026-09-14) — [SOL-010](./solutions/SOL-010-accounts-relay-preserve-null-accountid.md), TASK-020/021. Code fix applied and unit-tested; **not yet deployed** to `b15.openledger.vn` as of this writing — see "Deploy status" below.

---

## ⚠️ Update 2026-09-14 (supersedes this file's original filing) — the "isolated race" theory was WRONG

This bug was originally filed the same day with a **different, incorrect** hypothesis: a rare check-then-act race between `IsDevServerConnected` and `RelayByDevServer` (2 occurrences observed in 7 days, judged non-recurring, filed as monitor-only). That original evidence and reasoning is preserved below under "Original filing (superseded)" for the record — **do not act on it**, it does not describe the real bug.

**What actually happened:** [SOL-009](./solutions/SOL-009-apperrors-optional-cause-logging.md)'s observability fix (logging `AppError`'s wrapped cause server-side) was deployed, and the user reported the error was **still occurring frequently** ("vẫn còn nhiều lỗi"). Re-checking `orca-go-infra-fleet`'s live logs with the new logging in place immediately revealed the **real, deterministic, 100%-reproducible cause** — nothing like a race at all:

```json
{"time":"2026-09-14T09:09:09.076880486Z","level":"ERROR","msg":"apperrors: internal cause (not sent to client)","code":"INFRA_AGENT_EXEC_FAILED","cause":"accounts.selectClaude: accountId is required"}
{"time":"2026-09-14T09:09:09.07690997Z","level":"ERROR","msg":"rpc failed","method":"/orca.infrafleet.v1.InfraFleetService/RelayByDevServer","error":"rpc error: code = Internal desc = INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent",...}
```

**30 occurrences in a 3-hour window** (`docker logs orca-go-infra-fleet --since 3h | grep -c INFRA_AGENT_EXEC_FAILED` → `30`), every single one carrying the identical cause `"accounts.selectClaude: accountId is required"` — this is exactly the observability gap SOL-009 was built to close, and it worked on the very first real recurrence.

## Root Cause — CONFIRMED

**Selecting "System default" for a Claude/Codex account while using a Remote Dev Server always fails.** The chain:

1. **Frontend** (`frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts:233-249`, `selectClaudeProviderAccount`/`selectCodexProviderAccount`): when the account picker's "System default" option is chosen, `selection.accountId` is `null` (see `AccountsPane.tsx:190-198`'s `getClaudeAccountLabel`/`getCodexAccountLabel`, which treat `accountId == null` as the System-default case throughout this file). For a Remote Dev Server target, this sends `{ accountId: null, connectionId }` over the `accounts.selectClaude`/`accounts.selectCodex` wscompat channel — **`null` is the correct, intentional wire value for "deselect."**

2. **Agent** (`agent/src/relay/accounts-handler.ts:326-335`, `parseSelectAccountId`) **explicitly and correctly supports this**:
   ```ts
   function parseSelectAccountId(params) {
     if (!('accountId' in params)) return INVALID
     const { accountId } = params
     if (accountId === null) return null          // ← "System default" — VALID
     return typeof accountId === 'string' && accountId.trim() ? accountId : INVALID
   }
   ```
   And `selectClaudeAccount(accountId)`/`selectCodexAccount(accountId)` (same file, lines 177-191, 296-308) both handle `accountId === null` as "deselect, always valid" — this is a real, working, intentional feature on the agent side.

3. **The bug is entirely in `api-gateway`'s relay hop** (`internal/adapter/wscompat/channels_accounts.go:114-117`, before this fix):
   ```go
   type accountsRelayArgs struct {
       AccountID    string `json:"accountId"`   // ← plain string, not *string
       ConnectionID string `json:"connectionId"`
   }
   ```
   Go's `encoding/json` unmarshals a JSON `null` into a `string` field as the **zero value `""`** — silently, no error. `registerAccountsRelay` (same file, line 131) then re-marshals `map[string]any{"accountId": in.AccountID}` to forward to the agent — by this point, the client's `null` has already been irreversibly collapsed to `""`.

4. **Agent receives `{"accountId": ""}`, not `{"accountId": null}`.** `parseSelectAccountId` correctly treats `""` differently from `null` (see step 2's code again: only `null` short-circuits to valid; a present-but-falsy string hits the `.trim()` check and returns `INVALID`) — so it **correctly rejects** this corrupted request with `"accounts.selectClaude: accountId is required"`.

5. This agent-side `InvalidParams` JSON-RPC error surfaces back through `DevServerAgentClient.Exec` → `RelayByDevServer.Execute` → wrapped generically as `INFRA_AGENT_EXEC_FAILED` (the same wrapper this bug file originally investigated) → the same opaque message the user keeps seeing, every single time anyone picks "System default" on a Remote Dev Server.

**This is 100% deterministic** — not a race, not intermittent. Anyone using a Remote Dev Server who clicks "System default" in the Claude or Codex account picker hits this every time, which explains "vẫn còn nhiều lỗi" (still many errors) after the observability fix alone (which only makes failures diagnosable, not fixed).

## Fix

See [SOL-010](./solutions/SOL-010-accounts-relay-preserve-null-accountid.md) — change `accountsRelayArgs.AccountID` from `string` to `*string`, which round-trips JSON `null` correctly through `json.Marshal`/`json.Unmarshal` instead of collapsing it to `""`. `removeClaude`/`removeCodex` (the other 2 channels sharing this same struct) are unaffected in practice — the frontend never sends a null `accountId` for those (removing "system default" isn't a concept the UI exposes), so this fix only changes behavior for the 2 `select*` channels' null case.

## Deploy status

- **SOL-009 (`infra-fleet-service`, the cause-logging fix) — ✅ deployed** to `b15.openledger.vn` (part of the same `sync-to-server.sh` run that also carried SOL-008/`project-service`). This is precisely how the real root cause below was found: SOL-009's live logging surfaced it on the first real recurrence after deploy.
- **SOL-010 (`api-gateway`, the actual fix for this bug) — ✅ deployed** (2026-09-14, `sync-to-server.sh 2026.09.14-sol010-fix`). Live-verified: `docker logs orca-go-api-gateway` reports `version:"2026.09.14-sol010-fix"`, all 17 services + frontend `Up`, frontend health check passed. Zero `INFRA_AGENT_EXEC_FAILED`/`accountId is required` occurrences in the 15 minutes immediately after deploy — **not yet confirmed by an actual repro attempt** (someone picking "System default" on a Remote Dev Server post-deploy); update this line once that's done.

---

## Original filing (2026-09-14, superseded — kept for the record, do not act on this)

<details>
<summary>Original "check-then-act race" theory — WRONG, click to expand</summary>

Originally filed with this now-disproven theory, based on 2 occurrences observed over 7 days before the observability fix existed:

- Both failed calls completed in 1.4–1.8ms, ruling out a timeout.
- 2 *other* `RelayByDevServer` calls in the same burst succeeded.
- The failing calls were preceded by a successful `IsDevServerConnected` call from the same user, 28ms earlier.
- Hypothesis: the dev server's session flipped from handshaked to not in that 28ms gap, causing `session.call()`'s fast-fail path to fire.

This reasoning was self-consistent given the evidence available at the time, but the evidence was simply the wrong 2 data points — a coincidental timing correlation, not the actual mechanism. The real cause (this file's current content, above) has nothing to do with connection state timing at all; it is a pure data-corruption bug (`null` → `""`) that fires identically every time, unrelated to `IsDevServerConnected`'s timing. No SOL was written for the race theory, so nothing needs to be un-done — this section exists only so a future reader doesn't rediscover and re-investigate the same red herring.

**Lesson worth keeping**: 2 occurrences in a short window is not enough evidence to characterize a bug as "rare" — it's enough to notice it, not enough to diagnose it. The fix (SOL-009) that made this file's *second* finding possible was exactly the right next step when the evidence ran out, rather than shipping a race-condition fix for a bug that wasn't a race.

</details>
