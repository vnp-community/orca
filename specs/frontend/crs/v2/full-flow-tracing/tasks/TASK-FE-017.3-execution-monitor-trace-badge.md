# TASK-FE-017.3: Hiển thị `rootTraceId` copyable trên `ExecutionMonitor.tsx`

**Phase:** 3
**SOL Ref:** [SOL-FE-TRACE-017 §3.3](../solutions/SOL-FE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-017.1 (field `rootTraceId` trên `WorkflowExecution`)
**Status:** ✅ Done (2026-08-04) — Implemented as specced, no drift. Added 2 new test cases to existing `ExecutionMonitor.test.tsx` (badge shown+copy, badge hidden when `rootTraceId` undefined) — 7/7 passing. Needed `Object.defineProperty(navigator, 'clipboard', ...)` instead of `Object.assign` in test since happy-dom's `navigator.clipboard` is getter-only. `pnpm tsc --noEmit` clean.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "ExecutionMonitor"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "ExecutionMonitor", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục — task này thuần UI (hiển thị field đã có), nên kỳ vọng risk thấp, nhưng vẫn phải chạy để xác nhận.

## Mô tả

Task thuần UI, không tạo span mới — chỉ **hiển thị** `execution.rootTraceId` (đã lưu ở TASK-FE-017.1) cho user copy vào ô filter TracePanel. `TracePanel.tsx:236-251` có ô filter free-text tìm theo flow/id/field — nếu user dán đúng `rootTraceId`, TracePanel hiển thị MỌI span (browser + backend qua SSE) cùng id này, kể cả các `workflow:stepExecute` con mang field `parentTraceId` trùng giá trị (`TraceEventRow` tìm trong `Object.values(e.fields)` — `filter.ts:158-163`).

## File: `src/renderer/src/components/workflow/ExecutionMonitor.tsx` [MODIFY, additive]

```typescript
// Thêm vào phần header, cạnh StepStatusBadge — hiển thị rootTraceId dạng monospace,
// click để copy.
{execution.rootTraceId && (
  <button
    className="text-[10px] font-mono text-muted-foreground hover:text-foreground"
    title="Copy trace ID — paste into TracePanel filter (Ctrl+Shift+T) to see all steps"
    onClick={() => navigator.clipboard.writeText(execution.rootTraceId!)}
    data-testid="root-trace-id-badge"
  >
    trace:{execution.rootTraceId}
  </button>
)}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/workflow/__tests__/ExecutionMonitor.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `ExecutionMonitor` hiển thị `rootTraceId` dưới dạng có thể copy (`data-testid="root-trace-id-badge"`), để user dán vào ô filter TracePanel và thấy toàn bộ span (browser + backend) của 1 execution
- [ ] Không hiển thị badge khi `execution.rootTraceId` là `undefined` (execution cũ, trước khi tracing được bật)
- [ ] Click badge gọi `navigator.clipboard.writeText(execution.rootTraceId)`
- [ ] Task này KHÔNG tạo span/tracer mới — thuần đọc field đã lưu từ TASK-FE-017.1
- [ ] Test suite đạt ≥ 2 test case mới: hiển thị badge khi `rootTraceId` có giá trị; không hiển thị khi `undefined`
