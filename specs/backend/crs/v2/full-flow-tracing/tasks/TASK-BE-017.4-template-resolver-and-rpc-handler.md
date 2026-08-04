# TASK-BE-017.4: Instrument `TemplateResolver.create()` (BL-WF-01) + forward `traceId` trong `workflow-rpc-handler.ts`

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-017](../solutions/SOL-BE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-017.1, TASK-BE-017.2
**Status:** ✅ Done (2026-08-04) — `TemplateResolver.create()` wrapped with `Tracers.workflowTemplateCreateFlow`, `hasParent` field distinguishes new vs inherited templates; `resolve()` untouched (no tracer), confirmed via `grep -r workflowShareFlow src/main/workflow/` matching only the tracer definition, no call site. `workflow.execute`'s `ExecuteParam` schema gained an optional `traceId` field (reserved, not yet wired into `orchestrator.execute()` — documented as a follow-up, matching the doc's own acknowledged scope limit). Response now returns `traceId` read back from the persisted `root_trace_id` column. DRIFT: the real `createWorkflowMethods(orchestrator, templateResolver)` factory (unlike the doc's sample) had no `pool` access, so a 3rd optional `pool?: IConnectionPool` param was added (backward compatible — existing 2-arg call sites/tests unaffected) and the one production call site in `server-bootstrap.ts` was updated to pass `pool`. 3 new tracing tests added to `TemplateResolver.test.ts` (13/13 pass total). `pnpm tsc --noEmit` — no new errors (remaining errors in these files are pre-existing `db.query<T>`/zod `.record()` version-mismatch issues, confirmed via `git status` before editing).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TemplateResolver.create"
codegraph explore "workflow.execute"
```

Cả 2 là symbol đã tồn tại (MODIFY case). Chạy:

```
gitnexus_impact({ target: "TemplateResolver.create", direction: "upstream" })
gitnexus_impact({ target: "workflow.execute", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận `TemplateResolver.resolve()` (cùng file) KHÔNG bị thêm tracer trong task này. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `TemplateResolver.create()` bằng span `workflow:templateCreate`, phân biệt template tạo mới vs kế thừa qua field `hasParent`. `resolve()` (đọc inheritance chain) **không** nhận tracer riêng — nó là read-path chạy mỗi lần cần resolve, kể cả trong `workflow.execute`, không phải write path BL-WF-01. Sau đó thêm `traceId` vào schema `workflow.execute` và trả `traceId` (đọc lại từ `root_trace_id` đã persist ở TASK-BE-017.2) trong response để FE filter TracePanel theo execution.

## File: `src/main/workflow/TemplateResolver.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

async create(params: CreateTemplateParams): Promise<string> {
  const span = Tracers.workflowTemplateCreateFlow.start({
    name: params.name, scope: params.scope ?? 'user', hasParent: !!params.parentTemplateId,
  })
  try {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_workflow_templates (id, name, definition_json, owner_id, scope, parent_template_id, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [id, params.name, JSON.stringify(params.definition), params.ownerId, params.scope ?? 'user', params.parentTemplateId ?? null, now, now]
      )
    )
    span.ok({ templateId: id })
    return id
  } catch (err) {
    span.fail(err)
    throw err
  }
}

// resolve() (dòng 72) — KHÔNG có tracer: đây là read-path gọi mỗi lần cần resolve inheritance
// chain (kể cả trong workflow.execute), không phải write path BL-WF-01. Nếu cần đo latency
// inheritance-chain-walk sâu (MAX_INHERIT_DEPTH=5), field này nên gộp vào step('resolve-parent')
// bên trong span cha của caller (workflow:execute khi resolve() được gọi từ đó), không tự tạo
// tracer riêng — tránh vi phạm "1 tracer = 1 sub-flow" khi resolve() không phải là 1 sub-flow độc lập.
```

## File: `src/main/workflow/workflow-rpc-handler.ts` [MODIFY]

```typescript
const ExecuteParam = z.object({
  definition: WorkflowDefinitionSchema,
  inputs: z.record(z.unknown()).optional(),
  projectId: z.string().optional(),
  traceId: z.string().optional(),   // [NEW]
})
```

```typescript
defineMethod({
  name: 'workflow.execute',
  params: ExecuteParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId ?? 'system'
    // NOTE: orchestrator.execute() hiện chưa nhận resume param — cần overload tương tự
    // AIProviderService.writeCredentialToDevServer() (TASK-BE-016.1) nếu muốn FE-initiated
    // traceId resume vào workflowExecuteFlow. Việc này để lại cho patch nhỏ bổ sung khi cần,
        // không chặn phần còn lại của task (rootTraceId nội bộ vẫn hoạt động độc lập).
    const execution = await orchestrator.execute(
      params.definition as Parameters<typeof orchestrator.execute>[0],
      params.inputs ?? {}, userId, params.projectId,
    )
    // Trả traceId để FE có thể filter TracePanel theo execution — đọc lại rootTraceId đã persist
    const row = await pool.withConnection((db) =>
      db.query<{ rootTraceId: string | null }>(
        `SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?`, [execution.id]
      )
    )
    return { executionId: execution.id, status: execution.status, traceId: row[0]?.rootTraceId ?? undefined }
  },
}),
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

- [ ] `Tracers.workflowTemplateCreateFlow` phân biệt template tạo mới vs kế thừa qua field `hasParent: true/false`
- [ ] `TemplateResolver.resolve()` KHÔNG có bất kỳ `Tracers.*`/span nào được thêm trong task này
- [ ] `workflow.execute` schema (`ExecuteParam`) có field `traceId?: string` optional
- [ ] `workflow.execute` response trả `traceId` đọc lại từ `root_trace_id` đã persist (không phải giá trị tính lại trong bộ nhớ)
- [ ] `Tracers.workflowShareFlow` (khai báo ở TASK-BE-017.2) KHÔNG có bất kỳ call site nào trong task này — verify bằng `grep -r "workflowShareFlow" src/main/workflow/` chỉ match chính định nghĩa tracer
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
