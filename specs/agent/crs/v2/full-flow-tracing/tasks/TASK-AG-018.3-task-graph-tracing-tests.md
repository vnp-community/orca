# TASK-AG-018.3: Add Task Graph tracing tests (agent-rpc-dispatch ai.complete/agent.exec buckets)

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-018](../solutions/SOL-AG-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Precondition:** Phase 0 + [TASK-AG-005.1](./TASK-AG-005.1-ai-complete-handler-tracer.md) + [TASK-AG-018.2](./TASK-AG-018.2-agent-exec-taskid-and-ai-complete-bucket.md)
**Estimated time:** 30 phút (giảm từ 1h — phần `ai-complete-handler.test.ts` đã bị xoá, xem ghi chú dưới)
**Status:** ✅ Done (2026-08-03) — Added the 3 test cases to `agent-rpc-dispatch.test.ts` only, did not touch `ai-complete-handler.test.ts` (confirmed already complete from TASK-AG-005.3, per this task's narrowed 2026-08-02 scope). The `ai.complete` test stubs `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GOOGLE_API_KEY` to empty so `handleAIComplete` fails fast without a real network call — irrelevant to the assertion anyway since the `agent:rpc` start event fires from `extractTraceFields()` before `route()` runs. `pnpm run typecheck:node` clean; `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` → 40/40 passed (includes all TASK-AG-015.2/017.2 cases, no regressions).

---

## ✅ Cập nhật 2026-08-02 — phạm vi task này đã thu hẹp

Trước đây task này định "chuẩn hoá lại" các assertion trong `ai-complete-handler.test.ts` theo field naming của `TASK-AG-018.1` (đã xoá). Vì `TASK-AG-005.1`/`TASK-AG-005.3` giờ đã tạo tracer VÀ test file đó với field naming chuẩn (`promptLength`/`contentLength`/`'provider-call'`) ngay từ đầu — không còn gì để "chuẩn hoá lại". `TASK-AG-005.3` đã bao phủ đủ 4 case tương đương (start/promptLength, fail/no-api-key, fail/empty-prompt, step/provider-call, ok/contentLength) — task này KHÔNG động vào `ai-complete-handler.test.ts` nữa, chỉ còn phần `agent-rpc-dispatch.test.ts` (thật sự mới, không trùng với task nào khác).

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "extractTraceFields"
```

Symbol MODIFY (đã tồn tại, vừa được TASK-AG-015.1/017.1/018.2 mở rộng cùng 1 bucket) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-rpc-dispatch.test.ts` [MODIFY]

```typescript
describe('ai.complete — extractTraceFields (CR-TRACE-018)', () => {
  it('surfaces model/taskId/promptLength on the agent:rpc dispatch span', async () => { /* dispatch ai.complete, assert agent:rpc start fields */ })
})

describe('agent.exec — taskId (CR-TRACE-018, forward-compat)', () => {
  it('surfaces taskId when present in params', async () => { /* params.taskId = 'task-99' */ })
  it('omits taskId cleanly when absent (current ProfileAwareAgentSpawner behavior)', async () => { /* matches today's real backend payload shape */ })
})
```

## Verification

```bash
pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent-rpc-dispatch.test.ts` có thêm 3 test case (`ai.complete` bucket + `agent.exec` taskId × 2)
- [ ] Test "surfaces model/taskId/promptLength" xác nhận bucket `ai.complete` không còn rơi vào `{}`
- [ ] KHÔNG động vào `ai-complete-handler.test.ts` — file đó đã hoàn chỉnh từ TASK-AG-005.3, không cần "chuẩn hoá lại"
- [ ] `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` pass toàn bộ (kể cả test case từ TASK-AG-015.2/017.2)
