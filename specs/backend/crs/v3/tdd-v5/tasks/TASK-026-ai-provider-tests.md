# TASK-026: AI Provider Tests

**Phase:** 4 — AI Provider Management  
**Prerequisite:** TASK-021, TASK-022, TASK-023  
**Status:** ✅ DONE — 2026-07-29

---

## Files cần tạo

### `src/main/ai-providers/__tests__/AIProviderService.test.ts` (≥ 15 tests)

Setup: `SqliteSingleConnectionPool` + `ALL_MIGRATIONS`.

Tests:
1. `createAccount` → returns account with status='pending'
2. `getAccount` → returns null for non-existent
3. `listAccounts` → filters by devServerId
4. `listAccounts` → filters by scope when provided
5. `updateAccount` → updates status field
6. `updateAccount` → updates lastHealthCheck
7. `deleteAccount` → removes account
8. `writeCredentialToDevServer` → calls relay.call('ai.provider.writeCredential')
9. `writeCredentialToDevServer` → does NOT store plaintext
10. `testConnection` → returns { ok: false } when relay fails (no throw)
11. `recordUsage` → creates new record
12. `recordUsage` → adds to existing record (UPSERT)
13. `getUsageToday` → returns { tokens: 0 } for no usage
14. `getUsageToday` → returns accumulated usage
15. `getAllAccounts` → returns all accounts

### `src/main/ai-providers/__tests__/ProviderResolver.test.ts` (≥ 15 tests)

Use mocks.

Tests:
1. User-scope account returned first
2. Project-scope when no user-scope
3. Server-scope when no user/project scope
4. ModelHint filter applied
5. Fallback without modelHint
6. Inactive accounts excluded (status !== 'active')
7. Quota exceeded accounts excluded
8. Throws NO_PROVIDER_AVAILABLE when none found

### `src/main/ai-providers/__tests__/ProviderHealthChecker.test.ts` (≥ 7 tests)

Tests:
1. start → runCheck called immediately
2. Interval set correctly (15 min)
3. stop → interval cleared
4. Active accounts → status updated to 'active'
5. Failed accounts → status updated to 'invalid'
6. Quota error → status 'quota_exceeded'
7. One account fails → others still checked

## Acceptance Criteria

- [x] ≥ 15 AIProviderService tests pass (16 tests ✅)
- [x] ≥ 15 ProviderResolver tests pass (17 tests ✅)
- [x] ≥ 7 ProviderHealthChecker tests pass (10 tests ✅)
- [x] **Total ≥ 37 tests** (43 tests pass ✅)
