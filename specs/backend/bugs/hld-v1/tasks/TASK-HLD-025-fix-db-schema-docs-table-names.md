# TASK-HLD-025: Sửa docs backend-server-architecture.md §10 — tên bảng thật orca_v5_projects/orca_v5_project_members

**Priority:** 🟡 MEDIUM — rủi ro dev viết nhầm SQL production, không phải lỗi runtime
**Effort:** ~30 phút (chỉ sửa tài liệu)
**Status:** ✅ DONE — 2026-08-09 (sửa `docs/hld/backend-server-architecture.md` §10 đúng bản "SAU" đề xuất trong task: dòng 0004 đổi tên migration + danh sách bảng thật (xác nhận trực tiếp qua `grep "CREATE TABLE" 0004_orca_app_tables.ts` → `orca_projects, orca_repos, orca_ssh_targets, orca_global_settings`, khớp 100%); dòng 0007 đổi `orca_projects/orca_project_members` → `orca_v5_projects/orca_v5_project_members` + ghi chú tiền tố v5; thêm khối cảnh báo naming cuối bảng. Không đụng vào bất kỳ file `.ts` migration nào. Chưa sửa `docs/adrs/v2/ADR-016-db-migrations-0006-0010-schema.md` như task ghi chú (out-of-scope, cần task riêng nếu tech lead xác nhận) và chưa thực hiện kế hoạch xác minh dead-table 5 bước (đúng ý task — đây là kế hoạch dài hạn, không code ngay).)
**Bug refs:** BUG-BE-HLD-016
**Solution ref:** [SOLUTION-db-migration-naming-exact.md](../solutions/SOLUTION-db-migration-naming-exact.md)
**Depends on:** Không

---

## ⚠️ QUAN TRỌNG: đây KHÔNG phải fix code

Task này **chỉ sửa tài liệu** (`docs/hld/backend-server-architecture.md §10`). **KHÔNG** đụng vào bất kỳ migration `.ts` nào đã chạy production (`0004_orca_app_tables.ts`, `0007_projects.ts` giữ nguyên 100%). Không có code, không có test, không có migration mới trong phạm vi task này.

## Mục tiêu

`docs/hld/backend-server-architecture.md §10` (mục "DB Schema Overview") hiện ghi sai: migration 0007 tạo ra `orca_projects`, `orca_project_members` (không tiền tố `v5`). Thực tế trong DB, hai bảng đó **không tồn tại với tên này** — tên thật là `orca_v5_projects`, `orca_v5_project_members` (đổi tên có chủ đích để tránh đụng độ với bảng `orca_projects` legacy đã có sẵn từ migration `0004_orca_app_tables.ts`, dùng cho tab/state desktop/single-user mode — schema hoàn toàn khác, không có cột `dev_server_id`).

Tài liệu sai gây rủi ro: dev mới đọc doc rồi viết SQL/script debug trực tiếp sẽ nhắm nhầm vào bảng `orca_projects` (legacy) thay vì `orca_v5_projects` (project entity thật cho F34/TDD-15 Project-DevServer binding).

## File cần sửa/tạo

```
docs/hld/backend-server-architecture.md   (sửa §10, lines 285-298, dòng sai cụ thể: line 295)
```

## Thay đổi cụ thể

### TRƯỚC (nguyên văn hiện tại)

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

### SAU (đề xuất thay thế)

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

### Ghi chú về phạm vi thay đổi tối thiểu vs mở rộng

- Dòng 0004 trong bản TRƯỚC ghi sai cả tên migration (`automations`) lẫn danh sách bảng — đây là lệch tài liệu **khác**, không thuộc phạm vi BUG-BE-HLD-016 (ticket chỉ đề cập đụng độ tên bảng project). Bản SAU ở trên đã sửa luôn dòng 0004 cho khớp code thật (`migration0004OrcaAppTables`, tên `orca_app_tables`) vì nó liên quan trực tiếp đến việc giải thích bảng `orca_projects` legacy.
- **Nếu muốn giữ tối thiểu:** chỉ cần sửa dòng 0007 (thêm tiền tố `v5`) và thêm dòng cảnh báo cuối bảng — sửa dòng 0004 là tuỳ chọn mở rộng để tài liệu nhất quán hơn, không bắt buộc để đóng BUG-BE-HLD-016.
- **Cần rà thêm** `docs/adrs/v2/ADR-016-db-migrations-0006-0010-schema.md` (ticket gốc có nhắc tới) vì tài liệu đó cũng ghi `orca_projects`/`orca_project_members` không tiền tố — không nằm trong phạm vi đọc của solution này, chỉ ghi chú lại, chưa xác nhận nội dung chính xác cần sửa ở đó. Nếu tech lead xác nhận, tạo task riêng cho ADR-016.

## Kế hoạch dài hạn (KHÔNG code ngay) — xác nhận bảng `orca_projects` cũ có phải dead table hay không

Đây là kế hoạch xác minh từng bước, **không viết migration DROP/RENAME trong task này**, cần thực hiện tuần tự và có approval trước khi đụng vào schema production.

### Bước 1 — Xác nhận còn caller nào tham chiếu `orca_projects` (không phải `orca_v5_projects`)

```bash
grep -rn "orca_projects" backend/ desktop/ mobile/ scripts/ config/ --include="*.ts" --include="*.js" --include="*.sql" \
  | grep -v "orca_v5_projects"

grep -rn "FROM orca_projects\|INTO orca_projects\|UPDATE orca_projects\|orca_projects(" backend/src \
  | grep -v "orca_v5_projects"

grep -rln "orca_projects" backend/src/main/db --include="*.ts" | grep -v "orca_v5_projects"

grep -rln "orca_projects" backend/ --include="*.test.ts" | grep -v "orca_v5_projects"
```

Ghi lại toàn bộ danh sách file/dòng tìm được vào một bảng kiểm kê (inventory) trước khi quyết định bước tiếp theo.

### Bước 2 — Kiểm tra bảng `orca_projects` có dữ liệu thật trong production/staging hay không

- Kết nối vào DB production/staging (qua kênh vận hành đã được cấp quyền) và chạy `SELECT COUNT(*) FROM orca_projects;` và `SELECT * FROM orca_projects ORDER BY created_at DESC LIMIT 20;`.
- Đối chiếu `created_at` gần nhất — nếu bản ghi mới nhất rất cũ (không có insert sau khi server-mode/F34 ra mắt), đó là tín hiệu mạnh cho thấy bảng đã "chết".
- Kiểm tra lịch sử tăng trưởng số dòng theo thời gian nếu có backup/snapshot định kỳ.

### Bước 3 — Kiểm tra RPC handlers / repository layer đang query bảng nào

- Xác định `SqlStateRepository` (comment `0004_orca_app_tables.ts:4`: "Creates `orca_*` tables for SqlStateRepository (server mode)") có còn wire vào bất kỳ RPC handler nào đang hoạt động hay chỉ còn là code chết.
- Đối chiếu với `ProjectService.ts` (dùng `orca_v5_projects`/`orca_v5_project_members`) để xác nhận đây là service layer đang hoạt động thật.

### Bước 4 — Nếu xác nhận là dead table: quy trình deprecate an toàn

Chỉ thực hiện sau khi Bước 1–3 đều xác nhận không còn caller và không còn dữ liệu quan trọng:

1. Giai đoạn đổi tên trước khi xoá (migration mới, ví dụ `0011_deprecate_legacy_orca_projects.ts`): `ALTER TABLE orca_projects RENAME TO orca_projects_deprecated_YYYYMM;` — không xoá dữ liệu ngay, chỉ đổi tên để code còn sót lại lỗi rõ ràng (fail-fast).
2. Giữ bảng đã đổi tên tối thiểu 1-2 chu kỳ release trước khi DROP thật.
3. Thông báo team qua changelog nội bộ/PR description/kênh chat: bảng nào bị deprecate, version nào thực hiện, thời điểm dự kiến DROP, cách rollback.
4. Viết migration DROP TABLE ở version sau (ví dụ `0012_drop_legacy_orca_projects.ts`) kèm `down()` khôi phục cấu trúc (không khôi phục dữ liệu).
5. Backup/export đầy đủ dữ liệu bảng trước khi DROP thật, lưu theo chính sách retention của tổ chức.

### Bước 5 — Ai cần approve trước khi chạy migration dọn dẹp trên production

- Tech lead / chủ trì module `backend/src/main/db`.
- Người phụ trách vận hành DB production (DBA/SRE).
- Product/feature owner của tính năng desktop/single-user mode (nếu còn vai trò này) — xác nhận không còn được dùng bởi bất kỳ chế độ vận hành nào, kể cả bản desktop cũ ngoài field.
- Chỉ merge/chạy migration DROP sau khi có approval bằng văn bản (PR review, ticket comment) từ các vai trò trên.

## Verification

```bash
cd /opt/repos/orca

# Xác nhận doc đã sửa đúng
grep -n "orca_v5_projects\|orca_v5_project_members" docs/hld/backend-server-architecture.md

# Xác nhận không còn dòng doc ghi sai orca_projects cho migration 0007
grep -n "0007.*orca_projects\b" docs/hld/backend-server-architecture.md
# Expected: 0 kết quả (chỉ còn orca_v5_projects ở dòng 0007)
```

Không có test tự động — verification là đọc lại đoạn markdown đã sửa và review chéo với migration `.ts` thật (`0004_orca_app_tables.ts`, `0007_projects.ts`) để đối chiếu.
