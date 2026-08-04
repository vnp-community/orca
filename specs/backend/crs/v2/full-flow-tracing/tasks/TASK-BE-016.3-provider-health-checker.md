# TASK-BE-016.3: Instrument `ProviderHealthChecker.runCheck()` — 3-way status thật, không phải `healthy`/`degraded` (BL-AIP-03)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-016](../solutions/SOL-BE-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-016.1
**Status:** ✅ Done (2026-08-04) — Implemented exactly as specified: no span if `getAllAccounts()` throws, per-account `ping-account`/`ping-result` steps, 4-counter `ok()`, `onStatusChanged` callback behavior unchanged, no `relayPool` param reintroduced.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "ProviderHealthChecker.runCheck"
```

Symbol đã tồn tại (MODIFY case, private method, chạy theo cron 15 phút/lần). Chạy:

```
gitnexus_impact({ target: "ProviderHealthChecker.runCheck", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận không thêm lại tham số `relayPool` đã bị remove (BUG-AIP-004), và `onStatusChanged` callback (BUG-AIP-003) không bị ảnh hưởng. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `ProviderHealthChecker.runCheck()` bằng span `aiProvider:healthCheck`. **Quan trọng — code thật khác đáng kể so với minh hoạ trong CR gốc:** method thật tên `private async runCheck(service: AIProviderService)` (không phải cycle handler inline trong `setInterval`), KHÔNG có tham số `relayPool` (đã bị remove theo BUG-AIP-004), không throw ở mức account — mỗi account có try/catch riêng, cập nhật 1-trong-3 status thật `active`/`quota_exceeded`/`invalid` (KHÔNG phải nhị phân `healthy`/`degraded` như CR minh hoạ), và có sẵn cơ chế `onStatusChanged` callback (BUG-AIP-003) cần giữ nguyên hành vi.

## File: `src/main/ai-providers/ProviderHealthChecker.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

private async runCheck(service: AIProviderService): Promise<void> {
  let accounts
  try {
    accounts = await service.getAllAccounts()
  } catch (err) {
    console.warn('[ProviderHealthChecker] Failed to fetch accounts:', err)
    return // không tạo span nếu không load được account list — không có gì để đo
  }

  const span = Tracers.aiProviderHealthFlow.start({ accountCount: accounts.length })
  let activeCount = 0, quotaExceededCount = 0, invalidCount = 0, errorCount = 0

  for (const account of accounts) {
    try {
      const oldStatus = account.status
      span.step('ping-account', { accountId: account.id, provider: account.provider })

      const result = await service.testConnection(account.id)
      let newStatus: 'active' | 'quota_exceeded' | 'invalid'
      if (result.ok) newStatus = 'active'
      else if (result.error?.toLowerCase().includes('quota')) newStatus = 'quota_exceeded'
      else newStatus = 'invalid'

      span.step('ping-result', { accountId: account.id, ok: result.ok, latencyMs: result.latencyMs, newStatus })

      const checkedAt = new Date()
      await service.updateAccount(account.id, { status: newStatus, lastHealthCheck: checkedAt })

      if (newStatus === 'active') activeCount++
      else if (newStatus === 'quota_exceeded') quotaExceededCount++
      else invalidCount++

      if (oldStatus !== newStatus && this.onStatusChanged) {
        this.onStatusChanged({ accountId: account.id, oldStatus, newStatus, checkedAt })
      }
    } catch (err) {
      // Per-account failure — KHÔNG làm fail toàn bộ cycle (đúng hành vi hiện tại,
      // xem CR-TRACE-016 §4 "Lưu ý"). Đếm riêng để phân biệt với invalid do provider trả về.
      errorCount++
      console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
    }
  }

  span.ok({ activeCount, quotaExceededCount, invalidCount, errorCount })
}
```

## Verification

```bash
pnpm tsc --noEmit
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.aiProviderHealthFlow` báo cáo đủ 4 counter (`activeCount`/`quotaExceededCount`/`invalidCount`/`errorCount`) trong `ok()`, khớp 3 trạng thái thật `active`/`quota_exceeded`/`invalid`
- [ ] Nếu `service.getAllAccounts()` throw ngay từ đầu → KHÔNG tạo span nào (không có gì để đo)
- [ ] 1 account throw exception trong vòng lặp → `errorCount` tăng, KHÔNG làm fail toàn bộ `span` (vẫn `ok()` ở cuối), không phá vỡ hành vi per-account try/catch hiện có
- [ ] `this.onStatusChanged` callback (BUG-AIP-003) vẫn được gọi đúng khi `oldStatus !== newStatus`, không bị ảnh hưởng bởi việc thêm tracer
- [ ] Không có tham số `relayPool` được thêm lại vào `runCheck()` (đã bị remove theo BUG-AIP-004, không phục hồi trong task này)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
