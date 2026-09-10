# SQL Schema Catalog — 18 migrations, 25 bảng (`backend/src/main/db/migrations/0001..0018`)

Áp dụng cho **server mode** khi có DB config (SQLite/MySQL/PostgreSQL/TiDB — xem
[01-storage-architecture.md](./01-storage-architecture.md)). Mọi timestamp `BIGINT` là epoch-ms trừ khi ghi chú
khác; mọi cột `TEXT ... DEFAULT '{}'/'[]'` là JSON blob tự do (không được DB validate).

**Cột "Consumer thực tế"** được verify bằng `grep` tên bảng trong toàn bộ `backend/src/main` (ngoài thư mục
`migrations/`) tại thời điểm khảo sát — `✅ Active` = tìm thấy code đọc/ghi bảng; `⚠️ Dormant` = **không** tìm
thấy reader/writer nào trong code hiện tại (có thể do: chưa wiring xong, đã bị thay bằng cơ chế khác, hoặc grep
bỏ sót SQL dựng động — luôn double-check trước khi coi là dead code).

---

## Nhóm A — Legacy Desktop-mode core (migration 0001) — ⚠️ Dormant

> `settings`, `projects`, `repos`, `ssh_targets` — **không tiền tố `orca_`**. Không tìm thấy bất kỳ
> `SELECT/INSERT/UPDATE ... FROM|INTO {settings|projects|repos|ssh_targets}` nào ngoài chính file migration.
> Bị "che" về mặt khái niệm bởi: (a) `orca_projects`/`orca_repos`/`orca_ssh_targets`/`orca_global_settings`
> (migration 0004, có tiền tố, được `SqlStateRepository` dùng thật), và (b) `orca-data.json` ở desktop mode.

| Bảng | Cột | Index/FK |
|---|---|---|
| `settings` | `key TEXT PK`, `value TEXT NOT NULL`, `updated_at TEXT` | — |
| `projects` | `id TEXT PK`, `name TEXT`, `path TEXT`, `created_at/updated_at TEXT` | — |
| `repos` | `id TEXT PK`, `project_id TEXT FK→projects.id ON DELETE CASCADE`, `name TEXT`, `remote_url TEXT`, `created_at/updated_at TEXT` | `idx_repos_project_id` |
| `ssh_targets` | `id TEXT PK`, `alias TEXT UNIQUE`, `host TEXT`, `port INT DEFAULT 22`, `username TEXT`, `key_path TEXT`, `created_at/updated_at TEXT` | — |

---

## Nhóm B — Legacy scaffolding (migrations 0002, 0003) — ⚠️ Dormant

> Comment gốc mô tả đúng ý định nghiệp vụ, nhưng consumer thật hiện nay là **JSON** (`PersistedState.automations`
> /`automationRuns` và `PersistedState.workspaceSession(sByHostId)` trong `orca-data.json`), không phải 2 bảng
> SQL này. `AutomationService` (`automations/service.ts`) nhận `Store` (persistence.ts) trong constructor, **không
> nhận `pool`**.

| Bảng | Cột | Index |
|---|---|---|
| `automations` (0002) | `id TEXT PK`, `project_id TEXT FK→projects.id ON DELETE CASCADE`, `name`, `trigger`, `config TEXT DEFAULT '{}'`, `enabled INT DEFAULT 1`, `created_at/updated_at TEXT` | `idx_automations_project_id`, `idx_automations_enabled` |
| `workspace_sessions` (0003) | `id TEXT PK`, `project_id TEXT FK→projects.id CASCADE`, `repo_id TEXT FK→repos.id SET NULL`, `agent TEXT`, `status TEXT DEFAULT 'active'`, `started_at/ended_at TEXT`, `metadata TEXT DEFAULT '{}'` | `idx_ws_sessions_project_id`, `_status`, `_agent` |

---

## Nhóm C — Server-mode app tables (migration 0004) — ✅ Active

> `orca_*` — tách biệt có chủ đích khỏi `projects`/`repos` (nhóm A) để phân định rõ "server-mode state" vs
> "system state" (comment migration). Đây là backing store thật của `SqlStateRepository` (§6, file 01) — mỗi
> entity lưu nguyên khối JSON trong cột `data`, chỉ tách vài cột để index.

| Bảng | Cột | Index |
|---|---|---|
| `orca_projects` | `id TEXT PK`, `name TEXT`, `tab_order INT DEFAULT 0`, `data TEXT DEFAULT '{}'`, `created_at TEXT` | `idx_orca_projects_tab_order` |
| `orca_repos` | `id TEXT PK`, `project_id TEXT`, `data TEXT DEFAULT '{}'`, `created_at TEXT` | — |
| `orca_ssh_targets` | `id TEXT PK`, `label`, `host`, `port INT DEFAULT 22`, `username`, `data TEXT DEFAULT '{}'`, `created_at TEXT` | — |
| `orca_global_settings` | `key TEXT PK`, `value TEXT`, `updated_at TEXT` | dùng 1 row cố định `key='app_settings'` |

---

## Nhóm D — Auth & RBAC (migration 0005) — ✅ Active (`AuthManager`)

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_users` | `id TEXT PK`, `email TEXT UNIQUE`, `name`, `password_hash TEXT?`, `role TEXT DEFAULT 'developer'`, `provider TEXT DEFAULT 'none'`, `provider_user_id?`, `avatar_url?`, `teams TEXT DEFAULT '[]'`, `projects TEXT DEFAULT '[]'`, `created_at BIGINT`, `last_login_at BIGINT?`, `is_active INT DEFAULT 1`, *(+`department_id` từ 0006, +`profile_json` — xem nhóm E)* | — |
| `orca_sessions` | `session_id TEXT PK`, `user_id TEXT FK→orca_users CASCADE`, `created_at/expires_at BIGINT`, `last_seen_at BIGINT?`, `ip_address?`, `user_agent?` | `idx_orca_sessions_user`, `_expires` |
| `orca_audit_log` | `id AUTOINCREMENT PK`, `created_at BIGINT`, `user_id?`, `user_email?`, `action TEXT`, `detail?`, `ip_address?` | `idx_orca_audit_user(user_id,created_at DESC)`, `_action(action,created_at DESC)` — **append-only, bất biến** |
| `orca_access_policies` | `id TEXT PK`, `name`, `teams/roles/users TEXT DEFAULT '[]'` (JSON array), `allowed_servers/allowed_projects TEXT DEFAULT '"*"'`, `agent_trust TEXT DEFAULT 'standard'`, `can_create_worktrees/can_delete_worktrees INT DEFAULT 1`, `can_access_production INT DEFAULT 0`, `created_at/updated_at BIGINT` | RBAC policy definition (không FK cứng vào users — áp theo team/role/user list) |

---

## Nhóm E — Profile Hierarchy: Company → Department → User (migration 0006) — ✅ Active (`ProfileService`/`ProfileResolver`)

3-tier cascade merge (Company → Department → **Team** *(thêm sau, nhóm L)* → User), mỗi tầng có `profile_json`
kiểu `OrcaProfile` (xem [06-shared-domain-types.md](./06-shared-domain-types.md) §Profile). Merge: user thắng >
dept thắng > company (trừ section `security` bị khoá ở tầng company).

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_companies` | `id TEXT PK`, `name`, `profile_json TEXT DEFAULT '{}'`, `admin_user_id?`, `created_at/updated_at BIGINT`, `updated_by?` | số nhiều — không phải singleton `id='default'` như ADR-016 đề xuất |
| `orca_departments` | `id TEXT PK`, `company_id TEXT FK→orca_companies CASCADE`, `name`, `parent_dept_id TEXT FK→orca_departments SET NULL` (cây phân cấp), `profile_json TEXT DEFAULT '{}'`, `created_at/updated_at BIGINT`, `updated_by?` | `idx_orca_departments_company` |
| `orca_user_profiles` | `user_id TEXT PK FK→orca_users CASCADE`, `profile_json TEXT DEFAULT '{}'`, `updated_at BIGINT` | 1-1 với user — override cá nhân |
| *(ALTER)* `orca_users.department_id` | `TEXT FK→orca_departments SET NULL`, idempotent (try/catch nếu cột đã tồn tại) | user không set = kế thừa company only |

---

## Nhóm F — Project v5 (migration 0007) — ✅ Active (`ProjectService`)

> Tiền tố `orca_v5_` để **tránh đụng** `orca_projects` (nhóm C, dùng cho tab/state data cũ) — đây là entity
> Project "thật" gắn Dev Server, khác hẳn `Project` client-type trong `orca-data.json` (desktop). 3 khái niệm
> "Project" cùng tồn tại trong hệ thống — xem cảnh báo README.

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_v5_projects` | `id TEXT PK`, `name`, `description?`, `dev_server_id TEXT`, `repo_path TEXT`, `default_branch TEXT DEFAULT 'main'`, `visibility TEXT DEFAULT 'team'`, `created_by TEXT`, `created_at/updated_at BIGINT` | `idx_orca_v5_projects_server` |
| `orca_v5_project_members` | `project_id FK→orca_v5_projects CASCADE`, `user_id FK→orca_users CASCADE`, `role TEXT DEFAULT 'member'`, `added_at BIGINT`, `PK(project_id,user_id)` | `idx_orca_v5_project_members_user` |

---

## Nhóm G — AI Provider Accounts (migrations 0008, +0015 rotation) — ✅ Active (`AIProviderService`/`ProviderHealthChecker`)

Chỉ lưu **metadata** — key thật không bao giờ nằm trong bảng này (xem
[05-credential-secret-stores.md](./05-credential-secret-stores.md)).

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_ai_provider_accounts` | `id TEXT PK`, `dev_server_id TEXT`, `provider TEXT`, `scope TEXT DEFAULT 'server'`, `scope_ref_id?`, `label TEXT`, `model?`, `base_url?`, `status TEXT DEFAULT 'pending'`, `last_health_check BIGINT?`, `quota_limit_day INT DEFAULT 0`, `created_by TEXT`, `created_at/updated_at BIGINT`, *+0015:* `rotation_grace_until BIGINT?` | `idx_orca_ai_providers_server(dev_server_id,status)`, *+0015:* `idx_orca_ai_providers_rotating(status,rotation_grace_until)` |
| `orca_provider_usage` | `id AUTOINCREMENT PK`, `account_id TEXT FK→orca_ai_provider_accounts CASCADE`, `date TEXT`, `tokens_used INT DEFAULT 0`, `requests INT DEFAULT 0`, `cost_usd REAL DEFAULT 0`, `UNIQUE(account_id,date)` | `idx_orca_provider_usage_date(account_id,date DESC)` — 1 row/account/ngày |

`rotation_grace_until`: NULL = không đang xoay key; set bởi `AIProviderService.rotateKey()`, clear bởi
`completeRotation()`; cho phép `ProviderHealthChecker` cron sweep hồi phục account `'rotating'` nếu server
restart giữa chừng (BUG-BE-HLD-014).

---

## Nhóm H — Workflow DAG Engine (migrations 0009, +0013 trace, +0014 pause) — ✅ Active (`WorkflowOrchestrator`/`TemplateResolver`)

DAG-based multi-server workflow orchestration, chạy theo "wave" (nhóm step có thể chạy song song).

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_workflow_templates` | `id TEXT PK`, `name`, `version INT DEFAULT 1`, `parent_template_id TEXT FK→self SET NULL` (kế thừa template), `description?`, `definition_json TEXT DEFAULT '{"steps":[]}'` (kiểu `WorkflowDefinition`), `owner_id?`, `scope TEXT DEFAULT 'user'`, `created_at/updated_at BIGINT` | — |
| `orca_workflow_executions` | `id TEXT PK`, `definition_snapshot TEXT` (đóng băng definition lúc chạy), `status TEXT DEFAULT 'pending'`, `inputs_json TEXT DEFAULT '{}'`, `current_wave INT DEFAULT 0`, `triggered_by TEXT`, `project_id?`, `started_at/completed_at BIGINT?`, `error_message?`, `created_at BIGINT`, *+0013:* `root_trace_id TEXT?`, *+0014:* `paused_at BIGINT?` | `idx_orca_workflow_exec_status(status,created_at DESC)`, `_project(project_id,status)` |
| `orca_workflow_step_executions` | `id TEXT PK`, `execution_id TEXT FK→orca_workflow_executions CASCADE`, `step_id TEXT`, `status TEXT DEFAULT 'pending'`, `started_at/completed_at BIGINT?`, `output_json?`, `error_message?` | `idx_orca_step_exec_execution(execution_id,step_id)` |

- `root_trace_id` (0013): sống sót qua restart để `resumeRunningExecutions()` tái tạo đúng parent trace span —
  nếu không, TracePanel mất khả năng nhóm step trước/sau restart vào cùng 1 execution (CR-TRACE-017 §3.1).
- `paused_at` (0014): nullable, "paused since" cho pause/resume do user trigger; clear về NULL bởi
  `WorkflowOrchestrator.resumeFromPause()` (BUG-BE-HLD-009).

---

## Nhóm I — Task Graph (migration 0010, +0016 linkage) — ✅ Active (`TaskService`/`TaskGrantService`/`TaskOrchestrationBridge`)

Kanban/task-tree với dual-edge DAG: **cây cha-con** (`parent_id`) + **cạnh phụ thuộc** (`orca_task_edges`,
tách biệt khỏi cây phân cấp).

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_tasks` | `id TEXT PK`, `project_id TEXT FK→orca_v5_projects SET NULL`, `parent_id TEXT FK→self CASCADE`, `title`, `description?`, `type TEXT DEFAULT 'task'`, `status TEXT DEFAULT 'backlog'`, `priority TEXT DEFAULT 'medium'`, `labels TEXT DEFAULT '[]'`, `visibility TEXT DEFAULT 'team'`, `reporter_id?`, `assignee_id?`, `estimated_hours REAL?`, `progress_percent INT DEFAULT 0`, `ai_context?`, `prompt_template?`, `due_date BIGINT?`, `created_at/updated_at BIGINT`, *+0016:* `active_execution_task_id TEXT?`, `agent_session_id TEXT?` | `idx_orca_tasks_project(project_id,status)`, `_parent`, `_assignee(assignee_id,status)` |
| `orca_task_edges` | `from_task_id/to_task_id FK→orca_tasks CASCADE`, `edge_type TEXT DEFAULT 'depends_on'`, `created_at BIGINT`, `PK(from_task_id,to_task_id,edge_type)` | `idx_orca_task_edges_from`, `_to` |
| `orca_task_grants` | `id TEXT PK`, `task_id FK→orca_tasks CASCADE`, `scope TEXT`, `scope_id?`, `permission TEXT`, `apply_tree INT DEFAULT 0` (kế thừa xuống subtask), `granted_by TEXT`, `expires_at BIGINT?`, `created_at BIGINT` | `idx_orca_task_grants_task`, `_scope(scope,scope_id)` — resolve quyền qua **BFS ancestor** |
| `orca_task_comments` | `id AUTOINCREMENT PK`, `task_id FK→orca_tasks CASCADE`, `user_id TEXT`, `content TEXT`, `type TEXT DEFAULT 'comment'`, `created_at BIGINT` | `idx_orca_task_comments_task(task_id,created_at DESC)` — append-only |
| `orca_team_members` *(bảng gốc — không nhầm với `orca_teams` ở nhóm L)* | `team_id TEXT`, `user_id FK→orca_users CASCADE`, `role TEXT DEFAULT 'member'`, `added_at BIGINT`, `PK(team_id,user_id)`, *+0016:* `priority INT DEFAULT 0` | `idx_orca_team_members_user` — dùng cho grant-scope resolution |

`active_execution_task_id` / `agent_session_id` (0016) — liên kết pipeline **Source→Plan→Execute**:
- `active_execution_task_id`: set khi dispatch qua **"complex path"** — trỏ tới `TaskRow.id` bên trong
  `orchestration.db` (SQLite riêng, **id space khác hẳn** — xem [04](./04-orchestration-db.md)). Đây **không phải
  FK SQL thật** (khác database).
- `agent_session_id`: set khi dispatch qua **"simple path"** (1 agent session đơn, không qua coordinator).

`orca_task_grants.granted_by` và `orca_annotations.author_id` cố ý là `TEXT` thường (không FK cứng vào
`orca_users`) — vì `RpcContext.userId` không phải lúc nào cũng được điền bởi mọi transport; FK cứng sẽ reject
insert khi caller không có 1 row users sạch.

---

## Nhóm J — Terminal Session Persistence (migration 0011) — ⚠️ Dormant

> Comment migration: "Persist terminal scrollback + metadata across disconnects" (TDD-FE §TM-003). Không tìm
> thấy reader/writer nào cho `orca_terminal_sessions` trong code hiện tại — terminal scrollback ở desktop mode
> hiện được lưu qua `TerminalLayoutSnapshot` trong `orca-data.json` (xem file 03/06), không qua bảng này.

| Bảng | Cột | Index |
|---|---|---|
| `orca_terminal_sessions` | `id TEXT PK`, `worktree_id/tab_id TEXT`, `leaf_id TEXT DEFAULT ''`, `runtime_env_id TEXT DEFAULT ''` (khoá tự nhiên composite), `snapshot_data TEXT?` (base64 xterm SerializeAddon), `snapshot_cols INT DEFAULT 80`, `snapshot_rows INT DEFAULT 24`, `remote_handle?` (re-attach), `status TEXT DEFAULT 'active'`, `last_active_at/created_at/updated_at BIGINT` | UNIQUE `(worktree_id,tab_id,leaf_id,runtime_env_id)`, `_active(status,last_active_at DESC)`, `_worktree(worktree_id,status)` |

---

## Nhóm K — Port Forwards + Push Subscriptions (migration 0012) — ⚠️ Dormant (cả 2 bảng)

> `orca_port_forwards` — comment: fix BUG-BE-SSH-002 "in-memory only — data lost on restart". Không tìm thấy
> consumer. `SavedPortForward` (auto-restore forward) hiện lồng trong `SshTarget.portForwards` bên trong
> `orca-data.json` — khác cơ chế hoàn toàn với bảng này.
>
> `orca_push_subscriptions` — comment: TASK-MB-001, "required by MobileCompanionService". Đã verify:
> `WebPushManager` (`notifications/web-push-manager.ts`) nhận `Store` (persistence.ts) trong constructor và gọi
> `store.getWebPushSubscriptions()`/`setWebPushSubscriptions()`/`getVapidKeys()` — **đọc/ghi `orca-data.json`**,
> không đụng tới bảng SQL này.

| Bảng | Cột | Index |
|---|---|---|
| `orca_port_forwards` | `id TEXT PK`, `host_id TEXT`, `local_port INT`, `remote_host TEXT DEFAULT 'localhost'`, `remote_port INT`, `label TEXT DEFAULT ''`, `active INT DEFAULT 1`, `created_at BIGINT`, `closed_at BIGINT?` | `idx_orca_port_forwards_host(host_id,active)`, `_active(active,created_at DESC)` |
| `orca_push_subscriptions` | `user_id TEXT`, `endpoint TEXT PK`, `auth TEXT`, `p256dh TEXT` (ECDH keys, RFC 8030), `updated_at BIGINT` | `idx_orca_push_user(user_id,updated_at DESC)` |

---

## Nhóm L — Team, OrcaProject Sharing, Task-Exec linkage (migration 0016) — ✅ Active (một phần)

| Bảng | Cột | Index/FK | Consumer |
|---|---|---|---|
| `orca_teams` | `id TEXT PK`, `name`, `created_at/updated_at BIGINT`, *+0017:* `profile_json TEXT DEFAULT '{}'` | — | `ProfileService` (metadata + priority tiebreak) |
| `orca_project_source_projects` | `orca_project_id FK→orca_v5_projects CASCADE`, `owner_user_id FK→orca_users CASCADE`, `project_id TEXT` (**logic FK** sang `Project.id` trong file JSON per-user, KHÔNG phải SQL FK — vì Project data desktop nằm trong JSON, không SQL), `created_at BIGINT`, `PK(orca_project_id,owner_user_id,project_id)` | `idx_orca_project_source_projects_orca_project` | ✅ `OrcaProjectSourceProjectService` (verify bằng grep) |

`orca_teams` **không có** `department_id`/`parent_id` — 1 Team **không** thuộc về 1 department cụ thể theo
thiết kế (`docs/guides/profile/user-profile-team-department-rbac.md §5.2`); độc lập với `orca_team_members` (nhóm I,
tạo từ migration 0010, không đụng tới bởi 0016 ngoại trừ thêm cột `priority`).

---

## Nhóm M — Team Profile Storage (migration 0017) — ✅ Active (mở rộng nhóm L)

`ALTER TABLE orca_teams ADD COLUMN profile_json TEXT NOT NULL DEFAULT '{}'` (idempotent, try/catch) — mirror
`orca_companies.profile_json`/`orca_departments.profile_json`, cho phép Team tham gia cascade-merge (không có
section `security` riêng — khoá ở tầng company).

---

## Nhóm N — Code Review Annotations (migration 0018) — ✅ Active (`code-review/annotation-store.ts`, RPC `annotation.*`)

> Fix BL-CR-02: `annotation-panel.tsx` đã gọi RPC `annotation.list`/`annotation.create` nhưng chưa có bảng backing.

| Bảng | Cột | Index/FK |
|---|---|---|
| `orca_annotations` | `id TEXT PK` (client-generated UUID — entity đọc lại theo id, khác `orca_task_comments` là append-only log), `project_id FK→orca_v5_projects CASCADE`, `review_id?`, `file_path TEXT`, `line_number INT`, `body TEXT`, `author_id TEXT` (plain TEXT, không FK — cùng lý do `orca_task_grants.granted_by`), `created_at BIGINT` | `idx_orca_annotations_lookup(project_id,file_path,line_number)` — lookup theo đúng tuple `annotation-panel.tsx` dùng khi click dòng; `idx_orca_annotations_review(review_id)` |

---

## Tổng hợp trạng thái consumer

| Trạng thái | Bảng |
|---|---|
| ✅ Active | `orca_projects`, `orca_repos`, `orca_ssh_targets`, `orca_global_settings`, `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`, `orca_companies`, `orca_departments`, `orca_user_profiles`, `orca_v5_projects`, `orca_v5_project_members`, `orca_ai_provider_accounts`, `orca_provider_usage`, `orca_workflow_templates`, `orca_workflow_executions`, `orca_workflow_step_executions`, `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`, `orca_team_members`, `orca_teams`, `orca_project_source_projects`, `orca_annotations` |
| ⚠️ Dormant (không tìm thấy consumer bằng grep) | `settings`, `projects`, `repos`, `ssh_targets` (0001), `automations` (0002), `workspace_sessions` (0003), `orca_terminal_sessions` (0011), `orca_port_forwards`, `orca_push_subscriptions` (0012) |

**9/25 bảng** (36%) hiện không có consumer xác định được qua grep tĩnh. Trước khi xoá/sửa các bảng này, hãy
chạy `gitnexus impact` hoặc grep sâu hơn (kể cả SQL dựng động qua template string phức tạp) để chắc chắn —
tài liệu này chỉ phản ánh kết quả grep tại thời điểm khảo sát.
