# TASK-BE-000: Thêm `resume` option cho `Tracer.start()`

**Phase:** 0 — Core API (shared, cross-domain)
**SOL Ref:** [CR-TRACE-000 §3.1](../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md) (không có SOL-BE riêng — đây là blocker chung, implement 1 lần duy nhất)
**CR Ref:** [CR-TRACE-000](../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md)
**Prerequisite:** Không có — đây là root task của toàn bộ cây (tất cả 30 solution ở 3 domain agent/backend/frontend đều phụ thuộc task này)
**Status:** ✅ Done (2026-08-03) — `Tracer.start(fields?, resume?)` implemented, backward-compatible; `src/shared/trace/index.test.ts` created (4 test cases); `gitnexus_impact` confirmed HIGH blast radius (114 symbols, 16 direct callers) but change is purely additive.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

**⚠️ CẢNH BÁO — HIGH BLAST RADIUS:** `Tracer.start()` (`src/shared/trace/index.ts`) là symbol có fan-in cao nhất trong toàn bộ 45 task của bộ full-flow-tracing này — TẤT CẢ 30 solution ở cả 3 domain (agent/backend/frontend) của CR-TRACE-001 → 019 đều gọi xuyên qua `createTracer()` → `Tracer.start()`. Đây là task đầu tiên (Phase 0); mọi task khác trong `tasks/` đều phụ thuộc trực tiếp hoặc gián tiếp vào thay đổi này.

```bash
codegraph explore "Tracer.start"
```

Symbol đã tồn tại (MODIFY case) — BẮT BUỘC chạy thêm:

```
gitnexus_impact({ target: "Tracer.start", direction: "upstream" })
```

Đọc kỹ toàn bộ báo cáo blast radius (mọi caller trực tiếp, mọi process/execution flow bị ảnh hưởng, risk level) TRƯỚC KHI sửa. Dù thay đổi được thiết kế 100% backward-compatible (`resume` là tham số optional mới, không đổi chữ ký hiện có), fan-in cực lớn gần như chắc chắn khiến risk level được báo HIGH hoặc CRITICAL — đây KHÔNG phải lý do để bỏ qua cảnh báo. Trình bày blast radius cụ thể và xác nhận rõ ràng với người dùng trước khi tiến hành sửa `src/shared/trace/index.ts`.

## Mô tả

`src/shared/trace/index.ts` là code isomorphic (chạy được cả Node.js lẫn browser). Hiện tại `Tracer.start(fields?)` luôn tự sinh `id` ngẫu nhiên qua `shortId()` nội bộ, không có cách nào để một layer downstream (Main, Relay, Agent, Browser...) "tiếp nối" cùng một span `id` đã được tạo ở layer trước đó khi nó nhận `traceId` từ wire envelope (xem CR-TRACE-000 §3.2/§3.3). Task này mở rộng chữ ký `start()` để nhận thêm tham số `resume?: { id: string }` — khi có, dùng `resume.id` thay vì random mới. Đây là **precondition bắt buộc** cho toàn bộ 30 solution (agent/backend/frontend) của CR-TRACE-001 → 019; không solution nào được bắt đầu implement cho đến khi task này DONE.

## File: `src/shared/trace/index.ts` [MODIFY]

**1. Mở rộng interface `Tracer`** (hiện tại chỉ có `start(fields?: TraceFields): TraceSpan`):

```typescript
export interface Tracer {
  start(fields?: TraceFields, resume?: { id: string }): TraceSpan
}
```

**2. Mở rộng implementation trong `createTracer()`** — thay đoạn `start()` hiện tại:

```typescript
// TRƯỚC (hiện tại):
export function createTracer(flow: string): Tracer {
  return {
    start(fields: TraceFields = {}): TraceSpan {
      const id = shortId()
      const startMs = Date.now()

      emit({ id, flow, level: 'start', fields, ts: startMs })

      return {
        id,
        step(label: string, stepFields: TraceFields = {}): void { /* ... giữ nguyên ... */ },
        ok(okFields: TraceFields = {}): void { /* ... giữ nguyên ... */ },
        fail(err: unknown, failFields: TraceFields = {}): void { /* ... giữ nguyên ... */ }
      }
    }
  }
}
```

```typescript
// SAU (task này):
export function createTracer(flow: string): Tracer {
  return {
    start(fields: TraceFields = {}, resume?: { id: string }): TraceSpan {
      // resume.id cho phép span "tiếp nối" id từ layer trước (Browser/Main/Relay/Agent)
      // thay vì tạo id ngẫu nhiên mới — xem CR-TRACE-000 §3.1/§3.2.
      const id = resume?.id ?? shortId()
      const startMs = Date.now()

      emit({ id, flow, level: 'start', fields, ts: startMs })

      return {
        id,

        step(label: string, stepFields: TraceFields = {}): void {
          emit({
            id, flow, level: 'step', label, fields: stepFields,
            ts: Date.now(), elapsedMs: Date.now() - startMs
          })
        },

        ok(okFields: TraceFields = {}): void {
          emit({
            id, flow, level: 'ok', fields: okFields,
            ts: Date.now(), elapsedMs: Date.now() - startMs
          })
        },

        fail(err: unknown, failFields: TraceFields = {}): void {
          const errMsg = formatError(err)
          emit({
            id, flow, level: 'fail',
            fields: { err: errMsg, ...failFields },
            ts: Date.now(), elapsedMs: Date.now() - startMs
          })
        }
      }
    }
  }
}
```

**Ràng buộc bắt buộc:**
- 100% backward compatible — mọi call site hiện có (`Tracers.browseDirFlow.start(fields)`, không truyền `resume`) phải tiếp tục hoạt động y hệt, không đổi behavior.
- `elapsedMs` của `step()`/`ok()`/`fail()` LUÔN tính từ `startMs` cục bộ của chính lần gọi `start()` này (`Date.now()` tại thời điểm `start()` được gọi ở layer hiện tại) — **không** kế thừa `startMs` từ layer đã tạo ra `resume.id`. Mỗi layer đo latency riêng của chính nó; tổng latency end-to-end được TracePanel/log aggregation tính sau, không phải trách nhiệm của `TraceSpan`.
- Không đổi `TraceEvent`, `TraceFields`, `TraceSpan`, sink registry, hay `isTraceEnabled()` — chỉ mở rộng chữ ký `start()`.

## File: `src/shared/trace/index.test.ts` [NEW]

Chưa có test file nào cho `src/shared/trace/` — tạo mới, theo convention test-sibling-file hiện có trong repo (vd. `src/shared/pairing.test.ts`), dùng Vitest.

```typescript
import { describe, expect, it, vi } from 'vitest'
import { createTracer, registerTraceSink, type TraceEvent } from './index'

describe('Tracer.start resume option', () => {
  it('generates a random id when resume is omitted', () => {
    const tracer = createTracer('test:omittedResume')
    const spanA = tracer.start({})
    const spanB = tracer.start({})

    expect(spanA.id).toBeTruthy()
    expect(spanB.id).toBeTruthy()
    expect(spanA.id).not.toBe(spanB.id)
  })

  it('uses resume.id exactly when provided', () => {
    const tracer = createTracer('test:withResume')
    const span = tracer.start({ foo: 'bar' }, { id: 'abc123' })

    expect(span.id).toBe('abc123')
  })

  it('computes elapsedMs from this layer\'s own startMs, not inherited from the resumed id', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))

    try {
      const upstreamTracer = createTracer('test:upstream')
      const upstreamSpan = upstreamTracer.start({})

      // Simulate time passing in the upstream layer before the id crosses the boundary.
      await new Promise((resolve) => setTimeout(resolve, 30))
      upstreamSpan.ok({})

      const downstreamTracer = createTracer('test:downstream')
      const downstreamSpan = downstreamTracer.start({}, { id: upstreamSpan.id })
      downstreamSpan.ok({})

      const downstreamOkEvent = events.find(
        (e) => e.flow === 'test:downstream' && e.level === 'ok'
      )

      expect(downstreamSpan.id).toBe(upstreamSpan.id)
      expect(downstreamOkEvent?.elapsedMs).toBeDefined()
      // Downstream span started fresh — its own elapsed time must be small,
      // not include the ~30ms already spent in the upstream layer.
      expect(downstreamOkEvent!.elapsedMs!).toBeLessThan(30)
    } finally {
      unregister()
    }
  })
})
```

## Verification

```bash
pnpm run typecheck:node
pnpm test --run src/shared/trace/index.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracer.start(fields?: TraceFields, resume?: { id: string }): TraceSpan` — chữ ký mới đúng như CR-TRACE-000 §3.1
- [ ] Bỏ qua `resume` (hoặc không truyền) → vẫn sinh `id` ngẫu nhiên như cũ, không lỗi TypeScript ở bất kỳ call site hiện có nào (`grep -rn "\.start(" src/shared/trace/tracers.ts` và mọi nơi dùng tracer)
- [ ] Truyền `resume: { id: 'abc123' }` → `span.id === 'abc123'` chính xác, không qua `shortId()`
- [ ] `elapsedMs` của `step()`/`ok()`/`fail()` luôn tính từ `startMs` cục bộ của layer hiện tại, kể cả khi `resume.id` được dùng
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
- [ ] `pnpm test --run src/shared/trace/index.test.ts` pass cả 3 test case
- [ ] Không sửa `TraceEvent`, `TraceFields`, `TraceSpan`, sink registry, hay `isTraceEnabled()`
