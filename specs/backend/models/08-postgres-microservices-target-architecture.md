# Target Architecture — Hợp nhất Postgres + Microservices (Server mode)

> Quyết định kiến trúc: [ADR-021](../../../docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md).
> Tài liệu này là bản kế hoạch chi tiết (TDD) — bảng ánh xạ service↔schema, DDL, cơ chế tenant isolation,
> lộ trình triển khai theo phase. Áp dụng cho **Server mode only** (xem README §Phát hiện chính) —
> Electron Desktop giữ nguyên `orca-data.json`.
>
> **Cập nhật — trạng thái triển khai thực tế (không chỉ kế hoạch):**
> `OrchestrationDb` (SQLite riêng), `Store` (core `orca-data.json`), **và** `Automation` **đã cutover xong
> sang Postgres — mặc định**, không còn cờ opt-in — trong server mode, typecheck sạch, không lỗi mới so
> với baseline. Gap `getRepo()`/`getProjectHostSetups()` từng chặn Automation đã đóng: `PgAutomationStore`
> giờ delegate 2 method đó sang `Store` (đã Postgres-hydrated), vì `Repo`/`ProjectHostSetup` vốn nằm trong
> chính blob `PersistedState` mà `Store` đọc/ghi qua Postgres — không phải "không có tương đương server
> mode" như đánh giá ban đầu, mà là "chưa hydrate Store từ Postgres" — nay đã hydrate nên delegate là đúng
> nghĩa, không phải cách chữa cháy. Xem chi tiết ở cuối §5. Phần còn lại ngoài phạm vi: toàn bộ
> token/key/credential (loại trừ theo yêu cầu ban đầu của bạn — xem file 05).

## 1. Nguyên tắc thiết kế

1. **1 database engine (Postgres, TiDB-ready) — N schema, mỗi schema = 1 service.** Không cross-schema FK.
   ID tham chiếu sang service khác lưu dạng giá trị thường ("logical FK"), validate qua gọi API nội bộ.
2. **`tenant_id UUID NOT NULL`** trên mọi bảng thuộc phạm vi 1 tổ chức (company) — không trên bảng có tính
   chất global/system (vd. `schema_migrations`, `credential.rotation_log` theo user chứ không theo tenant).
3. **Cô lập theo Postgres ROLE**: mỗi service có 1 DB role riêng, chỉ được `GRANT` trên đúng schema của nó —
   thực thi "không service nào đọc trực tiếp bảng của service khác" ở tầng DB, không chỉ ở tầng code review.
4. **Application-layer tenant scoping là bắt buộc**, Postgres RLS là lớp phòng vệ thêm — xem ADR-021 mục 3.
5. **Không phá vỡ runtime hiện tại ở Phase 0** — chỉ thêm schema/bảng/cột mới, không xoá gì, không đổi hành
   vi service đang chạy.

## 2. Bảng ánh xạ Service ↔ Schema ↔ Dữ liệu hiện có

| # | Service | Schema | Bảng sở hữu (mới hoặc kế thừa) | Thay thế cho |
|---|---|---|---|---|
| 1 | `auth-service` | `auth` | `users`, `sessions`, `audit_log`, `access_policies` | `orca_users`/`orca_sessions`/`orca_audit_log`/`orca_access_policies` (0005) — rename khi tách schema thật (Phase 3), Phase 0 chỉ thêm `tenant_id` |
| 2 | `tenant-service` | `tenant` | `companies`, `departments`, `user_profiles`, `teams`, `team_members` | `orca_companies`/`orca_departments`/`orca_user_profiles`/`orca_teams`/`orca_team_members` (0006, 0016, 0017) — **đây là service SINH RA `tenant_id`**, mọi service khác tham chiếu logic tới `tenant.companies.id` |
| 3 | `project-service` | `project` | `projects`, `project_members`, `source_projects` | `orca_v5_projects`/`orca_v5_project_members`/`orca_project_source_projects` (0007, 0016) |
| 4 | `infra-service` | `infra` | `ssh_targets`, `port_forwards` | `orca_ssh_targets` (0004, **active** qua `SqlStateRepository`) + cấu trúc dormant `orca_port_forwards` (0012) làm điểm khởi đầu cho `port_forwards` |
| 5 | `ai-provider-service` | `ai_provider` | `accounts`, `usage` | `orca_ai_provider_accounts`/`orca_provider_usage` (0008, 0015) |
| 6 | `workflow-service` | `workflow` | `templates`, `executions`, `step_executions` | `orca_workflow_templates`/`orca_workflow_executions`/`orca_workflow_step_executions` (0009, 0013, 0014) |
| 7 | `task-service` | `task` | `tasks`, `task_edges`, `task_grants`, `task_comments` | `orca_tasks`/`orca_task_edges`/`orca_task_grants`/`orca_task_comments` (0010, 0016) — **không** ôm `team_members` (thuộc `tenant-service`, gọi qua API để resolve grant scope thay vì JOIN cục bộ) |
| 8 | `orchestration-service` | `orchestration` | `messages`, `tasks`, `dispatch_contexts`, `decision_gates`, `coordinator_runs` | `orchestration.db` (SQLite riêng) — xem [04](./04-orchestration-db.md) |
| 9 | `automation-service` | `automation` | `automations`, `automation_runs` | `PersistedState.automations`/`automationRuns` (JSON, server mode) — **không** phải bảng dormant `automations` (0002), thiết kế lại theo shape `Automation`/`AutomationRun` thật ([06](./06-shared-domain-types.md) §6) |
| 10 | `annotation-service` | `annotation` | `annotations` | `orca_annotations` (0018) |
| 11 | `notification-service` | `notification` | `push_subscriptions`, `vapid_key_metadata` | `PersistedState.webPushSubscriptions`/`vapidKeys` (JSON) + bảng dormant `orca_push_subscriptions` (0012) — giá trị `privateKey` của VAPID **không** lưu plaintext trong bảng mới (khác hiện trạng JSON) — xem §4 |
| 12 | `usage-service` | `usage` | `claude_usage_sessions`, `claude_usage_daily`, `codex_usage_sessions`, `codex_usage_daily` | `orca-claude-usage.json`/`orca-codex-usage.json` ([07](./07-usage-tracking-stores.md)) |
| 13 | `credential-service` | `credential` | `credential_metadata` (chỉ status/rotation, KHÔNG giá trị secret) | Metadata layer mới — giá trị secret vẫn qua [05](./05-credential-secret-stores.md) |

**Chưa gán service — cần quyết định riêng ở Phase 1**: `orca_projects`/`orca_repos`/`orca_global_settings`
(0004, active qua `SqlStateRepository`) — đây là kho JSON-blob-trong-1-cột cho 4 entity gốc, chồng lấn khái
niệm với `project-service` (Project v5 "thật") nhưng shape khác hẳn (tuỳ ý, không quan hệ). `orca_global_settings`
đặc biệt: 1 row cấu hình toàn server-instance, **không tenant-scoped** — không nên gán `tenant_id`, đề xuất giữ
trong 1 schema `system` nhẹ (chưa tạo ở Phase 0, quyết định khi làm Phase 1) thay vì gán cho service nghiệp vụ
nào. `orca_projects`/`orca_repos` (JSON blob) tạm gán vào `project-service`/`infra-service` tương ứng ở Phase 1
sau khi xác nhận có consumer thật đang dùng chúng ở production (không chỉ interface `IStateRepository` — cần
kiểm tra request log thật trên b15.openledger.vn).

**Không di trú** (giữ nguyên hoặc dọn ở Phase 4, không map sang schema mới): bảng dormant migration 0001
(`settings`/`projects`/`repos`/`ssh_targets` — lưu ý: khác `orca_projects`/`orca_repos`/`orca_ssh_targets` của
0004 ở trên, xem [02](./02-sql-schema-catalog.md) nhóm A), 0002 (`automations` cũ), 0003 (`workspace_sessions`),
0011 (`orca_terminal_sessions`) — không có consumer, không có lý do di trú dữ liệu (rỗng hoặc gần rỗng).

## 3. DDL mẫu — Phase 0 (đã implement, xem migration 0019-0022)

```sql
-- Migration 0019: schema + tenant_id retrofit
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS tenant;
CREATE SCHEMA IF NOT EXISTS project;
CREATE SCHEMA IF NOT EXISTS infra;
CREATE SCHEMA IF NOT EXISTS ai_provider;
CREATE SCHEMA IF NOT EXISTS workflow;
CREATE SCHEMA IF NOT EXISTS task;
CREATE SCHEMA IF NOT EXISTS orchestration;
CREATE SCHEMA IF NOT EXISTS automation;
CREATE SCHEMA IF NOT EXISTS annotation;
CREATE SCHEMA IF NOT EXISTS notification;
CREATE SCHEMA IF NOT EXISTS usage;
CREATE SCHEMA IF NOT EXISTS credential;

-- Retrofit tenant_id lên bảng active hiện có (ví dụ orca_v5_projects — đầy đủ trong file migration thật)
ALTER TABLE orca_v5_projects ADD COLUMN tenant_id UUID;
CREATE INDEX idx_orca_v5_projects_tenant ON orca_v5_projects(tenant_id);
-- Backfill: xem ghi chú trong migration — orca_v5_projects hiện KHÔNG có cột company/tenant nào để suy ra,
-- nên tenant_id để NULL-able ở Phase 0 (chưa NOT NULL), gán giá trị thật + đổi NOT NULL ở Phase 1 sau khi
-- có logic backfill đúng (map qua orca_v5_project_members → orca_users → orca_departments → company).
```

DDL đầy đủ cho từng bảng nằm trong file migration thật (`backend/src/main/db/migrations/0019..0022*.ts`) —
tài liệu này chỉ tóm tắt để tránh trùng lặp/lệch bản.

## 4. Notification service — xử lý VAPID private key

Hiện trạng (`03-electron-desktop-store.md` §5): `vapidKeys.privateKey` lưu **plaintext** trong
`orca-data.json` — có chủ đích vì đây là key ký Web Push, không phải credential người dùng, nhưng khi hợp nhất
vào Postgres (dùng chung nhiều service, nhiều người vận hành có quyền đọc DB hơn 1 file JSON local), rủi ro lộ
tăng lên. Quyết định: bảng `notification.vapid_key_metadata` chỉ lưu `key_id`, `public_key`, `created_at`,
`status` — `private_key` **tiếp tục qua cơ chế credential riêng** (mục ADR-021 §4), không phải ngoại lệ.

## 5. Lộ trình chi tiết theo Phase

### Phase 0 — Migration scaffolding (✅ đã thực thi trong session này)

- Migration `0019`: 13 schema + retrofit `tenant_id` (nullable) lên toàn bộ bảng active thuộc nhóm C–N
  ([02-sql-schema-catalog.md](./02-sql-schema-catalog.md)).
- Migration `0020`: DDL đầy đủ schema `orchestration` (mirror 5 bảng từ `OrchestrationDb`).
- Migration `0021`: DDL đầy đủ schema `automation` (`automations`, `automation_runs`).
- Migration `0022`: DDL schema `notification` (`push_subscriptions`, `vapid_key_metadata`) + schema `usage`
  (4 bảng usage tracking).
- Không đổi bất kỳ service code nào — các bảng mới **chưa có consumer**, an toàn deploy lên
  b15.openledger.vn mà không ảnh hưởng runtime.

### Phase 1 — Application-layer tenant isolation + wire code hiện có sang Postgres

**Đã làm (session này):**
- `RpcContext.tenantId` (`runtime/rpc/core.ts`) + `TenantResolver`/`ProfileService.getCompanyIdForUser()`
  (`tenancy/tenant-resolver.ts`) — resolve tenantId 1 lần/user-process tại bootstrap, forward vào mọi RPC
  dispatch trên Unix-socket path, đúng khuôn mẫu `userId`/`ORCA_USER_ID` đã có sẵn (single-user-per-process
  trong `ORCA_MULTI_USER=1`). `TenantContext` (AsyncLocalStorage, `tenancy/tenant-context.ts`) làm primitive dự
  phòng cho code path không có `RpcContext` (cron/scheduler).
- **Interface-seam extraction** (an toàn, additive, KHÔNG đổi hành vi — `Store` tự thoả các interface này qua
  structural typing, không sửa 1 dòng nào trong `persistence.ts`) cho 3 service, theo pattern
  `Pick<Store, ...>` đã có tiền lệ trong codebase (`usage-worktree-metadata.ts`):
  - `AutomationService` + `resolveAutomationRunTarget()` → `AutomationStoreDependency`
    (`automations/automation-store-dependency.ts`, 8 method: `listAutomations`, `listAutomationRuns`,
    `createAutomationRun`, `updateAutomationRun`, `advanceAutomationNextRun`,
    `getLatestAutomationOccurrence`, `getRepo`, `getProjectHostSetups`).
  - `ClaudeUsageStore`/`CodexUsageStore` → `UsageStoreRepoSource` (`usage-worktree-metadata.ts`, 2 method:
    `getRepos`, `getAllWorktreeMeta`).
  - `WebPushManager` → `WebPushStoreDependency` (`notifications/web-push-manager.ts`, 4 method:
    `getWebPushSubscriptions`, `setWebPushSubscriptions`, `getVapidKeys`, `setVapidKeys` — ⚠️ private VAPID
    key đi qua interface này, PgWebPush impl sau này KHÔNG được lưu nó vào `notification.vapid_key_metadata`,
    xem ADR-021 §4).
  - Typecheck xác nhận: 0 lỗi mới phát sinh (158 lỗi pre-existing trong branch, không liên quan, giữ nguyên).

**Chưa làm (còn lại của Phase 1):**
- Viết implementation Postgres thật cho 3 interface trên (`PgAutomationStore`, `PgWebPushStore`, `PgUsageStore`)
  — ghi vào bảng migration 0021/0022. Nhánh RRULE (`nextAutomationOccurrenceAfter`/
  `latestAutomationOccurrenceAtOrBefore`) đã là pure function tách khỏi `Store` sẵn — implementation mới có
  thể tái dùng thẳng, không cần viết lại logic lịch.
- Wire lựa chọn implementation (JSON `Store` vs Postgres) vào `server-bootstrap.ts` — đề xuất qua config flag
  tường minh, mặc định giữ hành vi hiện tại (JSON), KHÔNG tự động đổi default khi chưa test kỹ trên staging.
- Refactor `AuthManager`/`ProfileService`/`ProjectService`/`AIProviderService`/`WorkflowOrchestrator`/
  `TaskService`/`annotation-store.ts` — mỗi service tự thêm `WHERE tenant_id = ?` vào query, dùng
  `requireTenantId()`/`ctx.tenantId`. **Chưa bắt đầu** — đây là phần chạm business/security-critical logic,
  cần review riêng, KHÔNG nên gộp vội vào cùng phiên với phần scaffolding.
- Backfill `tenant_id` cho dữ liệu hiện có trên b15.openledger.vn (script riêng, dry-run trước), rồi khoá
  `NOT NULL`.
- ⚠️ Phát hiện phụ (không liên quan ADR-021): `ProfileService.ts` và 10 file khác (`TaskService`,
  `WorkflowOrchestrator`, `AIProviderService`, `TeamService`, `annotation-store.ts`...) đang có lỗi typecheck
  có sẵn (`db.query<T>()` dùng generic nhưng `IDatabase.query()` trong `db/types.ts` không khai báo generic)
  — có vẻ là 1 refactor dở dang khác trên cùng branch. Cần xử lý trước khi làm phần "refactor mỗi service tự
  filter tenant_id" ở trên, vì nó chặn build sạch của chính các service đó.

### Phase 2 — Migrate dữ liệu OrchestrationDb (SQLite) → Postgres — ✅ Đã cutover

- `PgOrchestrationDb` (`runtime/orchestration/pg-db.ts`) — port đủ 40 method, bọc `pool.withTransaction()`
  đúng những chuỗi thao tác vốn atomic nhờ SQLite đơn luồng (`updateTaskStatus`→`promoteReadyTasks`,
  `createDispatchContext`, `createGate`, `resolveGate`, `failDispatch`, ...).
- 11 file phụ thuộc (`coordinator.ts`, `lifecycle-reconciliation.ts`, `orca-runtime.ts`,
  `orca-runtime-pty-exit.ts`, `orca-runtime-worktree-lineage.ts`, `TaskOrchestrationBridge.ts`, 2 file RPC
  methods, test) đã chuyển xong — `KeyedAsyncQueue` (`runtime/orchestration/keyed-async-queue.ts`) giải
  quyết đúng rủi ro race condition đã cảnh báo (event handler đồng bộ → gọi DB bất đồng bộ fire-and-forget,
  tuần tự hoá theo `handle`).
- ⚠️ 1 suy giảm có chủ đích: `getAgentStatusOrchestrationContextForHandle`
  (`orca-runtime-terminal-agent-status.ts`) trả về `undefined` cố định trong server mode — hàm đồng bộ,
  chuỗi gọi ngược (`syncWindowGraph()`) chưa được review để cascade async an toàn. Chỉ mất hiển thị UI
  "task nào đang chạy cạnh terminal", không ảnh hưởng dispatch/coordinator thật.
- Không cần dual-write/rollback flag — `runtime/orchestration/db.ts` (SQLite) không còn ai import trong
  `backend/src/main` nữa (đã verify bằng grep), migration 0020 đã sửa khớp đúng schema thật.

### Phase "Store cutover" (ngoài kế hoạch gốc, mở rộng theo yêu cầu) — ✅ Đã cutover

`persistence.ts`'s `Store` (~3900 dòng, backing `orca-data.json` — Project/Repo/Worktree/Tab/UI
state/Automation/WebPush/Usage) là phần **lớn nhất** hệ thống (CRITICAL, 300 symbol/98 caller trực tiếp).
Không đụng vào 727 dòng logic migrate-on-load hay ~100 method mutator — chỉ tách "biên" load/save:

- Migration 0024 (`core.orca_data_state_blob`) — 1 blob JSON/tenant/user, cùng pattern migration 0023.
- `PgOrcaDataStatePersistence` (`orca-data-state-persistence.ts`) — `loadRawState()`/`save()`.
- `Store.hydrateFromPostgres()` — method mới, gọi **sau khi construct** (không phải lúc constructor, vì
  `DevServerManager` cần `store` sống trước khi `pool` tồn tại) — thay `this.state` bằng Postgres nếu đã
  có row, hoặc seed Postgres từ state hiện tại (đã qua load() file-based) nếu chưa có; từ đó
  `writeToDiskAsync()` ghi qua `persistOverride` thay vì file. Gọi trước `rpcServer.start()` nên không
  request thật nào thấy state trước khi hydrate xong.
- ⚠️ Đơn giản hoá có chủ đích: load từ Postgres chỉ merge-default nông (`{...defaults, ...postgresState}`),
  không chạy lại từng migration cụ thể (`migrateTerminalScrollbackRows` v.v.) như đường file — chấp nhận
  được vì Postgres row luôn do chính `buildStateToSave()` ghi ra (luôn ở 1 dạng đã-migrate nào đó), chỉ có
  thể lệch nếu field mới thêm giữa 2 lần chạy — ghi rõ trong doc comment để revisit nếu phát sinh field cần
  migration phức tạp hơn default-fill.

### Phase 3 — Tách service thật (process/container riêng)

- Pilot trước với `usage-service` hoặc `notification-service` (rủi ro thấp nhất, ít phụ thuộc realtime).
- Mỗi service: Dockerfile riêng (theo mẫu `deploy/dev/docker/backend/Dockerfile.artifact` đã có), thêm vào
  `docker-compose.orca.artifact.yml` (đã có `orca-net` bridge network, chỉ cần thêm service mới cùng mạng).
  Postgres role riêng theo schema (mục 1.3).
- `orca` gateway (RPC namespace `rpc/methods/*.ts`) đổi handler tương ứng thành HTTP proxy gọi service mới —
  làm từng namespace 1, không đổi tất cả cùng lúc.
- Sau khi pilot ổn định, mở rộng dần sang service còn lại — thứ tự đề xuất theo mức độ phụ thuộc tăng dần:
  `annotation` → `ai_provider` → `workflow` → `automation` → `infra` → `project` → `task` → `orchestration` →
  `tenant` → `auth` (2 service cuối rủi ro cao nhất — mọi service khác phụ thuộc chúng để resolve tenant/user,
  tách sau cùng khi pattern đã được validate).

### Phase 4 — Dọn dẹp

- `DROP` bảng dormant (0001/0002/0003, `orca_terminal_sessions`, `orca_port_forwards`/`orca_push_subscriptions`
  cũ) sau khi xác nhận trên production không còn dữ liệu (kiểm `SELECT count(*)` trước khi drop, có backup).
- Xoá code path JSON (`Store.automations`/`webPushSubscriptions`, usage JSON files) khỏi nhánh server-mode —
  Electron desktop-mode code path (dùng chính các type/field này) **không đổi**.
- Xoá `orchestration.db`/`better-sqlite3` dependency khỏi server-mode build nếu không còn nơi nào dùng.

## 6. Rủi ro & điều kiện dừng

- **Không tự động chạy Phase 1-4 mà không có kế hoạch rollback cụ thể cho từng bước** — b15.openledger.vn là
  production thật, có dữ liệu người dùng thật.
- Trước mỗi Phase từ 1 trở đi: chạy `detect_changes({scope:'compare', base_ref:'main'})` +
  `impact()` trên các symbol bị đổi, warn rõ nếu risk HIGH/CRITICAL (theo yêu cầu AGENTS.md/CLAUDE.md).
- Phase 3 (tách service thật) là thay đổi hạ tầng lớn nhất — nên demo trên môi trường staging/dev trước,
  không áp thẳng lên b15.openledger.vn.
