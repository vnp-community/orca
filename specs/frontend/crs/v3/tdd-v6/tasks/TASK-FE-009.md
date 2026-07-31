# TASK-FE-009: Verify useAIProviders RPC Method Names

**Task ID:** TASK-FE-009
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-003 (Section 3)
**Estimated effort:** 20 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Execution Results

### Audit of original `useAIProviders.ts`

Issues found:

| Issue | Detail |
|-------|--------|
| Missing `getActiveRuntimeTarget` | `callRuntimeRpc` called as `callRuntimeRpc('method', params)` — missing `target` param |
| `testConnection` stores `'active'` | Should be `'healthy'` per TDD-FE-13 |
| No `scope` or `status` filter | Only `devServerId` filter existed |
| No `createAccount` / `updateAccount` helpers | Missing from return value |

### RPC names audit

| Method | Status |
|--------|--------|
| `aiProvider.list` | ✅ CORRECT |
| `aiProvider.create` | Added in new hook |
| `aiProvider.update` | Added in new hook |
| `aiProvider.delete` | ✅ CORRECT |
| `aiProvider.testConnection` | ✅ CORRECT |
| `aiProvider.writeCredential` | Referenced in ProviderForm (correct) |

### Changes made to `useAIProviders.ts`

**Full rewrite** preserving backward compatibility:

1. **`getActiveRuntimeTarget()`** — all RPC calls now pass `target` correctly
2. **Overloaded signature** — supports both old `useAIProviders('srv1')` string and new `useAIProviders({ devServerId: 'srv1', scope: 'server' })` object
3. **`useMemo` filter** — client-side filtering by `devServerId`, `scope`, `status`
4. **`testConnection` status** — `'healthy'` on success (was `'active'`)
5. **Added `createAccount`** — calls `aiProvider.create` with target
6. **Added `updateAccount`** — calls `aiProvider.update` with target

### TypeScript errors (task scope): **0**

---

## Acceptance Criteria

- [x] `aiProvider.list` used for listing
- [x] `aiProvider.create` used for creation (via `createAccount()`)
- [x] `aiProvider.update` used for updates (via `updateAccount()`)
- [x] `aiProvider.delete` used for deletion
- [x] `aiProvider.testConnection` used for testing
- [x] `aiProvider.writeCredential` referenced (in ProviderForm, not hook — correct)
- [x] Filtering by `devServerId`, `scope`, `status` works in returned accounts
- [x] `testConnection` updates store status (`'healthy'`) after successful call
- [x] No TypeScript errors

---

## Output

```
aiProvider.list: CORRECT (no change)
aiProvider.create: ADDED helper createAccount()
aiProvider.update: ADDED helper updateAccount()
aiProvider.testConnection: CORRECT (was 'active' → fixed to 'healthy')
getActiveRuntimeTarget: ADDED to all RPC calls (was missing)
Filter by devServerId: IMPLEMENTED (via useMemo)
Filter by scope: ADDED (via useMemo)
Filter by status: ADDED (via useMemo)
testConnection updates store: YES (was 'active', now 'healthy')
TypeScript errors: 0
```
