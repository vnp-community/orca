# Task, Automation, AI Orchestration: 3 hệ thống "Task" tách biệt, và đề xuất liên kết

**Cập nhật:** 2026-08-13

> Nối tiếp [project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) —
> đối chiếu [F37 — Task Graph Management](../features/F37-task-graph-management.md) và
> [F14 — Automations](../features/F14-automations.md) với code thật, xác định vị trí AI
> orchestration, và đề xuất phương án liên kết.

## 1. "Task" là 3 khái niệm khác nhau trong codebase — không phải 1

| | Nguồn | Trạng thái | Phạm vi |
|---|---|---|---|
| **Task (Source)** | `TaskPage.tsx` (8278 dòng, render qua `App.tsx`), `task-source-context.ts` | ✅ **Thật, đang chạy** | Work item từ tracker ngoài: GitHub Issues / GitLab / Linear / Jira |
| **Task (Plan)** — `OrcaTask` | [F37](../features/F37-task-graph-management.md), `shared/task-types.ts` | ❌ **Gần như chưa xây** | Đồ thị phân rã việc nội bộ, cấp company, bền vững |
| **Task (Execute)** — `TaskRow` | `main/runtime/orchestration/` (`coordinator.ts`, `db.ts`) | ✅ **Thật, đang chạy** | Sub-task tạm thời trong 1 phiên multi-agent (lead dispatch cho worker) |

## 2. F37 (Task Graph): đối chiếu doc vs code

| File F37 liệt kê | Thực tế |
|---|---|
| `shared/task-types.ts` | ✅ có — nhưng `OrcaTask` không được dùng ở bất kỳ đâu ngoài chính file này |
| `main/task/task-service.ts`, `task-graph-builder.ts`, `task-dag-validator.ts`, `task-ai-planner.ts`, `task-grant-service.ts`, `task-agent-executor.ts` | **Không tồn tại — không 1 file nào** |
| `rpc/methods/tasks.ts` | **Không tồn tại** |
| `TaskGraphView.tsx`, `TaskBoardView.tsx`, `TaskDetailPanel.tsx`, `TaskGrantModal.tsx`, `AIPlanModal.tsx` | **Không tồn tại** |
| `TaskTreeView.tsx` | ✅ có, nhưng chỉ được `TaskGraphPanel.tsx` dùng — và `TaskGraphPanel` chỉ được `WorkspaceLayout.tsx` gọi, cụm đã xác nhận **không mount vào UI thật** (xem F38 guide) |

→ F37 cùng số phận với F38: chỉ có type + 1 nhánh UI mồ côi treo dưới cùng 1 cụm chưa mount.

## 3. F14 (Automations): đối chiếu doc vs code — feature khớp nhất trong 3 cái đã tra

Tất cả 6 file shared + `main/automations/` đúng tên như doc liệt kê, `AutomationsPage.tsx`
**render thật** qua `App.tsx`. Khác biệt duy nhất là mức độ chi tiết action model:

| Doc (YAML nhiều action) | Code thật |
|---|---|
| Chuỗi action: `create_worktree → run_agent → commit → create_pr → send_notification → run_script` | **Chỉ 1 hành động ngầm định**: chạy `agentId` với `prompt` trên `workspaceMode`/`baseBranch` — không có mảng `actions[]` |
| Cron string (`0 9 * * 1-5`) | **RRULE thật** (`rrule`/`dtstart`/`timezone`) — mạnh hơn, khác cú pháp doc |
| `send_notification`/`run_script` là action riêng | Không tồn tại trong Automation — notification là tính năng khác (F11) |

```typescript
// Quan hệ Task ↔ Automation CÓ THẬT — nhưng là với Task (Source), không phải OrcaTask
Automation {
  sourceContext?: TaskSourceContext | null   // trỏ tới 1 issue GitHub/Linear/Jira
  runContext?: WorkspaceRunContext | null    // project + host binding
  prompt: string
  agentId: TuiAgent
  rrule, dtstart, timezone                   // lịch chạy
}
```

## 4. AI Orchestration — nằm ở `main/runtime/orchestration/`, có `TaskRow` riêng thứ 3

Hệ điều phối multi-agent **thật, đang chạy**, SQLite-backed:

```typescript
// main/runtime/orchestration/types.ts
TaskRow { id, parent_id, task_title, spec, status, deps, result, created_at, completed_at }
MessageType: 'status'|'dispatch'|'worker_done'|'merge_ready'|'escalation'|'handoff'|'decision_gate'|'heartbeat'
```

Hình dạng (`parent_id` + `deps`) **giống hệt** `OrcaTask` của F37 — nhưng phạm vi khác hẳn:
đây là cơ chế để **1 agent "lead" điều phối nhiều agent "worker" trong CÙNG 1 phiên**
orchestration-skill (lead tự tạo sub-task, dispatch cho worker pane, worker báo `worker_done`,
lead merge kết quả). Terminal render token `task_<id>` agent tự in ra thành link click được
(`terminal-orchestration-task-links.ts`). Không liên quan `TaskPage.tsx` hay F37.

## 5. Đề xuất: coi 3 tầng là 1 pipeline, không phải 3 hệ thống độc lập

```
Tầng 1 — SOURCE (đã có, đang chạy)
  TaskSourceContext (GitHub/Linear/Jira issue) — "cái vé việc cần làm"
       │
       │  ①  MỚI: liên kết optional
       ▼
Tầng 2 — PLAN (chưa xây — chính là F37, nhưng RÚT GỌN)
  OrcaTask { parentId, dependsOn, projectId, promptTemplate, aiContext, status }
  — "kế hoạch/đồ thị phân rã việc, bền vững cấp company"
       │
       │  ②  MỚI: "Run Agent" tạo phiên orchestration, seed từ OrcaTask
       ▼
Tầng 3 — EXECUTE (đã có, đang chạy)
  Orchestration coordinator TaskRow (lead agent dispatch cho worker agent)
  — "việc đang thực thi thật, trong 1 phiên"
       │
       │  ③  MỚI: kết quả/status ghi ngược lên OrcaTask khi phiên xong
       ▼
  (quay lại Tầng 2 — OrcaTask.status cập nhật, không cần polling riêng)

Automation = CÒ SÚNG kích hoạt Tầng 2→3 theo lịch (rrule) — đã hoạt động
  cho Tầng 1→3 trực tiếp hôm nay (sourceContext), mở rộng thêm để cũng
  kích hoạt được Tầng 2→3.
```

### ① `OrcaTask.sourceContext?: TaskSourceContext`

Thêm field optional vào `OrcaTask` khi build tầng 2. Cho phép "AI: Plan this task" lấy
title/description từ 1 issue GitHub/Linear/Jira có sẵn làm gốc phân rã, thay vì gõ lại tay.
Không đổi gì ở `TaskSourceContext`/`TaskPage.tsx` đang chạy.

### ② "Run Agent" trên 1 OrcaTask → tạo phiên orchestration, seed từ OrcaTask

Backend tạo phiên orchestration mới, `coordinator.ts` seed root `TaskRow.spec`/`task_title`
**từ** `OrcaTask.promptTemplate + aiContext + description`, `TaskRow.deps` lấy từ những
`OrcaTask.dependsOn` đã `done`. Lưu `TaskRow.id` vào field mới `OrcaTask.activeExecutionTaskId`.

### ③ Phiên orchestration kết thúc → ghi ngược lên OrcaTask

Khi `worker_done`/`merge_ready` cuối cùng xảy ra, 1 listener ghi ngược `OrcaTask.status`
(`in_progress` → `review`/`done`), `OrcaTask.actualHours`, và có thể `OrcaTask.aiPlanJson` nếu
lead agent tự phân rã thêm sub-task lúc chạy — company-wide task graph tự cập nhật theo thực
thi thật, không cần cơ chế đồng bộ riêng.

### Automation mở rộng nhẹ — không đổi model đang chạy

```typescript
Automation {
  ...
  sourceContext?: TaskSourceContext | null   // ĐÃ CÓ — kích hoạt trực tiếp từ issue ngoài
  taskId?: string | null                     // MỚI — kích hoạt 1 OrcaTask theo lịch (qua bước ②)
}
```
Khi có `taskId`, Automation lấy `prompt`/`agentId` thẳng từ `OrcaTask.promptTemplate` thay vì
tự cầm riêng. Khi có `sourceContext` (như hiện tại), giữ nguyên hành vi cũ.

## 6. Đừng tự phát minh RBAC riêng cho Task — dùng lại `ProjectMember`

F37 doc có `TaskGrant[]`/`visibility` **riêng** cho từng task — đúng loại rủi ro "hệ thống song
song trùng tên" đã gặp nhiều lần trong phiên này (`OrcaProject` vs `Project`, 2 bản
`OrcaProfile`). Đề xuất: **bỏ `TaskGrant`**, để `OrcaTask.projectId` thừa hưởng RBAC từ
`OrcaProject`/`ProjectMember` (đã thiết kế ở
[user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md)) — task thuộc
project nào, quyền xem/sửa theo đúng quyền project đó. Chỉ thêm 1 bảng override mỏng
(`TaskGrant`) cho trường hợp hiếm cần quyền lệch khỏi project cha, không để nó thành cơ chế
chính.

## 7. Thứ tự triển khai — tránh lặp lại lỗi F38 (mount trước, hoàn thiện sau)

1. **Xây `OrcaTask` tối giản** (`parentId`/`dependsOn`/`projectId`/`promptTemplate`/
   `sourceContext`, KHÔNG có `TaskGrant`) — chỉ data model + service CRUD, chưa cần UI.
2. **Nối ②③** — seed orchestration từ OrcaTask + ghi ngược kết quả. Kiểm chứng bằng cách tự
   chạy tay (RPC/script), chưa cần UI.
3. **Mở rộng Automation** thêm `taskId` — tận dụng lại đúng pipeline chạy agent đã có, không
   viết engine action-sequence mới như doc F14 mô tả.
4. **Cuối cùng mới xây UI** (Tree/Board/Graph view F37) — lúc này luồng dữ liệu đã chạy thật
   qua RPC/script, UI chỉ là lớp hiển thị, không có nguy cơ "tab bấm vào trống" như đã thấy ở
   `WorkspaceLayout`.
