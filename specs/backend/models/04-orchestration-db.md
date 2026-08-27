# OrchestrationDb — kho SQLite riêng cho pipeline multi-agent coordinator

`backend/src/main/runtime/orchestration/db.ts`

## 1. Đây là gì?

**SQL-backed, nhưng là 1 file SQLite HOÀN TOÀN TÁCH BIỆT** khỏi DB chính (`db/**`, 25 bảng `orca_*`):

- `better-sqlite3` (qua `sqlite/sync-database`), WAL mode, `SCHEMA_VERSION = 6` với migration runner tự viết
  tay bên trong `db.ts` (`migrate()`), **không dùng chung** `MigrationRunner`/`ALL_MIGRATIONS` của `db/migrations/**`.
- Khởi tạo lazy trong `OrcaRuntimeService.getOrchestrationDb()`, path:
  `join(app.getPath('userData'), 'orchestration.db')` — khác hẳn `orca.db` (chứa `orca_tasks`,
  `orca_workflow_executions`...).
- Có thể khởi tạo `:memory:` cho test.
- Row types: `TaskRow`, `MessageRow`, `DispatchContextRow`, `DecisionGateRow`, `CoordinatorRun`
  (`runtime/orchestration/types.ts`).

## 2. Quan hệ với `orca_tasks` và `WorkflowOrchestrator` — 3 pipeline song song, KHÔNG liên quan nhau

Đây là điểm dễ nhầm nhất trong toàn bộ hệ thống — 3 khái niệm "task/workflow execution" **độc lập hoàn toàn**:

1. **`orca_tasks`** (SQL chính, migration 0010) — kanban/task-graph người dùng thấy trực tiếp.
2. **`OrchestrationDb.tasks` (`TaskRow`)** — DAG task node của **coordinator đa-agent**, sống trong
   `orchestration.db` riêng.
3. **`orca_workflow_executions`/`orca_workflow_step_executions`** (SQL chính, migration 0009) — engine DAG
   workflow (template → execution → step), **không hề tham chiếu** `OrchestrationDb`/`TaskRow` (verify: 0 hit
   khi grep).

**Cầu nối duy nhất** giữa (1) và (2): `TaskOrchestrationBridge.dispatch(taskId)`
(`backend/src/main/task/TaskOrchestrationBridge.ts`):
- Đọc subtree `OrcaTask` từ `TaskService` (bảng SQL `orca_tasks`).
- Với đường **"complex path"**: gieo (`seedTaskRows`) toàn bộ cây `TaskRow` mới bên trong `OrchestrationDb` —
  code tự ghi chú rõ `OrcaTask.id` và `TaskRow.id` là **"different id spaces in different SQLite databases"**
  — **không phải FK SQL thật**, chỉ là liên kết logic qua cột `orca_tasks.active_execution_task_id` (migration
  0016).
- Khởi động `Coordinator` (`runtime/orchestration/coordinator.ts`) chạy hoàn toàn dựa trên `OrchestrationDb`.
- Khi xong, ghi kết quả ngược lại vào row `orca_tasks` (`recordOrchestrationRunCompletion`).

## 3. Các bảng trong `OrchestrationDb` (`db.ts` `createTables()`)

| Bảng | Field chính | Vai trò |
|---|---|---|
| `messages` | `id`, `from_handle`/`to_handle`, `subject`, `body`, `type` (`status\|dispatch\|worker_done\|merge_ready\|escalation\|handoff\|decision_gate\|heartbeat`), `priority`, `thread_id`, `payload`, `read`, `sequence` (PK autoincr), `delivered_at`, `sender_pane_key` | Mailbox giữa các agent — lệnh dispatch, tín hiệu worker hoàn thành, escalation, heartbeat |
| `tasks` (`TaskRow`) | `id` PK (tiền tố `task_*`), `parent_id`, `created_by_terminal_handle`, `task_title`, `display_name`, `spec`, `status` (`pending\|ready\|dispatched\|completed\|failed\|blocked`), `deps` (JSON array id task), `result`, `created_at`, `completed_at` | Node DAG của coordinator — `deps` điều khiển `promoteReadyTasks()` (giải DAG) |
| `dispatch_contexts` (`DispatchContextRow`) | `id`, `task_id`, `assignee_handle`, `assignee_pane_key`, `status` (`pending\|dispatched\|completed\|failed\|circuit_broken`), `failure_count`, `last_failure`, `dispatched_at`, `completed_at`, `last_heartbeat_at` | Theo dõi terminal/worker nào đang chạy task nào; circuit-break sau 3 lần fail; phát hiện dispatch/heartbeat "stale" |
| `decision_gates` (`DecisionGateRow`) | `id`, `task_id`, `question`, `options` (JSON), `status` (`pending\|resolved\|timeout`), `resolution`, `resolved_at` | Checkpoint quyết định người/agent — chặn task tới khi được resolve |
| `coordinator_runs` (`CoordinatorRun`) | `id`, `spec`, `status` (`idle\|running\|completed\|failed`), `coordinator_handle`, `poll_interval_ms`, `completed_at` | 1 row/lượt chạy coordinator (phiên điều phối top-level) |

## 4. Luồng nghiệp vụ

Đây là backend của pipeline **Source → Plan → Execute "complex path"**
(`docs/guides/task-automation-orchestration-integration.md §9.2/§9.4.2/§9.4.4`): 1 task kanban (`orca_tasks`)
được mở rộng thành DAG sub-task trong `OrchestrationDb`, `Coordinator` dispatch task sẵn sàng tới các
terminal-hosted AI agent worker qua mailbox `messages`, theo dõi liveness/failure qua `dispatch_contexts`,
resolve các điểm quyết định con người qua `decision_gates`, và promote task phụ thuộc khi dependency hoàn
thành — cuối cùng báo cáo kết quả tổng hợp ngược lại row `orca_tasks` gốc.
