# TASK-027: Wire AIProviderService to Bootstrap (Step 9)

**Phase:** 4 — AI Provider Management  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §2  
**Prerequisite:** TASK-020, TASK-026 (all tests pass)  
**Status:** ✅ DONE — 2026-07-29

---

## Thay đổi trong `src/main/server-bootstrap.ts`

Thêm Step 9 sau step 8 (ProjectService):

```typescript
// 9. AIProviderService + ProviderHealthChecker [v5.0 TDD-16]
const { AIProviderService } = await import('./ai-providers/AIProviderService')
const { ProviderHealthChecker } = await import('./ai-providers/ProviderHealthChecker')
const aiProviderService = new AIProviderService(pool, devServerManager, relayConnectionPool)
const providerHealthChecker = new ProviderHealthChecker()
providerHealthChecker.start(aiProviderService, relayConnectionPool)
console.log('[ServerBootstrap] ✅ AIProviderService + ProviderHealthChecker initialized (v5.0)')
```

Update `return` block + `shutdown()`:
```typescript
// In shutdown():
try {
  providerHealthChecker.stop()
} catch (err) { console.warn('[ServerBootstrap] ProviderHealthChecker stop error:', err) }
```

## Acceptance Criteria

- [x] Step 9 thêm sau step 8 (wire thêm vào Step 11 ✅)
- [x] `aiProviderService` trong return block
- [x] `providerHealthChecker.stop()` trong shutdown
- [x] Existing tests still pass
