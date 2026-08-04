# TASK-AG-018.2: Add ai.complete bucket + taskId field to agent-rpc-dispatch.ts extractTraceFields

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-018](../solutions/SOL-AG-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Precondition:** Phase 0 + [TASK-AG-015.1](./TASK-AG-015.1-agent-exec-trace-field-bucket.md) + [TASK-AG-017.1](./TASK-AG-017.1-agent-exec-stepid-parenttraceid-bucket.md) (extends the SAME `agent.exec` bucket — apply on top, do not duplicate) + [TASK-AG-005.1](./TASK-AG-005.1-ai-complete-handler-tracer.md) (creates `agent:aiComplete` on `ai-complete-handler.ts` — a different file; TASK-AG-018.1 was removed 2026-08-02 as fully redundant with TASK-AG-005.1 once both solutions' field naming was reconciled)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — Added a new `ai.complete` bucket (was falling through to `return {}`) and extended the same `agent.exec` object literal (from TASK-AG-015.1/017.1) with `taskId`, no second `if` block. Confirmed BL-TG-01/BL-TG-03 are backend-only (already stated in task doc, not re-verified by reading `TaskDAGValidator.ts`/`TaskGrantService.ts` again — out of this task's file scope). `pnpm run typecheck:node` clean; `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` → 37/37 passed (no regressions from 017.1/017.2 additions).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

`extractTraceFields()` trong `agent-rpc-dispatch.ts` là lần thứ 3 (sau TASK-AG-015.1, TASK-AG-017.1) mở rộng CÙNG bucket `method === 'agent.exec'`, cộng thêm bucket mới `method === 'ai.complete'` — target ĐÚNG symbol này (không phải cả file):

```bash
codegraph explore "extractTraceFields"
```

`extractTraceFields` là symbol MODIFY (đã tồn tại) — chạy thêm

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

Đọc `TaskDAGValidator.ts`/`TaskGrantService.ts` xác nhận: BL-TG-01 (CRUD + cycle detection) và BL-TG-03 (grant resolution) hoàn toàn in-process ở backend, KHÔNG có `relay.call()` nào — **không có và không cần agent-side counterpart**. Chỉ 2 sub-flow chạm agent: BL-TG-02 (AI decompose, qua `ai.complete` — tracer `agent:aiComplete` đã tạo ở [TASK-AG-005.1](./TASK-AG-005.1-ai-complete-handler-tracer.md), dùng chung giữa CR-TRACE-005 và CR-TRACE-018) và BL-TG-04 (agent execution, qua `agent.exec` — cùng path đã instrument bởi TASK-AG-015.1/017.1).

**Gap 1:** `extractTraceFields()` không có bucket nào cho `ai.complete` — method rơi vào `return {}` cuối hàm, span `agent:rpc` chỉ có `{method: 'ai.complete', id}`, không `model`, không `promptLength`.

**Gap 2 (forward-compat, không phải lỗi hôm nay):** `TaskAgentExecutor.executeTask()` → `ProfileAwareAgentSpawner.spawn()` chỉ gửi `{binary, args, cwd, env, timeoutMs}` tới `agent.exec` — KHÔNG gửi `taskId` như top-level param (`taskId` hiện chỉ nằm trong `env.ORCA_TASK_ID`). Đây là prerequisite backend chưa tồn tại — task này chuẩn bị sẵn phía agent để đọc `params.taskId` ngay khi backend gửi, không throw lỗi nếu chưa có.

## File: `src/relay/agent-rpc-dispatch.ts` [MODIFY]

```typescript
// src/relay/agent-rpc-dispatch.ts

function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
  // ...existing helpers unchanged...

  // ...existing fs./git./github.-gitlab./ai.provider. buckets unchanged...

  if (method === 'ai.complete') {
    // CR-TRACE-018 BL-TG-02: trước đây method này rơi vào `return {}` cuối hàm
    // — span agent:rpc ngoài cùng không có field nào. Bucket riêng ở đây là lớp
    // wrapper thô (id dispatch-level); breakdown chi tiết (provider-call step,
    // contentLength, fail reason) nằm ở tracer agent:aiComplete riêng
    // (ai-complete-handler.ts, TASK-AG-005.1) — hai tracer bổ sung cho nhau,
    // không trùng lặp: agent:rpc đo tổng thời gian dispatch (bao gồm cả import
    // động ./ai-complete-handler), agent:aiComplete đo riêng phần gọi provider.
    return {
      model:        str(p['model']),
      taskId:       str(p['taskId']),
      promptLength: typeof p['prompt'] === 'string' ? (p['prompt'] as string).length : undefined,
    }
  }

  if (method === 'agent.exec') {
    return {
      // (TASK-AG-015.1) base:
      binary:         str(p['binary']),
      argsCount:      Array.isArray(p['args']) ? (p['args'] as unknown[]).length : undefined,
      hasEnvOverride: p['env'] !== undefined && p['env'] !== null,
      timeoutMs:      num(p['timeoutMs']),
      // (TASK-AG-017.1):
      stepId:         str(p['stepId']),
      parentTraceId:  str(p['parentTraceId']),
      // CR-TRACE-018 BL-TG-04: chỉ có giá trị SAU KHI backend
      // (ProfileAwareAgentSpawner.spawn() / TaskAgentExecutor) được cập nhật để
      // gửi `taskId` như một top-level param thay vì chỉ nhét vào `env.ORCA_TASK_ID`
      // — cho tới lúc đó field này luôn undefined, không lỗi.
      taskId: str(p['taskId']),
    }
  }

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId']),
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}
```

**Điều phối:** đây là lần thứ 3 (sau TASK-AG-015.1, TASK-AG-017.1) mở rộng bucket `method === 'agent.exec'` — MỞ RỘNG object literal đã có (thêm `taskId` vào cuối), KHÔNG tạo `if (method === 'agent.exec')` riêng thứ 2.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-rpc-dispatch" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `extractTraceFields()` có bucket riêng cho `ai.complete` (`model`/`taskId`/`promptLength`) — trước đây rơi vào `return {}`
- [ ] Bucket `agent.exec` đọc `taskId` từ params, MỞ RỘNG object literal đã có từ TASK-AG-015.1/017.1 (không tạo block `if` thứ 2 cho cùng `method === 'agent.exec'`)
- [ ] Xác nhận trong code/comment: BL-TG-01 và BL-TG-03 không có và không cần agent-side counterpart
- [ ] Không lặp lại phần base instrumentation của `agent.exec` đã thuộc TASK-AG-015.1, không lặp lại `stepId`/`parentTraceId` đã thuộc TASK-AG-017.1
- [ ] `pnpm run typecheck:node` pass
