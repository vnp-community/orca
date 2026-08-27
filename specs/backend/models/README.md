# Backend Data Models — Tổng quan

> **Nguồn:** khảo sát trực tiếp mã nguồn `backend/src/main/db/**`, `backend/src/main/repositories/**`,
> `backend/src/main/persistence.ts`, `backend/src/main/runtime/orchestration/**`, các service theo domain
> (`profile/`, `project/`, `ai-providers/`, `workflow/`, `task/`, `automations/`, `credentials/`), và
> `backend/src/shared/types.ts` / `ssh-types.ts` / `automations-types.ts` / `ai-provider-types.ts` / `team-types.ts`.
> **Ngày khảo sát:** 2026-08-15, branch `fix/pty-session-expired-on-pane-remount`. Đây là snapshot — code thay
> đổi liên tục, hãy đối chiếu lại migration/service tương ứng trước khi dựa vào tài liệu này để ra quyết định.
>
> ⚠️ `docs/adrs/v2/ADR-016-db-migrations-0006-0010-schema.md` (schema đề xuất) **không khớp** với migrations
> 0006–0010 thật trong code (tự ADR đó đã cảnh báo "Migration Numbering Collision"). Tài liệu này lấy migration
> code thật (`backend/src/main/db/migrations/0001..0018`) làm nguồn chân lý duy nhất, không dùng ADR-016.

## Phát hiện chính

Orca **không có một "database" duy nhất**. Có **5 cơ chế lưu trữ độc lập, song song**, được chọn theo runtime
mode (Electron desktop vs. Node.js server mode) và theo domain nghiệp vụ:

| # | Cơ chế | File(s) | Runtime mode | Domain phục vụ |
|---|--------|---------|---------------|-----------------|
| 1 | **JSON file lớn — `orca-data.json`** (`Store` / `persistence.ts`) | 1 file JSON + 5 bản backup xoay vòng + vài sidecar file | Electron/Desktop (mặc định) | ~30 domain con: Project/Repo/SSH/Worktree/Tab/Terminal/Browser/Automation/Settings/UI state... (xem [03](./03-electron-desktop-store.md)) |
| 2 | **SQL đa dialect** (`orca.db` hoặc MySQL/Postgres/TiDB) qua `IConnectionPool` | `db/migrations/0001..0018` → 25 bảng `orca_*` | Server mode (`ORCA_MULTI_USER=1` hoặc có `ORCA_DB_URL`) | Auth/RBAC, Profile hierarchy, Project v5, AI Providers, Workflow DAG, Task Graph, Team, Annotations (xem [02](./02-sql-schema-catalog.md)) |
| 3 | **JSON file nhỏ — `store.json`** (`JsonFileStateRepository`) | 1 file JSON, chỉ 4 field | Server mode fallback (không có DB config) | Chỉ Project/Repo/SshTarget/GlobalSettings — subset tối giản của #1, dùng `IStateRepository` interface (xem [01](./01-storage-architecture.md)) |
| 4 | **SQLite riêng biệt — `orchestration.db`** (`OrchestrationDb`) | 1 file SQLite khác, tách biệt hoàn toàn khỏi #2 | Cả 2 mode (lazy, chỉ khi dùng multi-agent coordinator) | Pipeline điều phối multi-agent "complex path": `messages`, `tasks` (TaskRow), `dispatch_contexts`, `decision_gates`, `coordinator_runs` (xem [04](./04-orchestration-db.md)) |
| 5 | **Kho credential/secret rời rạc** (5 cơ chế khác nhau) | nhiều file mã hoá + OS Keychain | Cả 2 mode, theo loại secret | AES-256-GCM per-user (`WebCredentialStore`), Electron `safeStorage`, OS Keychain (Claude/Codex CLI), encrypted-blob-qua-relay (AI provider key), in-memory-only (SSH passphrase) (xem [05](./05-credential-secret-stores.md)) |

Ngoài ra còn 2 kho JSON độc lập cho usage tracking (`orca-claude-usage.json`, `orca-codex-usage.json`) — xem [07](./07-usage-tracking-stores.md).

**Hệ quả quan trọng cho luồng nghiệp vụ:** một entity nghiệp vụ (vd. "Project") có thể tồn tại ở **nhiều nơi
khác nhau với schema khác nhau** tuỳ runtime mode — `Project` (client type, `shared/types.ts:108`) sống trong
`orca-data.json` ở desktop mode, trong bảng SQL `orca_projects`/`orca_v5_projects` (hai bảng khác nhau!) ở
server mode. Xem cảnh báo chi tiết trong từng file.

## Mục lục

| File | Nội dung |
|------|----------|
| [01-storage-architecture.md](./01-storage-architecture.md) | Lớp trừu tượng DB đa dialect (`IDatabase`/`IConnectionPool`/`DatabaseProvider`), connection pooling, migration runner, repository pattern (`IStateRepository`), cách server-bootstrap chọn backend |
| [02-sql-schema-catalog.md](./02-sql-schema-catalog.md) | Danh mục đầy đủ 25 bảng SQL từ 18 migrations — field, index, FK, service sở hữu, **trạng thái consumer thực tế đã verify bằng grep** (active/dormant) |
| [03-electron-desktop-store.md](./03-electron-desktop-store.md) | `orca-data.json` (Electron `Store`) — full shape `PersistedState` (~30 field gốc), backup/sidecar files, cơ chế mã hoá từng field, quan hệ với `JsonFileStateRepository` |
| [04-orchestration-db.md](./04-orchestration-db.md) | `orchestration.db` — kho riêng cho pipeline multi-agent coordinator (Source→Plan→Execute "complex path") |
| [05-credential-secret-stores.md](./05-credential-secret-stores.md) | 5 cơ chế lưu credential/secret khác nhau, theo loại secret và runtime mode |
| [06-shared-domain-types.md](./06-shared-domain-types.md) | Danh mục type nghiệp vụ trong `backend/src/shared/*.ts` — Project/Repo/SSH, Worktree/Workspace, Tab/Terminal/Browser, Team, AI Provider, Workflow/Automation, Task (PR/Issue mirror), `GlobalSettings` (~230 field), `PersistedState` root |
| [07-usage-tracking-stores.md](./07-usage-tracking-stores.md) | `orca-claude-usage.json` / `orca-codex-usage.json` — kho JSON riêng theo dõi token/cost usage |
| [08-postgres-microservices-target-architecture.md](./08-postgres-microservices-target-architecture.md) | **Kiến trúc đích:** hợp nhất toàn bộ server-mode data plane vào Postgres (TiDB-ready), database-per-service, multi-tenant, lộ trình 5 phase — xem [ADR-021](../../../docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md) |

## Cách đọc nhanh theo luồng nghiệp vụ

> ⚠️ Bảng dưới là **snapshot khảo sát ban đầu** (trước ADR-021). Từ đó, Automation/WebPush/Task Graph
> "complex path"/toàn bộ `Store` (dòng "Project/Repo/SSH/Worktree/Tab/Terminal") **đã cutover sang Postgres
> trong server mode** — xem trạng thái thật + cơ chế cutover ở
> [08-postgres-microservices-target-architecture.md](./08-postgres-microservices-target-architecture.md).
> Các dòng "JSON (`orca-data.json`)"/"SQLite riêng" bên dưới vẫn đúng cho **Electron Desktop mode**.

| Luồng nghiệp vụ | Model chính | Nơi lưu (server mode) | Service |
|---|---|---|---|
| Đăng nhập, RBAC, audit | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` | SQL | `AuthManager` |
| Company → Dept → User profile cascade-merge | `orca_companies`, `orca_departments`, `orca_user_profiles`, `orca_teams.profile_json` | SQL | `ProfileService` / `ProfileResolver` |
| Project ↔ Dev Server binding, membership | `orca_v5_projects`, `orca_v5_project_members` | SQL | `ProjectService` |
| Chia sẻ Project (desktop) vào OrcaProject (server) | `orca_project_source_projects` | SQL (logic FK sang JSON) | `OrcaProjectSourceProjectService` |
| Quản lý AI provider account + usage + xoay key | `orca_ai_provider_accounts`, `orca_provider_usage` | SQL | `AIProviderService` / `ProviderHealthChecker` |
| Workflow DAG (template → execution → step) | `orca_workflow_templates`, `orca_workflow_executions`, `orca_workflow_step_executions` | SQL | `WorkflowOrchestrator` / `TemplateResolver` |
| Task Graph (kanban, grant, comment) | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`, `orca_team_members` | SQL | `TaskService` / `TaskGrantService` |
| Task Graph → dispatch multi-agent ("complex path") | `TaskRow` trong `orchestration.db` (khác id space với `orca_tasks`) | SQLite riêng | `TaskOrchestrationBridge` / `Coordinator` |
| Inline code-review annotation | `orca_annotations` | SQL | `annotation-store.ts` / RPC `annotation.*` |
| Automation (scheduled agent prompt) | `Automation` / `AutomationRun` | **JSON** (`orca-data.json`, KHÔNG phải bảng SQL `automations`) | `AutomationService` (nhận `Store`, không nhận `pool`) |
| Web Push subscription | `WebPushSubscription` / `vapidKeys` | **JSON** (`orca-data.json`, KHÔNG phải bảng SQL `orca_push_subscriptions`) | `WebPushManager` (nhận `Store`) |
| Project/Repo/SSH/Worktree/Tab/Terminal (desktop) | `Project`, `Repo`, `SshTarget`, `Worktree`, `Tab`, ... | JSON (`orca-data.json`) | `persistence.ts` `Store` |
| Credential AI CLI (Claude/Codex OAuth) | — | OS Keychain | `desktop/src/main/claude-accounts/keychain.ts` |
| SSH port-forward (đã lưu để auto-restore) | `SavedPortForward` lồng trong `SshTarget.portForwards` | JSON (`orca-data.json`) | — (KHÔNG dùng bảng SQL `orca_port_forwards`, xem [02](./02-sql-schema-catalog.md) mục "dormant") |

Chi tiết field-level, index, FK và bằng chứng "active/dormant" nằm trong từng file con.
