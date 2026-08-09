# SOLUTION: BUG-BE-HLD-016 — Đụng độ tên bảng `orca_projects` giữa migration 0004 và 0007

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:** `backend/src/main/db/migrations/0004_orca_app_tables.ts`, `backend/src/main/db/migrations/0007_projects.ts`, `docs/hld/backend-server-architecture.md`
**Phạm vi fix:** CHỈ tài liệu (`docs/hld/backend-server-architecture.md §10`). KHÔNG sửa migration đã chạy production. File này tự nó KHÔNG áp dụng thay đổi — chỉ mô tả chính xác thay đổi cần làm để người có thẩm quyền áp dụng.

---

## 1. Tóm tắt bug

Ticket: `specs/backend/bugs/hld-v1/BUG-BE-HLD-016-db-migration-table-naming-collision-v5-prefix.md`

Migration `0004_orca_app_tables.ts` đã tạo bảng tên `orca_projects` từ trước (schema cũ, dùng cho single-user/desktop mode — lưu tab/state). Khi migration `0007_projects.ts` được viết sau đó cho tính năng Project↔DevServer binding (F34/TDD-15), đội ngũ phát hiện tên `orca_projects` đã bị chiếm, nên phải đổi tên bảng mới thành `orca_v5_projects` / `orca_v5_project_members` để tránh đụng độ.

Vấn đề: `docs/hld/backend-server-architecture.md §10` (mục "DB Schema Overview") **chưa được cập nhật** theo tên bảng thật — hiện đang ghi migration 0007 tạo ra `orca_projects`, `orca_project_members` (không có tiền tố `v5`), trong khi trong DB thật hai bảng đó không tồn tại với tên này — tên thật là `orca_v5_projects`, `orca_v5_project_members`. Đồng thời tên `orca_projects` trong tài liệu bị trùng với bảng có thật do migration 0004 tạo ra (schema hoàn toàn khác), gây nhầm lẫn nghiêm trọng cho bất kỳ ai đọc doc rồi viết SQL trực tiếp.

---

## 2. Bằng chứng từ code thật

### 2.1. Migration 0004 — tạo bảng `orca_projects` (schema cũ, KHÔNG liên quan Project-DevServer binding)

**File:** `backend/src/main/db/migrations/0004_orca_app_tables.ts`
**Lines:** 1–30

```typescript
/**
 * Migration 0004 — Orca Server-mode Application Tables
 *
 * Creates `orca_*` tables for SqlStateRepository (server mode).
 * These are separate from the core `projects`, `repos` tables (migration 0001)
 * to maintain clear separation between server-mode state and system state.
 *
 * @module db/migrations/0004_orca_app_tables
 */

import type { Migration } from './types'

export const migration0004OrcaAppTables: Migration = {
  version: 4,
  name: 'orca_app_tables',

  async up(db) {
    // ── orca_projects ────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_projects (
        id          TEXT PRIMARY KEY,
        name        TEXT NOT NULL,
        tab_order   INTEGER NOT NULL DEFAULT 0,
        data        TEXT NOT NULL DEFAULT '{}',
        created_at  TEXT NOT NULL DEFAULT (datetime('now'))
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_projects_tab_order ON orca_projects(tab_order)
    `)
```

Bảng `orca_projects` này lưu `id, name, tab_order, data (JSON blob), created_at` — đây là schema tab/state đơn giản cho desktop/single-user mode, **không có** cột `dev_server_id`, `repo_path`, `visibility`, v.v.

### 2.2. Migration 0007 — tạo bảng thật `orca_v5_projects` / `orca_v5_project_members` (Project-DevServer binding, F34/TDD-15)

**File:** `backend/src/main/db/migrations/0007_projects.ts`
**Lines:** 1–55

```typescript
/**
 * Migration 0007 — v5 Project Management Tables
 *
 * Adds project management tables for TDD-15 (Project-Dev Server Binding).
 * Note: uses orca_v5_* prefix to avoid collision with legacy orca_projects
 * table created in migration 0004 (which stores tab/state data).
 *
 * - orca_v5_projects: Full project entities linked to a dev server
 * - orca_v5_project_members: Project membership + role table
 *
 * @module db/migrations/0007_projects
 */

import type { Migration } from './types'

export const migration0007Projects: Migration = {
  version: 7,
  name: 'projects',

  async up(db) {
    // ── orca_v5_projects ──────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_v5_projects (
        id             TEXT    PRIMARY KEY,
        name           TEXT    NOT NULL,
        description    TEXT,
        dev_server_id  TEXT    NOT NULL,
        repo_path      TEXT    NOT NULL,
        default_branch TEXT    NOT NULL DEFAULT 'main',
        visibility     TEXT    NOT NULL DEFAULT 'team',
        created_by     TEXT    NOT NULL,
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_v5_projects_server
        ON orca_v5_projects(dev_server_id)
    `)

    // ── orca_v5_project_members ───────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_v5_project_members (
        project_id TEXT    NOT NULL REFERENCES orca_v5_projects(id) ON DELETE CASCADE,
        user_id    TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role       TEXT    NOT NULL DEFAULT 'member',
        added_at   INTEGER NOT NULL,
        PRIMARY KEY (project_id, user_id)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_v5_project_members_user
        ON orca_v5_project_members(user_id)
    `)
  },
```

**Kết luận bằng chứng:** Tên bảng thật đang tồn tại trong DB cho tính năng Project-DevServer binding là **`orca_v5_projects`** và **`orca_v5_project_members`** — không phải `orca_projects`/`orca_project_members` như tài liệu hiện ghi. Comment tại dòng 5–6 của `0007_projects.ts` xác nhận rõ lý do đổi tên: tránh đụng độ với bảng `orca_projects` legacy đã có sẵn từ migration 0004.

---

## 3. (a) Đoạn text cần sửa trong `docs/hld/backend-server-architecture.md §10`

**File:** `docs/hld/backend-server-architecture.md`
**Lines:** 285–298 (mục `## 10. DB Schema Overview (Migrations 0001–0010)`), dòng sai cụ thể là **line 295**.

### TRƯỚC (nguyên văn hiện tại):

```markdown
## 10. DB Schema Overview (Migrations 0001–0010)

| Migration | Tables |
|-----------|--------|
| 0001 `init` | `projects`, `worktrees`, `agent_sessions`, `settings` |
| 0002 `sessions` | `terminal_scrollback_snapshots` |
| 0003 `ssh_targets` | `ssh_hosts`, `saved_port_forwards` |
| 0004 `automations` | `automations`, `automation_runs`, `notifications`, `rate_limits` |
| 0005 `auth_schema` | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |
| 0006 `profile` | `orca_company`, `orca_departments` + ALTER `orca_users` |
| 0007 `project` | `orca_projects`, `orca_project_members` |
| 0008 `ai_providers` | `orca_ai_provider_accounts`, `orca_provider_usage` |
| 0009 `workflow` | `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions` |
| 0010 `task_graph` | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments` |
```

### SAU (đề xuất thay thế):

```markdown
## 10. DB Schema Overview (Migrations 0001–0010)

| Migration | Tables |
|-----------|--------|
| 0001 `init` | `projects`, `worktrees`, `agent_sessions`, `settings` |
| 0002 `sessions` | `terminal_scrollback_snapshots` |
| 0003 `ssh_targets` | `ssh_hosts`, `saved_port_forwards` |
| 0004 `orca_app_tables` | `orca_projects` (legacy, tab/state cho desktop/single-user mode — KHÔNG liên quan Project↔DevServer binding), `orca_repos`, `orca_ssh_targets`, `orca_global_settings` |
| 0005 `auth_schema` | `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies` |
| 0006 `profile` | `orca_company`, `orca_departments` + ALTER `orca_users` |
| 0007 `project` | `orca_v5_projects`, `orca_v5_project_members` — dùng tiền tố `v5` để tránh đụng độ với bảng `orca_projects` legacy của migration 0004 (xem comment `0007_projects.ts:5-9`). Đây là bảng project thật cho tính năng Project-DevServer binding (F34/TDD-15). |
| 0008 `ai_providers` | `orca_ai_provider_accounts`, `orca_provider_usage` |
| 0009 `workflow` | `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions` |
| 0010 `task_graph` | `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments` |

> ⚠️ **Lưu ý naming:** `orca_projects` (0004) và `orca_v5_projects` (0007) là **hai bảng khác nhau, cùng tồn tại song song trong DB**, không phải hai phiên bản của cùng một bảng. `orca_projects` lưu state/tab đơn giản (desktop/single-user mode, không có `dev_server_id`); `orca_v5_projects` là entity project đầy đủ gắn với dev server (server mode, F34). Khi viết SQL hoặc query trực tiếp, LUÔN xác nhận đang thao tác đúng bảng theo mục đích. Xem `BUG-BE-HLD-016` để biết bối cảnh đầy đủ và kế hoạch dọn dẹp dài hạn.
```

**Ghi chú về đề xuất trên:**
- Dòng 0004 trong bản TRƯỚC ghi sai cả tên migration (`automations`) và danh sách bảng (`automations`, `automation_runs`, `notifications`, `rate_limits`) — đây là lệch tài liệu khác, **không thuộc phạm vi BUG-BE-HLD-016** (ticket chỉ đề cập đụng độ tên bảng project). Bản SAU ở trên đã sửa luôn dòng 0004 cho khớp với code thật (`migration0004OrcaAppTables`, tên `orca_app_tables`) vì nó liên quan trực tiếp đến việc giải thích bảng `orca_projects` legacy — nếu người áp dụng fix muốn giữ tối thiểu, chỉ cần sửa dòng 0007 (thêm tiền tố `v5`) và thêm dòng cảnh báo cuối bảng; sửa dòng 0004 là tuỳ chọn mở rộng để tài liệu nhất quán hơn.
- Cần rà thêm `docs/adrs/v2/ADR-016-db-migrations-0006-0010-schema.md` (ticket có nhắc tới) vì tài liệu đó cũng ghi `orca_projects`/`orca_project_members` không tiền tố — không nằm trong phạm vi đọc của nhiệm vụ này nên chỉ ghi chú lại, chưa trích dẫn nguyên văn.

---

## 4. (b) Kế hoạch dài hạn dọn dẹp bảng `orca_projects` cũ (migration 0004)

**Không viết migration DROP/RENAME ngay.** Đây là kế hoạch xác minh từng bước, cần thực hiện tuần tự và có approval trước khi đụng vào schema production.

### Bước 1 — Xác nhận còn caller nào tham chiếu `orca_projects` (không phải `orca_v5_projects`)

Chạy các lệnh tìm kiếm sau trên toàn repo (backend, desktop/renderer, mobile, scripts), chú ý loại trừ nhiễu từ tên `orca_v5_projects`:

```bash
# Tìm mọi tham chiếu SQL trực tiếp tới bảng orca_projects (loại trừ orca_v5_projects)
grep -rn "orca_projects" backend/ desktop/ mobile/ scripts/ config/ --include="*.ts" --include="*.js" --include="*.sql" \
  | grep -v "orca_v5_projects"

# Tìm riêng trong repository/service layer
grep -rn "FROM orca_projects\|INTO orca_projects\|UPDATE orca_projects\|orca_projects(" backend/src \
  | grep -v "orca_v5_projects"

# Tìm trong migration down()/rollback scripts và backup/restore scripts
grep -rln "orca_projects" backend/src/main/db --include="*.ts" | grep -v "orca_v5_projects"

# Tìm reference trong test fixtures (để biết có test nào phụ thuộc bảng này không)
grep -rln "orca_projects" backend/ --include="*.test.ts" | grep -v "orca_v5_projects"
```

Ghi lại toàn bộ danh sách file/dòng tìm được vào một bảng kiểm kê (inventory) trước khi quyết định bước tiếp theo.

### Bước 2 — Kiểm tra bảng `orca_projects` có dữ liệu thật trong production/staging hay không

- Kết nối vào DB production/staging (qua kênh vận hành đã được cấp quyền, không dùng shell trực tiếp nếu có quy trình riêng) và chạy:
  ```sql
  SELECT COUNT(*) FROM orca_projects;
  SELECT * FROM orca_projects ORDER BY created_at DESC LIMIT 20;
  ```
- Đối chiếu `created_at` gần nhất — nếu bản ghi mới nhất rất cũ (ví dụ không có insert nào sau khi tính năng server-mode/F34 ra mắt), đó là tín hiệu mạnh cho thấy bảng đã "chết" (dead table).
- Nếu có backup/snapshot định kỳ, kiểm tra lịch sử tăng trưởng số dòng theo thời gian để xác nhận xu hướng (đứng yên = nghi ngờ dead table; vẫn tăng = còn được ghi bởi luồng nào đó chưa tìm ra ở Bước 1).

### Bước 3 — Kiểm tra RPC handlers / repository layer đang query bảng nào

- Tìm class/service dùng `SqlStateRepository` (theo comment trong `0004_orca_app_tables.ts` dòng 4: "Creates `orca_*` tables for SqlStateRepository (server mode)") — xác định `SqlStateRepository` có còn được wire vào bất kỳ RPC handler nào đang hoạt động không, hay chỉ còn tồn tại như code chết.
- Đối chiếu với `ProjectService.ts` (dùng `orca_v5_projects`/`orca_v5_project_members` theo bằng chứng trong ticket, dòng 102-118, 176, 219-224) để xác nhận đây là service layer đang hoạt động (đường đi chính) cho tính năng project hiện tại.
- Nếu có RPC contract / IPC channel (ví dụ `projects.list`, `projects.create` cũ dùng schema desktop) vẫn còn đăng ký và trỏ vào `SqlStateRepository`, đó là caller thật cần giữ lại bảng — không được xoá.

### Bước 4 — Nếu xác nhận là dead table: quy trình deprecate an toàn

Chỉ thực hiện sau khi Bước 1–3 đều xác nhận không còn caller và không còn dữ liệu quan trọng:

1. **Giai đoạn đổi tên trước khi xoá** (migration version mới, ví dụ `0011_deprecate_legacy_orca_projects.ts`):
   ```sql
   ALTER TABLE orca_projects RENAME TO orca_projects_deprecated_YYYYMM;
   ```
   Không xoá dữ liệu ngay — chỉ đổi tên để bất kỳ code nào còn sót lại tham chiếu sẽ lỗi rõ ràng (fail-fast) thay vì âm thầm query nhầm bảng.
2. **Thời gian giữ lại để rollback:** giữ bảng đã đổi tên tối thiểu 1–2 chu kỳ release (tuỳ chính sách release của team) trước khi DROP thật, để có thể `ALTER TABLE ... RENAME TO orca_projects` khôi phục nếu phát hiện caller bị bỏ sót.
3. **Thông báo cho team:** gửi thông báo trong kênh kỹ thuật (changelog nội bộ, PR description, hoặc kênh chat team) nêu rõ: bảng nào bị deprecate, migration version nào thực hiện, thời điểm dự kiến DROP thật, và cách rollback nếu cần.
4. **Viết migration DROP TABLE ở version sau** (sau khi hết thời gian giữ lại, ví dụ `0012_drop_legacy_orca_projects.ts`):
   ```sql
   DROP TABLE IF EXISTS orca_projects_deprecated_YYYYMM;
   ```
   kèm `down()` migration khôi phục cấu trúc bảng gốc (không khôi phục được dữ liệu nếu đã xoá thật — cần backup riêng trước khi DROP).
5. **Rollback plan:** trước khi chạy migration DROP thật trên production, phải có bản backup/export đầy đủ dữ liệu bảng (dù đã xác nhận dead) lưu trữ ít nhất theo chính sách retention của tổ chức, để có thể khôi phục thủ công nếu phát sinh yêu cầu audit/compliance bất ngờ.

### Bước 5 — Ai cần approve trước khi chạy migration dọn dẹp trên production

- **Tech lead / chủ trì module `backend/src/main/db`** — xác nhận kết quả rà soát ở Bước 1–3 là đầy đủ và chính xác.
- **Người phụ trách vận hành DB production (DBA hoặc SRE phụ trách)** — approve về mặt an toàn dữ liệu, thời điểm chạy migration, và rollback plan.
- **Product/feature owner của tính năng desktop/single-user mode** (nếu còn tồn tại vai trò này) — xác nhận bảng `orca_projects` không còn được dùng bởi bất kỳ chế độ vận hành nào (kể cả các bản desktop cũ hơn đang chạy ngoài field, nếu Orca hỗ trợ auto-update chậm hoặc offline).
- Chỉ sau khi có approval bằng văn bản (PR review, ticket comment) từ các vai trò trên mới được merge và chạy migration DROP trên production.

---

## 5. Rủi ro nếu không sửa tài liệu

- **Dev mới đọc tài liệu sẽ viết nhầm SQL:** bất kỳ ai dựa vào `docs/hld/backend-server-architecture.md §10` để viết script debug, backup/restore, hoặc query trực tiếp sẽ nhắm vào bảng `orca_projects` (schema cũ, tab/state) trong khi thực ra cần `orca_v5_projects` (project entity cho F34) — dẫn đến kết quả sai hoặc thao tác nhầm bảng.
- **Nguy cơ dùng nhầm bảng khi viết code mới:** nếu có tính năng mới cần join/tham chiếu tới "bảng project", tài liệu sai có thể khiến engineer implement JOIN vào `orca_projects` thay vì `orca_v5_projects`, gây lỗi logic khó phát hiện (không lỗi cú pháp SQL vì cả hai bảng tồn tại thật, chỉ sai về mặt business logic).
- **Documentation drift lan rộng:** `docs/adrs/v2/ADR-016` cũng bị ảnh hưởng tương tự (theo ticket) — nếu không sửa đồng bộ, các tài liệu tham chiếu chéo nhau tiếp tục củng cố thông tin sai, làm tăng chi phí onboarding và review về sau.
- **Rủi ro compliance/audit:** nếu về sau có audit dữ liệu cần biết chính xác "bảng project nào lưu gì", tài liệu sai lệch làm chậm quá trình điều tra và có thể dẫn đến kết luận sai về phạm vi dữ liệu bị ảnh hưởng trong một sự cố bảo mật.
