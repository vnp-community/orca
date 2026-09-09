# Multi-Server Workflow Orchestration (F36) — Change Requests (v4)

> **Bối cảnh:** [`docs/roadmap/feature-completion-matrix.md`](../../../roadmap/feature-completion-matrix.md)
> xếp F36 là Backend-go ✅ / Agent ✅ / Frontend 🟡 ("thiếu Template Library/
> Sharing UI"). Khảo sát sâu hơn (đối chiếu code thật với
> `specs/backend-go/bugs/logic-v1/BUG-WF-01..03`,
> `specs/backend-go/bugs/task-v1/BUG-TASKV1-006/007`,
> `specs/backend-go/bugs/missing-v1/BUG-030`,
> `specs/frontend/bugs/workflow-orchestration/BUG-FE-WF-001`,
> `specs/frontend/bugs/task-v1/BUG-FE-TASKV1-007`,
> `specs/agent/bugs/task-v1/BUG-AGENT-TASKV1-004`) cho thấy "✅" ở Backend-go
> trong ma trận đúng ở tầng **DAG/wave engine + pause/resume** (thật, có test)
> nhưng còn nhiều mảnh spec **chưa từng được build**: không có server/provider
> target resolution, không có variable interpolation, thiếu 2/6 step type
> (`action`/`parallel`), sharing/library hoàn toàn chưa có schema, và — nghiêm
> trọng nhất — **agent step executor gọi sai RPC (`agent.exec` thay vì
> `agent.execPrompt`), khiến MỌI bước loại `agent` trong workflow thất bại
> ngay hôm nay**.
>
> Không lặp lại nội dung đã có ở [`docs/crs/v3/flow-task/`](../../v3/flow-task/README.md)
> (kiến trúc liên kết Task↔Workflow làm Engine 3, event catalog hợp nhất,
> cutover Node→backend-go) hay [`docs/crs/v2/full-flow-tracing/CR-TRACE-017`](../../v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
> (tracer cho kiến trúc `backend/` legacy, P3/observability-only). Series này
> xử lý **bản thân F36 hoàn thiện đến đâu** trong `workflow-service` +
> frontend workflow components + agent step executor.

## Bảng CR

| CR | Vấn đề | Giải pháp | Layer | Priority | Status |
|----|--------|-----------|-------|----------|--------|
| [CR-WF-001](./CR-WF-001-fix-agent-step-executor-relay-method.md) | `agent_step_executor.go` gọi `agent.exec` (generic exec) thay vì `agent.execPrompt` — mọi step `agent` fail hôm nay | Sửa method + param shape (theo SOL-PRF-04) | Backend-go | P0 (hotfix) | 🔵 Proposed |
| [CR-WF-002](./CR-WF-002-server-and-provider-resolution.md) | Step chỉ có `connectionId` trần — không resolver nào cho `project:`/`server:`/`fleet:tag:`; `agent` step không resolve AI provider | Resolver layer mới cho server target + AI provider (theo priority chain TDD đã sketch) | Backend-go | P0 | 🔵 Proposed |
| [CR-WF-003](./CR-WF-003-variable-interpolation-and-step-types.md) | `ExecuteRequest` không có `inputs`; không interpolation `{{outputs.*}}`; thiếu step type `action`/`parallel` | Thêm `inputs`, interpolation pass, 2 step type mới | Backend-go | P1 | 🔵 Proposed |
| [CR-WF-004](./CR-WF-004-template-inheritance-merge-and-clone.md) | Inheritance là "nearest-ancestor-wins" toàn bộ DAG, không phải field-level merge; không có Clone; version bump vô điều kiện | Deep-merge `overrides`/`inject_steps`/`remove_steps`, Clone mode, version bump có điều kiện | Backend-go | P1 | 🔵 Proposed |
| [CR-WF-005](./CR-WF-005-template-sharing-library-and-list-executions.md) | Không schema/RPC nào cho sharing/library; `WorkflowMonitor.tsx` gọi `workflow.listExecutions` — **RPC không tồn tại** | Schema + RPC sharing/library mới, thêm `ListExecutions` | Backend-go | P1 | 🔵 Proposed |
| [CR-WF-006](./CR-WF-006-frontend-builder-library-pause-resume.md) | `WorkflowBuilder.tsx` không mount ở đâu; không Library/Sharing UI; `ExecutionMonitor` không có nút Pause/Resume dù backend đã hỗ trợ; step-type vocabulary lệch (`notify`/`approval` vs `notification`) | Mount Builder, xây Library/Sharing UI, thêm Pause/Resume, sửa vocabulary | Frontend | P0/P1 | 🔵 Proposed |
| [CR-WF-007](./CR-WF-007-execution-live-streaming.md) | Không WS push nào cho step/execution event — UI chỉ poll 4s | Emit event tại đúng điểm trong `wave_dispatcher.go`, tái dùng contract của CR-FLOW-TASK-003 | Backend-go + Frontend | P2 | 🔵 Proposed |

## Thứ tự thực thi

```
CR-WF-001 (hotfix — làm ngay, có thiết kế sẵn từ SOL-PRF-04)
  ├── CR-WF-002 (resolver server/provider — độc lập kỹ thuật)
  └── CR-WF-004 (inheritance merge + Clone — độc lập kỹ thuật)
        └── CR-WF-003 (inputs/interpolation — phối hợp thời điểm đổi proto
              ExecuteRequest với CR-FLOW-TASK-002's origin_task_id, tránh 2 lần
              migration proto riêng lẻ)
              └── CR-WF-005 (sharing/library — cần Clone mode từ CR-WF-004
                    làm nền cho "Import to My Workflows"; có thể tách nhanh
                    phần ListExecutions ra làm fix riêng nếu WorkflowMonitor
                    cần chạy được gấp)
                    └── CR-WF-006 (frontend — Library UI phụ thuộc CR-WF-005；
                          Pause/Resume UI + step-type fix không phụ thuộc gì,
                          có thể ship sớm hơn độc lập)
CR-WF-007 — phụ thuộc cứng CR-FLOW-TASK-003 (kênh event) đã tồn tại, không
  build trước để tránh 2 event schema không tương thích.
```

## Không thuộc phạm vi series này (đã có CR/thiết kế riêng — không lặp lại)

- **`docs/crs/v3/flow-task/CR-FLOW-TASK-002`**: wiring Task→Workflow (Engine 3)
  — `task.workflow_template_id`, `workflow.executions.origin_task_id`,
  `task-service.WorkflowExecutor`, callback `ReportTaskExecutionResult`. CR-WF-003's
  thêm field `inputs` vào `ExecuteRequest` PHẢI phối hợp thời điểm với CR-002's
  `origin_task_id` — cùng 1 migration proto nếu 2 CR chạy gần nhau.
- **`docs/crs/v3/flow-task/CR-FLOW-TASK-003`**: event catalog/outbox hợp nhất
  3-engine — CR-WF-007 chỉ "cắm" vào catalog này ở đúng điểm trong
  `wave_dispatcher.go`, không tự thiết kế event schema riêng.
- **`docs/crs/v2/full-flow-tracing/CR-TRACE-017`**: tracer cho kiến trúc
  `backend/` legacy — mục tiêu file lỗi thời, P3, observability-only. Nếu
  sharing (CR-WF-005) build xong, cần 1 follow-up nhỏ re-map tracer
  `workflowShareFlow` sang file backend-go thật — không phải phạm vi CR-WF-005.
- **`docs/crs/v2/dev-server/CR-DS-001..004`**: định nghĩa protocol `step.execute`
  đề xuất — vẫn ở trạng thái "Proposed", **chưa implement**; implementation
  thật dùng RPC riêng theo từng step-type (`agent.execPrompt`/`shell.exec`/
  `notification.send`). CR-WF-001/002/003 mô tả đúng cơ chế thật đang chạy,
  không giả định `step.execute` tồn tại.
- **`docs/crs/v4/task-graph/`** (series song song, F37): permission/execution
  cho bản thân Task thuộc series đó, không phải series này.

## Nguồn

Từng CR trích dẫn trực tiếp `specs/backend-go/bugs/logic-v1/BUG-WF-0N-*.md`,
`specs/backend-go/bugs/task-v1/BUG-TASKV1-006/007-*.md`,
`specs/backend-go/bugs/missing-v1/BUG-030-*.md`,
`specs/frontend/bugs/workflow-orchestration/BUG-FE-WF-001-*.md`,
`specs/frontend/bugs/task-v1/BUG-FE-TASKV1-007-*.md`,
`specs/agent/bugs/task-v1/BUG-AGENT-TASKV1-004-*.md`,
`specs/backend-go/bugs/logic-v1/BUG-PRF-04-*.md` + `solutions/SOL-PRF-04-*.md`.
Xem từng CR để biết bản đồ đầy đủ BUG↔CR.
