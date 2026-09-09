# Task Graph Management (F37) — Change Requests (v4)

> **Bối cảnh:** [`docs/roadmap/feature-completion-matrix.md`](../../../roadmap/feature-completion-matrix.md)
> xếp F37 là Backend-go ✅ / Agent ✅ / Frontend 🟡 ("thiếu Board view + Grant/Share
> modal"). Khảo sát sâu hơn cho series này (đối chiếu trực tiếp với code +
> `specs/backend-go/bugs/logic-v1/BUG-TG-01..04`, `specs/backend-go/bugs/task-v1/
> BUG-TASKV1-001..005`, `specs/frontend/bugs/task-v1/BUG-FE-TASKV1-001..006`,
> `specs/agent/bugs/task-v1/BUG-AGENT-TASKV1-001..003`) cho thấy bức tranh "✅"
> ở Backend-go/Agent trong ma trận là đúng ở tầng **kiến trúc tồn tại** (RPC có
> thật, không phải toàn bộ stub) nhưng **sai ở tầng hoàn thiện nghiệp vụ**:
> data model chỉ có 6/~20 field, AI decompose không tạo dependency edge,
> `ComplexExecutor` là stub cứng (`stub-orchestration-exec:...`), grant
> team-scope resolver trả `nil` cứng.
>
> Series CR này **không thiết kế lại từ đầu** — phần lớn gap ở backend-go đã
> có solution doc đầy đủ (`SOL-TG-01..04`) kèm task breakdown
> (`TASK-TG-0N-NN`) trong `specs/backend-go/bugs/logic-v1/solutions/` — các CR
> dưới đây **áp dụng trực tiếp** các thiết kế đó, chỉ thiết kế mới phần chưa
> từng có solution doc (orchestration-service's coordinator run lifecycle —
> CR-TG-004) và toàn bộ phần frontend UI (CR-TG-007).
>
> Không lặp lại nội dung đã có ở [`docs/crs/v3/flow-task/`](../../v3/flow-task/README.md)
> — series đó xử lý **kiến trúc liên kết 3-engine** (OrcaTask/Task Execute/
> Workflow dùng chung 1 mô hình `ExecutionEngine`); series này xử lý **bản
> thân F37 hoàn thiện đến đâu** trong từng engine nó sở hữu trực tiếp (Engine 1
> Direct Agent execution, cộng phần Engine 2 Task Execute mà F37 phụ thuộc để
> chạy task có subtask/dependency).

## Bảng CR

| CR | Vấn đề | Giải pháp | Layer | Priority | Status |
|----|--------|-----------|-------|----------|--------|
| [CR-TG-001](./CR-TG-001-orcatask-data-model-widening.md) | `Task` chỉ có 6/~20 field, 4/7 status, không có progress cascade, `AddEdge` có race cycle-check | Mở rộng schema/proto, `GetSubtree`, `RecalculateProgress`, auto-block, comment CRUD, atomic `AddEdge` (theo SOL-TG-01) | Backend-go | P0 | 🔵 Proposed |
| [CR-TG-002](./CR-TG-002-ai-decompose-context-and-dependency-edges.md) | AI decompose chỉ thấy `task.Title`, proposal chỉ có `Title`, không tạo `depends_on` edge, không có critical path | Context bundle 5 nguồn, structured JSON proposal, dependency edges, critical path, "Generate Agent Prompt" (theo SOL-TG-02) | Backend-go | P1 | 🔵 Proposed |
| [CR-TG-003](./CR-TG-003-task-access-control-team-scope-and-sharing.md) | `StubTeamScopeResolver` trả `nil` cứng — mọi grant scope=team chết; không có revoke/expiry/share-link | `TeamScopeResolver` thật, `RevokeGrant`/`ListGrants`, expiry, share-link, notification (theo SOL-TG-03) | Backend-go | P0 | 🔵 Proposed |
| [CR-TG-004](./CR-TG-004-orchestration-service-coordinator-run-lifecycle.md) | `orchestration-service` thiếu 6/11 RPC theo TDD, không có vòng lặp nền nào — `orchestration.messages` là dead schema | Thiết kế + build `StartCoordinatorRun`/`GetCoordinatorRun`/.../ tick loop | Backend-go | P0 | 🔵 Proposed |
| [CR-TG-005](./CR-TG-005-task-agent-execution-permission-and-complex-executor.md) | `ExecuteTask` không precheck permission, không revert status khi fail; `ComplexExecutor` là stub cứng; context/env injection sai | Permission precheck, real `ComplexExecutor`, callback, context preamble, env injection đúng (theo SOL-TG-04) | Backend-go + Agent | P0 | 🔵 Proposed |
| [CR-TG-006](./CR-TG-006-task-execute-streaming-relay.md) | 4 RPC agent gọi (`agent.execPrompt`/`exec`/`shell.exec`/`ai.complete`) đều buffer-toàn-bộ-rồi-trả-1-lần; Task Execute (Engine 2) dispatch không có kênh nhận output/exit | Streaming RPC mới ở `infra-fleet-service` (theo mẫu `AttachPty`) + `chunk` notification ở `agent/` | Backend-go + Agent | P1 | 🔵 Proposed |
| [CR-TG-007](./CR-TG-007-frontend-task-crud-board-grant-ui.md) | Không có "New Task" UI, `TaskDAGView` luôn rỗng, không có Board view, không có Grant/Share UI, "Run with Agent" prompt bị bỏ qua | Task CRUD dialog, sửa DAG view dùng data thật, **Board/Kanban view mới**, **Grant/Share modal mới**, sửa Run-Agent UX, `useTaskActivity` | Frontend | P0 | 🔵 Proposed |

## Thứ tự thực thi

```
CR-TG-001 (data model — nền tảng cho mọi CR khác cần field mới)
  ├── CR-TG-002 (AI decompose — cần description/aiContext/estimatedHours/promptTemplate)
  ├── CR-TG-003 (access control — cần ownerId cho owner short-circuit)
  └── CR-TG-004 (orchestration coordinator — độc lập kỹ thuật, không cần field mới)
        └── CR-TG-005 (real ComplexExecutor — cần StartCoordinatorRun tồn tại + field CR-TG-001)
              └── CR-TG-006 (streaming — cần 1 dispatch path thật để stream từ đó)
CR-TG-007 (frontend) — có thể bắt đầu song song ngay khi RPC shape của 001/002/003 chốt,
  nhưng chức năng đầy đủ (Run Agent thấy activity thật, Grant UI gọi đúng RPC) phụ thuộc
  001 → 005 lần lượt hoàn thành. Không chờ CR-TG-006 — Activity Feed dùng
  CR-FLOW-TASK-003's kênh event rời rạc trước, streaming liên tục là nâng cấp sau.
```

## Không thuộc phạm vi series này (đã có CR/thiết kế riêng — không lặp lại)

- **`docs/crs/v3/flow-task/`** (CR-FLOW-TASK-001..005): mô hình `ExecutionEngine`
  hợp nhất 3 hệ, liên kết Task↔Workflow (Engine 3), event catalog hợp nhất
  `task.activity`, kế hoạch cutover Node→backend-go, UI "Run" chọn engine. CR-TG-007
  **áp dụng** `useTaskActivity`/engine badge của CR-FLOW-TASK-005, không thiết kế lại.
- **`docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md`**: tracer/observability
  cho **kiến trúc `backend/` legacy** (`src/main/task/*.ts`) — mục tiêu file đã lỗi
  thời so với backend-go, P3, không phải gap chức năng. Không re-scope ở đây.
- **`docs/crs/v2/dev-server/CR-DS-001/003`**: định nghĩa pattern "Dispatch + Stream"
  cho F37 ở tầng kiến trúc chung Dev Server Agent — CR-TG-006 tuân theo pattern này,
  không phát minh lại.
- **`docs/crs/v4/workflow/`** (series song song, F36): khi task chạy qua
  `workflow_template_id` (CR-FLOW-TASK-002's Engine 3), phần thực thi thuộc về
  `docs/crs/v4/workflow/CR-WF-*`, không phải series này.

## Nguồn

Từng CR trích dẫn trực tiếp `specs/backend-go/bugs/logic-v1/BUG-TG-0N-*.md` +
`solutions/SOL-TG-0N-*.md` (+ `specs/backend-go/bugs/task-v1/BUG-TASKV1-00N-*.md`
cho phần re-audit 2026-09-08), `specs/frontend/bugs/task-v1/BUG-FE-TASKV1-00N-*.md`,
`specs/agent/bugs/task-v1/BUG-AGENT-TASKV1-00N-*.md`. Xem từng CR để biết bản đồ
đầy đủ BUG↔CR.
