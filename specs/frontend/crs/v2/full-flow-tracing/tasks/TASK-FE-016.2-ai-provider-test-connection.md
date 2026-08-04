# TASK-FE-016.2: Instrument `useAIProviders().testConnection()` (BL-AIP-03, manual trigger)

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-016 §2.4](../solutions/SOL-FE-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001 — tracer `uiAiProviderTestConnFlow` đã đăng ký)
**Status:** ✅ Done (2026-08-04) — implemented as spec'd. `uiAiProviderTestConnFlow` tracer already existed in `tracers.ts` (re-added after a shared-file reset during this session, additive-only). `pnpm tsc --noEmit` clean; `useAIProviders.test.ts` 8/8 passing (3 new tracer cases).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useAIProviders"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useAIProviders", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Click nút 🧪 (`data-testid="test-btn-{id}"`) trên 1 dòng trong `ProviderList` → `testConnection(account.id)` → RPC `aiProvider.testConnection`. Đây là **manual trigger** — phân biệt với health-check cron chạy nền phía backend (`aiProvider:healthCheck`, không có field `trigger`). BL-AIP-02 (resolution cascade) không có UI trigger trực tiếp trong AI Provider Admin UI — không instrument phía FE trong task này.

## File: `src/renderer/src/hooks/useAIProviders.ts` [MODIFY]

```typescript
import { Tracers } from '../../../shared/trace/tracers'

export function useAIProviders(devServerIdOrFilter?: string | AIProvidersFilter) {
  // ...existing filter/accounts/refresh logic unchanged...

  const testConnection = useCallback(async (accountId: string) => {
    const target       = getActiveRuntimeTarget(useAppStore.getState().settings)
    const store        = useAppStore.getState() as any
    const updateStatus = store.updateAIAccountStatus ?? store.updateAccountStatus
    // BL-AIP-03: field `trigger: 'manual'` phân biệt với cron backend
    // (aiProvider:healthCheck không có field này vì nó luôn là background).
    const span = Tracers.uiAiProviderTestConnFlow.start({ accountId, trigger: 'manual' })
    try {
      const result = await callRuntimeRpc<{ ok: boolean; latencyMs: number; error?: string }>(
        target, 'aiProvider.testConnection', { accountId, traceId: span.id }
      )
      if (updateStatus) updateStatus(accountId, result.ok ? 'healthy' : 'invalid')
      span.ok({ accountId, ok: result.ok, latencyMs: result.latencyMs })
      return result
    } catch (err) {
      if (updateStatus) updateStatus(accountId, 'invalid')
      span.fail(err, { accountId })
      throw new Error('Connection test failed')
    }
  }, [])

  // ...existing deleteAccount/createAccount/updateAccount unchanged (không traced —
  // CRUD đơn, không phải BL-AIP-01/02/03)...

  return { accounts, isLoading: isLoadingAccounts, refresh, testConnection, deleteAccount, createAccount, updateAccount }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/useAIProviders.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.uiAiProviderTestConnFlow` bọc `testConnection()` trong `useAIProviders.ts`, field `trigger: 'manual'` phân biệt với cron backend
- [ ] `testConnection()` ok → `span.ok({ accountId, ok: true, latencyMs })`
- [ ] `testConnection()` reject → `span.fail(err, { accountId })`, status account set `'invalid'`
- [ ] `aiProvider.testConnection` RPC nhận `traceId: span.id`
- [ ] BL-AIP-02 (resolution cascade) không có tracer FE nào — xác nhận không có UI trigger trực tiếp trong AI Provider Admin UI
- [ ] Test suite đạt ≥ 3 test case mới: `start({ accountId, trigger: 'manual' })`; ok → `span.ok` với `latencyMs`; reject → `span.fail` + status `'invalid'`
