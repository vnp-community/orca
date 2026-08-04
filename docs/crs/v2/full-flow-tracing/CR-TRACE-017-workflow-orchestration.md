# CR-TRACE-017 — Workflow Orchestration Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-017 |
| **Tên** | Workflow Orchestration — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P3 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/workflow-orchestration.md`, `src/main/workflow/WorkflowOrchestrator.ts`, `src/main/workflow/DAGBuilder.ts`, `src/main/workflow/StepExecutors.ts`, `src/main/workflow/TemplateResolver.ts`, `src/main/workflow/workflow-rpc-handler.ts`, `src/main/project/ProjectServerRouter.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

`WorkflowOrchestrator` (`WorkflowOrchestrator.ts:74`) chạy execution qua nhiều **wave** (`runExecution()`, private, dòng 228) — mỗi wave dispatch song song N step qua `Promise.allSettled` (comment dòng 6-11 trong file), mỗi step gọi `StepExecutors.execute()` (`StepExecutors.ts:33`) rồi `relay.call('agent.exec' | 'shell.exec' | 'notification.send', ...)` tới Dev Server tương ứng (có thể là nhiều Dev Server khác nhau trong cùng 1 workflow — xem sơ đồ tổng quan trong flow doc). Hiện tại **không có tracer nào** ở toàn bộ 3 sub-flow. Hệ quả cụ thể:

- Khi 1 workflow "kẹt" giữa chừng, không biết đang ở wave nào, step nào trong wave đó đang chạy, hay đã hoàn tất nhưng `updateCurrentWave()` (dòng 421) chưa persist.
- `StepExecutors.execute()` có `Promise.race` với timeout riêng từng step (`STEP_TIMEOUT`, mặc định 30 phút) — khi step timeout, không biết relay đã gửi request chưa hay đang chờ ở đâu (network, agent bận, hay Dev Server không phản hồi).
- Vì 1 workflow execution gồm nhiều step trải trên nhiều Dev Server, log hiện tại (nếu có) không thể nhóm theo execution — mỗi step trông như một request độc lập.

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row |
|---|---|---|---|---|
| Browser (Workflow builder/monitor) | — | UI | WebSocket RPC (`workflow.execute`, `workflow.template.*`) | Row 1 |
| TemplateRegistry (CRUD template) | `TemplateResolver.ts` — có method `create()` (dòng 120), `list()` (146), `resolve()` (72). Không có class riêng tên `TemplateRegistry`; toàn bộ CRUD nằm trong `TemplateResolver` | Business Logic | in-process, gọi từ `workflow-rpc-handler.ts:149-186` | n/a |
| TemplateResolver (inheritance merge) | `TemplateResolver.resolve()` (dòng 72) | Business Logic | in-process | n/a |
| WorkflowOrchestrator | `WorkflowOrchestrator.ts:74` — `execute()` (93), `runExecution()` private (228), `executeStep()` private (307) | Business Logic | orchestrator nội bộ, dispatch ra ngoài qua `StepExecutors` | n/a (điểm build DAG + wave, không tự nó là network hop) |
| StepExecutors | `StepExecutors.ts:22` — `execute()` (33), `executeAgent()`/`executeShell()`/`executeNotification()` gọi `relay.call()` | Business Logic | `relay.call()` (Orca Server ↔ Dev Server, qua `ProjectServerRouter`) | Row 2 |
| WorkflowServerResolver (routing "project:"/"server:"/"fleet:tag:") | **chưa xác định file cụ thể — cần điều tra thêm khi triển khai.** Grep không tìm thấy class hay hàm nào implement cascade `project:`/`server:`/`fleet:tag:` này; `StepExecutors` chỉ có `getRelay(step)` dùng `ProjectServerRouter.getRelayForProject()` (`ProjectServerRouter.ts:29`), tức routing hiện tại chỉ theo `projectId`, chưa thấy cú pháp `fleet:tag:` trong code đã grep | Business Logic | `relay.call()` | Row 2 |
| Server Database | `orca_workflow_templates`/`_executions`/`_step_executions` qua `this.pool.withConnection()` trong `WorkflowOrchestrator.ts`/`TemplateResolver.ts` | Persistence | in-process | n/a |
| Dev Server (relay) | ngoài repo | Remote | nhận `agent.exec`/`shell.exec`/`notification.send` qua relay | Row 2 |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  workflowTemplateCreateFlow: createTracer('workflow:templateCreate'), // BL-WF-01
  workflowExecuteFlow:        createTracer('workflow:execute'),        // BL-WF-02 (span cha — 1 per execution)
  workflowStepFlow:           createTracer('workflow:stepExecute'),    // BL-WF-02 (span con — 1 per step)
  workflowShareFlow:          createTracer('workflow:share'),          // BL-WF-03
}
```

**1 tracer = 1 sub-flow** theo quy ước CR-TRACE-000 §4, nhưng BL-WF-02 cần **2 tracer** (`workflowExecuteFlow` cho span cha của cả execution, `workflowStepFlow` cho span con của từng step) vì đây là flow DAG/multi-step — xem §4 bên dưới về mô hình parent-correlation.

## 4. Instrumentation theo từng sub-flow

### BL-WF-01 — Workflow Template Management (Create/Inherit/Share)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `workflow.template.create` | `start` | `name`, `scope`, `hasParent: !!parentTemplateId` | `workflow-rpc-handler.ts:149-165` |
| IF có parentTemplateId: resolve + merge | `step('resolve-parent')` | `parentTemplateId` | `TemplateResolver.ts:72` (`resolve()`), gọi từ `create()` khi có kế thừa |
| INSERT template | `ok({ templateId })` / `fail(err)` | `templateId?` | `TemplateResolver.ts:120` (`create()`) |

```typescript
// workflow-rpc-handler.ts — trong handler của 'workflow.template.create'
handler: async (params, ctx) => {
  const userId = ctx.userId ?? 'system'
  const span = Tracers.workflowTemplateCreateFlow.start({
    name: params.name, scope: params.scope, hasParent: !!params.parentTemplateId
  })
  try {
    const id = await templateResolver.create({
      name: params.name, definition: params.definition,
      ownerId: userId, scope: params.scope, parentTemplateId: params.parentTemplateId
    })
    span.ok({ templateId: id })
    return { templateId: id }
  } catch (err) {
    span.fail(err)
    throw err
  }
}
```

### BL-WF-02 — Multi-Server Workflow Execution (DAG / Wave)

Đây là sub-flow phức tạp nhất trong 19 flow: **1 workflow execution → N wave → M step mỗi wave, chạy song song, mỗi step có thể nhắm tới Dev Server khác nhau.**

**Mô hình parent-correlation (khuyến nghị dùng cho DAG/wave flow):**
- `workflowExecuteFlow.start()` tạo span cha khi `WorkflowOrchestrator.execute()` được gọi — span id này = `executionId`-scoped trace, gọi là `rootTraceId`.
- Mỗi step, khi `executeStep()` (`WorkflowOrchestrator.ts:307`) gọi `StepExecutors.execute()`, tạo span con riêng bằng `workflowStepFlow.start(fields)` (per CR-TRACE-000's per-boundary-hop model — mỗi step vẫn có `id` riêng vì mỗi step là 1 network hop độc lập, có thể fail/timeout độc lập).
- Field bổ sung `parentTraceId: rootTraceId` được set trong `fields` của MỌI span con (`workflowStepFlow`) để TracePanel group toàn bộ step theo 1 execution, dù mỗi step có `id` riêng. Đây KHÔNG phải cơ chế `resume` của CR-TRACE-000 §3.1 (resume dùng để 1 span tiếp nối id qua nhiều layer của CÙNG một hop) — `parentTraceId` là field nghiệp vụ thuần túy để nhóm nhiều span độc lập lại, tương tự cách `devServerId` hay `accountId` hiện được set trong `fields`.
- Nếu FE muốn theo dõi tiến độ execution real-time, nó cũng dùng `rootTraceId` (= `workflowExecuteFlow.start().id`, trả về cùng `executionId` trong response `workflow.execute`) để filter TracePanel.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu execution (span cha) | `start` | `executionId`, `templateId`, `projectId` | `WorkflowOrchestrator.ts:93` (`execute()`) |
| Build DAG + wave | `step('build-waves')` | `waveCount`, `stepCount` | `DAGBuilder.ts:32` (`buildWaves()`), gọi từ `runExecution()` dòng 235 |
| Mỗi wave bắt đầu | `step('wave-start')` | `waveIndex`, `stepsInWave` | `WorkflowOrchestrator.ts:237-243` |
| Mỗi step (span con riêng, KHÔNG phải step() của span cha) | `start` (tracer `workflowStepFlow`, field `parentTraceId`) | `parentTraceId`, `executionId`, `stepId`, `stepType`, `devServerId` | `WorkflowOrchestrator.ts:307` (`executeStep()`) → `StepExecutors.execute()` |
| Step gọi relay | `step('agent-call')` | `method: 'agent.exec' \| 'shell.exec' \| 'notification.send'` | `StepExecutors.ts:88,108,149` |
| Step kết thúc | `ok({ exitCode })` / `fail(err)` | `exitCode?` | `StepExecutors.ts` (return `StepOutput`) |
| Execution kết thúc (span cha) | `ok({ waveCount, status: 'completed' })` / `fail(err)` | | `WorkflowOrchestrator.ts:402` (`markExecutionCompleted`) / `:411` (`markExecutionFailed`) |

```typescript
// WorkflowOrchestrator.ts — execute() + runExecution()
async execute(definition, inputs, triggeredBy, projectId): Promise<WorkflowExecution> {
  const id = randomUUID()
  const span = Tracers.workflowExecuteFlow.start({ executionId: id, projectId })
  // ...persistExecution...
  void this.runExecution({ id, definition, /* ... */ }, 0, span.id) // truyền rootTraceId xuống
  return execution
}

private async runExecution(execution: WorkflowExecution, startWave = 0, rootTraceId?: string): Promise<void> {
  const waves = this.dagBuilder.buildWaves(execution.definition.steps)
  // span.step('build-waves', ...) — gọi trên span cha đã start ở execute(); nếu resumeRunningExecutions()
  // gọi lại runExecution() sau restart, rootTraceId có thể null → tạo span cha mới ở đây bằng resume nếu có id cũ lưu DB
  for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
    const wave = waves[waveIndex]
    await Promise.allSettled(
      wave.map(async (step) => {
        const stepSpan = Tracers.workflowStepFlow.start({
          parentTraceId: rootTraceId, executionId: execution.id,
          stepId: step.id, stepType: step.config.type
        })
        try {
          const output = await this.executeStep(step, execution, /* signal */ undefined as never)
          stepSpan.ok({ exitCode: output.exitCode })
        } catch (err) {
          stepSpan.fail(err, { stepId: step.id })
        }
      })
    )
  }
}
```

**Lưu ý resume sau restart:** `resumeRunningExecutions()` (`WorkflowOrchestrator.ts:218`) gọi lại `runExecution()` cho execution còn `status='running'` sau khi Orca Server restart. Để giữ `rootTraceId` xuyên suốt kể cả sau restart, `rootTraceId` (span cha ban đầu) nên được persist vào `orca_workflow_executions` (cột mới, ví dụ `root_trace_id`) khi `persistExecution()` chạy, rồi đọc lại và truyền vào `resume: { id: rootTraceId }` khi tạo lại span cha lúc resume — nếu không, span cha sẽ có id mới sau mỗi lần restart và làm gãy correlation.

### BL-WF-03 — Workflow Sharing & Library Discovery

> **Ghi chú quan trọng:** grep code hiện tại (`TemplateResolver.ts`, `workflow-rpc-handler.ts`) **không tìm thấy** implementation cho `updateVisibility()`, share-link token, hay `/api/workflows/shared/:token` — các API mô tả trong `workflow-orchestration.md` mục BL-WF-03 dường như **chưa được triển khai** (chỉ có `create`/`list`/`resolve` theo `scope` tĩnh). CR này vẫn định nghĩa tracer theo naming convention để dùng ngay khi tính năng sharing được code, nhưng KHÔNG có file:function thật để trích dẫn — đây là **placeholder instrumentation spec**, không phải patch áp dụng ngay.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Đổi visibility | `start` → `ok`/`fail` | `templateId`, `visibility` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai |
| Tạo share link (nếu public) | `step('generate-share-link')` | `templateId` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai |
| Import từ share link | `start` → `ok`/`fail` | `token` (KHÔNG log toàn bộ token nếu token đóng vai trò bí mật — chỉ log 6 ký tự đầu) | chưa xác định file cụ thể — cần điều tra thêm khi triển khai |

## 5. Lan truyền traceId qua transport của flow này

- **Browser → Orca Server (`workflow.execute` WS RPC)**: theo CR-TRACE-000 §3.3 hàng 1 — nếu FE gửi kèm `traceId`, `workflow-rpc-handler.ts` truyền `resume: { id: params.traceId }` vào `Tracers.workflowExecuteFlow.start()`; response trả `{ executionId, status }` — nên bổ sung trả thêm `traceId: span.id` để FE map execution với trace stream.
- **Orca Server (WorkflowOrchestrator) → Dev Server (StepExecutors → relay.call)**: theo CR-TRACE-000 §3.3 hàng 2 — mỗi lời gọi `relay.call('agent.exec' | 'shell.exec' | 'notification.send', { ..., traceId: stepSpan.id })` trong `StepExecutors.ts` (dòng 88, 108, 149) đính `traceId` vào params envelope bằng `id` của **span con của step đó** (`workflowStepFlow`), KHÔNG phải `rootTraceId` của execution — vì đây là hop riêng của step, đúng mô hình "mỗi layer đo latency riêng" (CR-TRACE-000 §3.1).
- **Nhóm theo execution ở tầng hiển thị**: TracePanel/log aggregation dùng field nghiệp vụ `parentTraceId` (không phải `id`) để filter tất cả `workflow:stepExecute` span thuộc cùng 1 `workflow:execute` span cha — cơ chế này bổ sung, không thay thế, cơ chế `resume`/`traceId` chuẩn của CR-TRACE-000.

## Acceptance Criteria

- [ ] `Tracers.workflowExecuteFlow` bao phủ trọn vòng đời execution: start → build-waves → wave-start × N → ok/fail
- [ ] `Tracers.workflowStepFlow` tạo 1 span độc lập cho mỗi step, mang field `parentTraceId` trỏ về span cha
- [ ] Field `parentTraceId` xuất hiện nhất quán trên MỌI step span trong cùng 1 execution, kể cả sau khi `resumeRunningExecutions()` chạy lại sau restart
- [ ] `traceId` forward đúng vào `relay.call()` params envelope ở cả 3 loại step (agent/shell/notification)
- [ ] TracePanel có thể group toàn bộ span của 1 execution bằng `parentTraceId` mà không cần join thủ công qua `executionId` trong DB
- [ ] `Tracers.workflowTemplateCreateFlow` phân biệt được template tạo mới vs kế thừa qua field `hasParent`
- [ ] BL-WF-03 tracer (`workflowShareFlow`) được document sẵn nhưng gắn cờ rõ ràng "chưa triển khai" cho tới khi code sharing thực sự tồn tại
- [ ] Không thêm `span.step()` cho các UPDATE đơn dòng (`updateCurrentWave`, `persistStepStart`, `persistStepComplete`) theo nguyên tắc §5 CR-TRACE-000 — các UPDATE này gộp vào fields của `ok()`/`fail()` step tương ứng
