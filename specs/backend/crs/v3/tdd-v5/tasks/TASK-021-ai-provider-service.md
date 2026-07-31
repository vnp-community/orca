# TASK-021: AIProviderService

**Phase:** 4 — AI Provider Management  
**Solution ref:** [SOL-V5-003](../solutions/SOL-V5-003-ai-provider.md) §4  
**Prerequisite:** TASK-001 (migration 0008), TASK-002 (RelayConnectionPool), TASK-004 (ai-provider-types.ts)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/ai-providers/AIProviderService.ts`

Implement theo SOL-V5-003 §4. Constructor:
```typescript
constructor(
  private readonly pool: IConnectionPool,
  private readonly devServerManager: DevServerManager,
  private readonly relayPool: RelayConnectionPool
) {}
```

**Public API (12 methods):**
- `createAccount(params)` → `AIProviderAccount`
- `getAccount(accountId)` → `AIProviderAccount | null`
- `listAccounts(devServerId, scope?)` → `AIProviderAccount[]`
- `getAllAccounts()` → `AIProviderAccount[]`
- `updateAccount(accountId, patch)` → `void`
- `deleteAccount(accountId)` → `void`
- `writeCredentialToDevServer(accountId, encryptedBlob, iv)` → `void` (via relay)
- `testConnection(accountId)` → `{ ok, latencyMs, error? }`
- `recordUsage(accountId, tokens, requests, costUsd)` → `void` (UPSERT on date)
- `getUsageToday(accountId)` → `{ tokens, requests, cost }`
- `resolveForProject(devServerId, projectId, userId, modelHint?)` → `AIProviderAccount | null`

**Key constraints:**
- `writeCredentialToDevServer()`: relay.call('ai.provider.writeCredential') — NEVER store plaintext credential on Orca Server
- `testConnection()`: catch error → return `{ ok: false, latencyMs: 0, error }` (non-throwing)
- `recordUsage()`: UPSERT pattern `ON CONFLICT(account_id, date) DO UPDATE SET tokens_used = tokens_used + excluded.tokens_used`
- Column mapping: `dev_server_id` → `devServerId`, `scope_ref_id` → `scopeRefId`, etc.

## Acceptance Criteria

- [x] `AIProviderService` class export
- [x] 12 methods implement
- [x] `writeCredentialToDevServer` does NOT store credential
- [x] `testConnection` non-throwing
- [x] `recordUsage` uses UPSERT
- [x] Không TypeScript errors
