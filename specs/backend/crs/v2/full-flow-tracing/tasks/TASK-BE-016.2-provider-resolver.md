# TASK-BE-016.2: Instrument `ProviderResolver.resolve()` — thuật toán 2-pass thật (BL-AIP-02)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-016](../solutions/SOL-BE-TRACE-016-ai-providers.md)
**CR Ref:** [CR-TRACE-016](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-016.1
**Status:** ✅ Done (2026-08-04) — Implemented exactly per the real 2-pass algorithm (no drift from current source — `ProviderResolver.resolve()` already matched the task doc's description). Added `quota-filter` step, `scope-match` step distinguishing `usedModelHint`, and `reason: 'quota-or-inactive' | 'no-scope-match'` on fail, with the double-fail guard for infrastructure errors.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "ProviderResolver.resolve"
```

Symbol đã tồn tại (MODIFY case) — thuật toán 2-pass thật, được gọi trên mọi lần cần chọn AI provider. Chạy:

```
gitnexus_impact({ target: "ProviderResolver.resolve", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận đúng thuật toán 2-pass thật (không theo pseudo-code tuyến tính của CR gốc). Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `ProviderResolver.resolve()` bằng span `aiProvider:resolve`. **Quan trọng:** code thật dùng thuật toán 2-pass (pass 1 với `modelHint`, pass 2 không `modelHint`) qua `scopePriority = [user, project, server]` — KHÁC với pseudo-code "quota-filter → scope-match" tuyến tính đơn giản trong CR-TRACE-016 gốc. Instrumentation phải bám đúng 2-pass thật, và field `reason` khi fail phải phân biệt rõ `'quota-or-inactive'` (không còn account nào active/trong quota) vs `'no-scope-match'` (còn account nhưng không khớp scope nào ở cả 2 pass).

## File: `src/main/ai-providers/ProviderResolver.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

async resolve(options: ResolveOptions): Promise<AIProviderAccount> {
  const { devServerId, projectId, userId, modelHint } = options
  const span = Tracers.aiProviderResolveFlow.start({ devServerId, projectId, userId, modelHint })

  try {
    const all = await this.service.listAccounts(devServerId)
    const active = all.filter(a => a.status === 'active')

    // Quota filter — điểm rẽ nhánh quan trọng (CR-TRACE-000 §5 rule 3), có network hop
    // ẩn bên trong getUsageToday() (SQLite, nhưng chạy song song N lần — đáng đo)
    const quotaCheckPromises = active
      .filter(a => a.quotaLimitDay > 0)
      .map(async (a) => {
        const usage = await this.service.getUsageToday(a.id)
        return { id: a.id, withinQuota: usage.tokens < a.quotaLimitDay }
      })
    const quotaResults = await Promise.all(quotaCheckPromises)
    const overQuotaIds = new Set(quotaResults.filter(r => !r.withinQuota).map(r => r.id))
    const available = active.filter(a => a.quotaLimitDay === 0 || !overQuotaIds.has(a.id))

    span.step('quota-filter', { totalAccounts: all.length, activeCount: active.length, overQuotaCount: overQuotaIds.size })

    if (available.length === 0) {
      span.fail('NO_PROVIDER_AVAILABLE', { reason: 'quota-or-inactive' })
      throw new Error('NO_PROVIDER_AVAILABLE: no active AI provider accounts within quota')
    }

    const scopePriority: Array<{ scope: AIProviderScope; scopeRefId?: string }> = [
      { scope: 'user', scopeRefId: userId },
      { scope: 'project', scopeRefId: projectId },
      { scope: 'server' },
    ]

    // Pass 1: với modelHint
    if (modelHint) {
      for (const { scope, scopeRefId } of scopePriority) {
        const match = this.findInScope(available, scope, scopeRefId, modelHint)
        if (match) {
          span.step('scope-match', { matchedScope: scope, usedModelHint: true })
          span.ok({ accountId: match.id, scope })
          return match
        }
      }
    }

    // Pass 2: không modelHint (any model)
    for (const { scope, scopeRefId } of scopePriority) {
      const match = this.findInScope(available, scope, scopeRefId, undefined)
      if (match) {
        span.step('scope-match', { matchedScope: scope, usedModelHint: false })
        span.ok({ accountId: match.id, scope })
        return match
      }
    }

    span.fail('NO_PROVIDER_AVAILABLE', { reason: 'no-scope-match' })
    throw new Error('NO_PROVIDER_AVAILABLE: no matching AI provider account found')
  } catch (err) {
    // fail() đã được gọi ở 2 nhánh throw phía trên; nhánh này bắt lỗi hạ tầng khác
    // (vd. listAccounts() ném exception) — tránh double span.fail() nếu err đã trace.
    if (!(err instanceof Error) || !err.message.startsWith('NO_PROVIDER_AVAILABLE')) {
      span.fail(err)
    }
    throw err
  }
}
```

> **Lưu ý:** field `reason` phân biệt rõ 2 nguyên nhân fail đúng theo acceptance criteria CR — `'quota-or-inactive'` (không còn account nào active/trong quota) vs `'no-scope-match'` (còn account nhưng không khớp scope nào ở cả 2 pass).

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

- [ ] `Tracers.aiProviderResolveFlow` phân biệt đúng 2 lý do fail: `quota-or-inactive` vs `no-scope-match`
- [ ] `span.step('scope-match', { matchedScope, usedModelHint })` phân biệt `usedModelHint: true/false` đúng theo pass 1/pass 2
- [ ] `span.step('quota-filter', ...)` báo cáo `totalAccounts`/`activeCount`/`overQuotaCount`
- [ ] Không double `span.fail()` khi lỗi hạ tầng (`listAccounts()` throw) — chỉ fail() 1 lần
- [ ] Review code không thêm `span.step()` cho SQLite SELECT đơn (`listAccounts`, `getUsageToday`)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
