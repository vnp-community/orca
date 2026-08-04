# TASK-BE-018.2: Instrument `TaskAIPlanner.decompose()` — tách "AI call chậm" khỏi "parse JSON lỗi" (BL-TG-02)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-018](../solutions/SOL-BE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-018.1
**Status:** ✅ Done (2026-08-04) — Implemented per spec: `TaskAIPlanner.decompose()` wrapped with `Tracers.taskGraphAiPlanFlow`, `step('ai-call')` before the relay call, `parseAIResponseWithDiagnostics()` wrapper added (parseAIResponse() 152-170 untouched), `step('parse-plan', {subtaskCount, parseOk})`, `relay.call('ai.complete', {..., traceId: span.id})`, `AiDecomposeParam` got optional `traceId`. Kept the doc's single try/catch (TASK_NOT_FOUND path calls `span.fail()` then the outer catch calls `span.fail()` again on re-throw — matches the task doc's own sample verbatim, not separately fixed here). `pnpm tsc --noEmit` clean; `TaskAIPlanner.test.ts` 18/18 pass (14 original + 4 new tracing tests, see TASK-BE-018.6).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TaskAIPlanner.decompose"
```

Symbol đã tồn tại (MODIFY case). Chạy:

```
gitnexus_impact({ target: "TaskAIPlanner.decompose", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận `parseAIResponse()` gốc giữ nguyên không đổi, wrapper `parseAIResponseWithDiagnostics()` không đổi hành vi public của `decompose()` (vẫn trả `[]` khi parse lỗi). Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `TaskAIPlanner.decompose()` (tên hàm thật, không phải `decomposeTask()` như CR-TRACE-018 gốc) bằng span `taskGraph:aiPlan`. `parseAIResponse()` hiện có nuốt mọi lỗi parse JSON và luôn trả `[]` — không thể phân biệt "AI call chậm/lỗi network" với "AI trả về text không parse được". Task này thêm wrapper `parseAIResponseWithDiagnostics()` để expose thêm `parseOk`, KHÔNG đổi hành vi public của `decompose()` (vẫn trả `[]` khi parse lỗi), chỉ thêm observability nội bộ.

## File: `src/main/task/TaskAIPlanner.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

async decompose(taskId: string, projectId: string, userId: string): Promise<OrcaTask[]> {
  const span = Tracers.taskGraphAiPlanFlow.start({ taskId, projectId, userId })

  try {
    const task = await this.taskService.get(taskId)
    if (!task) { span.fail('TASK_NOT_FOUND', { taskId }); throw new Error(`TASK_NOT_FOUND: ${taskId}`) }

    const prompt = this.buildDecomposePrompt(task)

    const relay = await this.router.getRelayForProject(projectId, userId)
    span.step('ai-call', { method: 'ai.complete', promptLength: prompt.length })
    const response = (await relay.call('ai.complete', {
      prompt, format: 'json', taskId,
      traceId: span.id,   // [NEW] — forward vào relay envelope theo CR-TRACE-000 §3.3 hàng 2
    })) as { content?: string; text?: string } | string

    const { proposals, parseOk } = this.parseAIResponseWithDiagnostics(response)
    span.step('parse-plan', { subtaskCount: proposals.length, parseOk })

    const result = proposals.map((p) => ({
      id: `proposed:${Date.now()}:${Math.random().toString(36).slice(2)}`,
      parentId: taskId, projectId: task.projectId, title: p.title, description: p.description,
      type: p.type ?? 'subtask', status: 'backlog' as const, priority: task.priority, labels: [],
      visibility: task.visibility, progressPercent: 0, estimatedHours: p.estimatedHours,
      createdAt: new Date(), updatedAt: new Date(),
    }))

    span.ok({ subtaskCount: result.length, parseOk })
    return result
  } catch (err) {
    span.fail(err, { taskId })
    throw err
  }
}

// [NEW] — wrapper quanh parseAIResponse() hiện có, KHÔNG đổi hành vi public (vẫn trả [] khi lỗi),
// chỉ expose thêm parseOk để decompose() có thể trace chính xác "AI call chậm" (ai-call step latency)
// tách biệt khỏi "parse JSON lỗi" (parseOk: false) — trước đây 2 nguyên nhân này không phân biệt được
// từ bên ngoài vì parseAIResponse() nuốt mọi lỗi thành mảng rỗng.
private parseAIResponseWithDiagnostics(response: unknown): { proposals: SubtaskProposal[]; parseOk: boolean } {
  try {
    let text = ''
    if (typeof response === 'string') {
      text = response
    } else if (response && typeof response === 'object') {
      const r = response as Record<string, unknown>
      text = (r['content'] ?? r['text'] ?? '') as string
    }
    const match = text.match(/\[[\s\S]*\]/)
    if (!match) return { proposals: [], parseOk: false }
    return { proposals: JSON.parse(match[0]) as SubtaskProposal[], parseOk: true }
  } catch {
    console.warn('[TaskAIPlanner] Failed to parse AI response')
    return { proposals: [], parseOk: false }
  }
}

// parseAIResponse() (152-170) giữ nguyên KHÔNG đổi — chỉ dùng nội bộ bởi các call site khác nếu có.
```

## File: `src/main/task/task-rpc-handler.ts` [MODIFY — chỉ phần schema]

```typescript
const AiDecomposeParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  traceId: z.string().optional(),   // [NEW]
})
```

> `aiPlanner.decompose()` hiện chưa nhận `traceId` param ở entry point (chỉ tự `start()` span mới bên trong) — cùng caveat như TASK-BE-018.1, không chặn phần lõi.

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

- [ ] `Tracers.taskGraphAiPlanFlow` tách rõ 2 nguyên nhân: `step('ai-call')` đo latency relay call riêng biệt với `step('parse-plan', { parseOk })` — 2 field độc lập, không gộp
- [ ] `parseAIResponseWithDiagnostics()` KHÔNG đổi hành vi public của `decompose()` (vẫn trả `[]` khi parse lỗi, không throw)
- [ ] `parseAIResponse()` gốc (dòng 152-170) giữ nguyên, không bị sửa
- [ ] `relay.call('ai.complete', ...)` forward `traceId: span.id`
- [ ] `AiDecomposeParam` (`task-rpc-handler.ts`) có field `traceId?: string` optional
- [ ] `decompose()` với `relay.call` throw lỗi network → `span.fail(err)`, KHÔNG có `step('parse-plan')` (phân biệt rõ với lỗi parse)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
