# BE-CV-SOL-011-data-model-and-migrations: Migration `0002_code_intel_core` và domain của `code-intel-service`

> **📋 Proposed.** Chưa chạy migration, build hay test nào. Phần thứ nhất của CR-CV-011 (schema + domain). Phần repository và bảo trì ở [`BE-CV-SOL-011-repositories-and-maintenance`](./BE-CV-SOL-011-repositories-and-maintenance.md).

**CR:** [CR-CV-011](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md)
**Service:** `code-intel-service` (`migrations/{postgres,mysql}`, `internal/domain`)
**TDD tham chiếu:** [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (mục "Multi-tenancy", "Migration conventions"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "Multi-tenancy isolation"), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain không phụ thuộc adapter)

---

## Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| §4.1 | Kiểu khái niệm → Postgres/MySQL; không FK; `tenant_id` mọi bảng; RLS `FORCE` + `NULLIF`; MySQL lọc `tenant_id`; khoá lạc quan `version` |
| §4.2 (bảng migration) | `0002_code_intel_core` tạo **bảy** bảng: `tenant_settings`, `repo_bindings`, `graph_snapshots`, `review_states`, `finding_dismissals`, `c4_overrides`, `reindex_jobs`; đủ cột, **không `ALTER` sau** (PQ-24) |
| T1–T7 | Cột/khoá/chỉ mục từng bảng (không chép lại ở đây; mục 2.B chỉ nêu điểm đặc thù) |
| PQ-05 | `finding_dismissals`: khoá `repo_id`, `disposition`, `note`, `reason ≤ 500`, `finding_key ≤ 128` |
| PQ-15 | `graph_snapshots`: `head_commit`, `etag`, `total_count`, `schema_version`, UNIQUE theo `head_commit` |
| PQ-16 | `reindex_jobs`: `status` 5 giá trị DB (`cancelling`→`running`, `interrupted`→`failed` + `error_code`), `percent smallint NULL`, `trigger`, `outcome` |
| PQ-22 | `review_states`: `turn_markers`, dòng mức worktree (`base_commit=''`, `head_commit=''`), `status open|reviewed` |
| PQ-24 | `tenant_settings` đủ cột ngay từ `0002`; `code_intel_enabled` không DEFAULT |
| PQ-03 | Mã lỗi `CODEINTEL_INVALID_PARAMS` (không `INVALID_ARGUMENT`), `CODEINTEL_PAYLOAD_TOO_LARGE`, `CODEINTEL_NOT_FOUND`, `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_REINDEX_IN_PROGRESS`, `CODEINTEL_ALREADY_EXISTS`, `CODEINTEL_NO_TENANT` |
| PQ-14 (2)(3) | Trần payload, `payload_bytes ≤ 16 MiB`, TTL 7 ngày, 3 commit/`(binding, view)` |
| H10, H8 | Hai dialect; không bí mật/mã nguồn trong cột log |

## Lệch giữa CR và hợp đồng

| # | CR-CV-011 nói | Hợp đồng | Xử lý (hợp đồng thắng) |
|---|---|---|---|
| L1 | `tenant_settings` chỉ `code_intel_enabled`, `updated_by`, `updated_at` | T1: thêm 8 cột cờ (`quality_gate_enabled`, `quality_security_scan_enabled`, `index_policy`, `ai_review_level`, `ai_review_model`, `agent_turn_store_prompt_excerpt`, `agent_claim_text_enabled`, `hotspot_window_days`) | Tạo đủ trong `0002` |
| L2 | `graph_snapshots`: cột `commit`; thiếu `etag`, `total_count`; trần 8 MiB (max 12 MiB); TTL 24 h | PQ-15, PQ-14: `head_commit`, `etag char(32)`, `total_count bigint`; mặc định 3 MiB (max 8 MiB); TTL 7 ngày | Theo hợp đồng; CHECK `payload_bytes ≤ 16777216` giữ |
| L3 | `reindex_jobs`: `percent NOT NULL DEFAULT 0`, `requested_by NOT NULL`, không `trigger/outcome/agent_job_id/trigger_event_id` | T7, PQ-16 | `percent smallint NULL`; `requested_by uuid NULL` (NULL = hệ thống); thêm `trigger`, `trigger_event_id`, `outcome`, `agent_job_id` |
| L4 | `finding_dismissals`: `finding_key varchar(255)`, `reason text ≤ 1000`, không `disposition`/`note` | PQ-05, T5 | `finding_key v128`, `reason v500`, `note v500`, `disposition` (`ignored|resolved`) |
| L5 | `review_states.status varchar(32)` không CHECK | T4: `v16`, `open|reviewed`; thêm `turn_markers` | Theo T4; kiểm ở domain **và** CHECK (tập đóng đã chốt) |
| L6 | `c4_overrides.document` ≤ 128 KiB | T6/PQ-14 (5): ≤ 64 KiB nghiệp vụ, 128 KiB là trần cột | Domain giới hạn 64 KiB; cột `TEXT`/`MEDIUMTEXT` |
| L7 | `INVALID_ARGUMENT` | PQ-03 (2) | Đổi thành `CODEINTEL_INVALID_PARAMS` |
| L8 | Chính sách bảo trì với điều kiện hàng theo cấu hình retention (7 ngày…) | Cấu hình retention là env (SOL-010) | Xem D3: chính sách chỉ chặn bằng điều kiện **không phụ thuộc cấu hình**; mốc thời gian nằm ở truy vấn Go |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-010-scaffold-code-intel-service` | Cần module, `0001`, `withTenantTx/withRelayTx`, `OutboxRecord` |
| BE | `BE-CV-SOL-011-repositories-and-maintenance` | Dùng schema và domain của solution này |
| BE | `BE-CV-SOL-022-snapshot-cache` | Dùng `graph_snapshots` (`view`, `schema_version`, `etag`); tập `view` ở §4.2 T3 |
| BE | `BE-CV-SOL-082-quality-run-storage-and-ingest` | Migration `0003` đặt **sau** `0002`; đọc `ls migrations/postgres` trước khi đặt số |
| BE | `BE-CV-SOL-073-settings-flag-and-rollout` | Ghi `tenant_settings` qua `SetSettings` |
| AG / FE | — | Không có |

Thứ tự (§7.2): `010 → 011 → 012 → 013`.

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc (2026-10-06): `backend-go/services/mcp-service/migrations/postgres/{0001_init,0002_authorization}.up.sql` (mẫu RLS `FORCE`, chính sách `relay_*`, `reconcile_read` ở `0002` dòng 50–75), `mcp-service/internal/adapter/postgres/tenant_tx.go`, `common/dbcapability/capability.go` (`DetectDialectFromDSN`), `common/apperrors/apperrors.go`, `services/task-service/internal/adapter/{postgres,mysql}/task_sources.go` (dòng 14–15: `pgUniqueViolation = "23505"`, `mysqlDuplicateEntry = 1062`), `services/project-service/internal/usecase/lifecycle_events.go` (payload `worktree_id`, `project_id`), `services/api-gateway/internal/adapter/wscompat/channels_worktree.go` (dòng 738–745 `stripWorktreeSelectorPrefix`; dòng 800–830 id `repoID + "::" + path`, `IsMainWorktree = i == 0`), `frontend/src/shared/worktree-id.ts` (`FOLDER_WORKSPACE_INSTANCE_SEPARATOR = '::workspace:'`).

### Correction relative to CR-CV-011

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | Mẫu RLS bảo trì `reconcile_read` ở `0002_authorization` dòng 59–64 | Có ở `0002_authorization.up.sql` (đọc dòng 50–75): `FOR SELECT USING (current_setting('app.relay', true) = 'on' AND status = 'revoked' AND …)`; nó dùng cờ `app.relay`, **không** có cờ `app.maintenance` trong `mcp-service` | `app.maintenance` là cờ **mới** của service này (hợp đồng §4.1 cho phép); cần `withMaintenanceTx` mới (SOL-011 repositories) |
| C2 | `task-service/migrations/postgres/0012_task_sources.up.sql` dòng 19–21 dùng `COALESCE` cho UNIQUE | **Chưa đọc** file này ở lần soạn này (chỉ đọc `task_sources.go`) | Không dùng làm bằng chứng; lý do `scope_key` đứng độc lập (MySQL không có chỉ mục duy nhất từng phần) vẫn đúng theo hiểu biết MySQL, ghi "chưa kiểm chứng" |
| C3 | `*pgconn.PgError` 23505 / `*mysql.MySQLError` 1062 | Đúng: hằng `pgUniqueViolation`, `mysqlDuplicateEntry` ở hai `task_sources.go` | Dùng cùng cách phát hiện |
| C4 | Mã `reindex_jobs.status` và cột ít | Xem L3 | Theo hợp đồng |
| C5 | Tên cột `view`, `commit`, `at` có thể trùng từ khoá (CR mục 6) | Hợp đồng đã đổi `commit` → `head_commit`; còn `view`, `at`. **Cột `trigger` (T7) là từ dành riêng của MySQL 8** theo hiểu biết về danh sách từ khoá (chưa kiểm chứng bằng chạy); Postgres coi là từ khoá không dành riêng | Mọi câu MySQL dùng `` `trigger` `` có dấu nháy ngược; task 011-01 kiểm bằng chạy DDL; nếu cản, đề nghị chủ hợp đồng đổi tên cột (ví dụ `trigger_kind`) |

Chưa kiểm chứng: phiên bản MySQL/TiDB mục tiêu (CHECK cần 8.0.16+); `JSON` MySQL từ chối UTF-8 hỏng; `jsonb` từ chối `\u0000`; chi phí `NOT EXISTS` mồ côi; `repo_id` ổn định qua `RebindRepoDevServer`/`AssignRepoToProject` (project-service chưa đọc hết).

## 2. Giải pháp

### A. Cây file (mới, tiền tố `backend-go/services/code-intel-service/`)

```
migrations/postgres/0002_code_intel_core.{up,down}.sql
migrations/mysql/0002_code_intel_core.{up,down}.sql
internal/domain/tenant_settings.go
internal/domain/repo_binding.go            # RepoBinding, IndexScope, BindingScope (wt:/path:)
internal/domain/graph_snapshot.go          # GraphSnapshot, SnapshotKey, CanonicalParamsHash
internal/domain/review_state.go            # ReviewState (reading_progress, notes, turn_markers)
internal/domain/finding_dismissal.go
internal/domain/c4_override.go
internal/domain/reindex_job.go             # ReindexJob, ReindexStatus, CanTransition
internal/domain/payload_limits.go          # hằng giới hạn (mục D)
internal/domain/code_intel_errors.go       # hàm dựng AppError (mục E)
```

### B. Migration `0002`: điểm đặc thù (cột đầy đủ theo hợp đồng T1–T7)

Postgres, ví dụ hai bảng có ràng buộc đặc thù:

```sql
CREATE TABLE codeintel.reindex_jobs (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, repo_binding_id UUID NOT NULL,
  mode VARCHAR(16) NOT NULL, status VARCHAR(16) NOT NULL,
  stage VARCHAR(32) NOT NULL DEFAULT '', percent SMALLINT,
  outcome VARCHAR(32) NOT NULL DEFAULT '', trigger VARCHAR(16) NOT NULL DEFAULT 'manual',
  trigger_event_id VARCHAR(64), requested_by UUID,
  agent_job_id VARCHAR(64) NOT NULL DEFAULT '', active_key UUID,
  message VARCHAR(500) NOT NULL DEFAULT '', error_code VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), started_at TIMESTAMPTZ, finished_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), version BIGINT NOT NULL DEFAULT 1,
  CONSTRAINT chk_reindex_status CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
  CONSTRAINT chk_reindex_mode CHECK (mode IN ('incremental','full')),
  CONSTRAINT chk_reindex_percent CHECK (percent IS NULL OR percent BETWEEN 0 AND 100),
  CONSTRAINT uq_reindex_active_key UNIQUE (active_key));
CREATE INDEX idx_reindex_binding ON codeintel.reindex_jobs (tenant_id, repo_binding_id, created_at);
CREATE INDEX idx_reindex_status_updated ON codeintel.reindex_jobs (status, updated_at);
```

MySQL tương đương (`CHAR(36)`, `` `trigger` ``, `TINYINT(1)`, `JSON`; `UNIQUE KEY uq_reindex_active_key (active_key)` cho phép nhiều `NULL`). Tên ràng buộc đặt tường minh để adapter phân biệt vi phạm `active_key` với PK ở Postgres (`ConstraintName`) và MySQL (chuỗi `Message` chứa tên khoá).

Các bảng còn lại theo hợp đồng nguyên văn: `repo_bindings` (UNIQUE `(tenant_id, project_id, scope_key)`; chỉ mục `(tenant_id, dev_server_id, path_hash)`, `(tenant_id, repo_id)`; CHECK `index_scope IN ('exact','repo_root','unresolved')`), `graph_snapshots` (UNIQUE `(tenant_id, repo_binding_id, view, head_commit, params_hash)`; chỉ mục `(expires_at)`, `(tenant_id, repo_binding_id, created_at)`; CHECK `payload_bytes <= 16777216`), `review_states` (UNIQUE `(tenant_id, repo_binding_id, base_commit, head_commit)`; CHECK `status IN ('open','reviewed')`), `finding_dismissals` (UNIQUE `(tenant_id, repo_id, finding_key)`; CHECK `disposition IN ('ignored','resolved')`), `c4_overrides` (UNIQUE `(tenant_id, repo_id, container)`; MySQL `document MEDIUMTEXT`), `tenant_settings` (PK `tenant_id`).

**Ước lượng độ dài khoá MySQL (utf8mb4, 4 byte/ký tự, giới hạn InnoDB 3072 byte):** `graph_snapshots` UNIQUE `(36+36+32+64+64)×4 = 928`; `c4_overrides` `(36+36+255)×4 = 1308`; `repo_bindings` `(36+36+128)×4 = 800`; `review_states` `(36+36+64+64)×4 = 800`. Đều dưới giới hạn; chưa chạy.

### C. Postgres: RLS và chính sách bảo trì

- Cả bảy bảng: `ENABLE` + `FORCE ROW LEVEL SECURITY`; `tenant_isolation` (`USING` + `WITH CHECK`, `NULLIF(current_setting('app.tenant_id', true), '')::uuid`).
- Chính sách bảo trì (cờ `app.maintenance = 'on'`, **không bao giờ `INSERT`**, mọi hành động ghi chỉ trên điều kiện không phụ thuộc cấu hình):
  - `maint_read` `FOR SELECT` trên `graph_snapshots`, `repo_bindings`, `reindex_jobs`, `review_states` (để quét tenant/dữ liệu mồ côi) và trên `outbox_events`, `processed_events` (bảng của `0001`, thêm chính sách ở `0002`).
  - `maint_delete_expired` `FOR DELETE` trên `graph_snapshots` với `expires_at < now()`; `maint_delete_published` trên `outbox_events` với `published_at IS NOT NULL`; `maint_delete_old` trên `processed_events` (chỉ cờ).
- Các việc cần mốc theo cấu hình (mồ côi 7 ngày, binding rảnh 90 ngày, job quá hạn 45 phút, vượt hạn mức) chạy **theo từng tenant** trong `withTenantTx` sau khi `maint_read` liệt kê tenant, nên không cần chính sách ghi rộng (D3).

### D. Domain

- Mọi constructor kiểm: `tenant_id` không rỗng; `json.Valid`; `utf8.Valid` (ở cả hai dialect cho hành vi giống nhau); kích thước; tập giá trị.
- `payload_limits.go`: `MaxReadingProgressBytes=64<<10`, `MaxNotesBytes=256<<10`, `MaxNotesItems=500`, `MaxTurnMarkersBytes=256<<10`, `MaxTurnMarkers=5`, `MaxC4DocumentBytes=64<<10`, `MaxLastStatusBytes=64<<10`, `MaxReindexMessageChars=500`, `MaxFindingKeyChars=128`, `MaxDismissReasonChars=500`, `MaxDismissNoteChars=500`, `MaxSnapshotFrameBytes=16<<20`; trần snapshot theo cấu hình (SOL-010).
- `CanonicalParamsHash(any) (string, error)`: JSON khoá sắp xếp, số chuẩn hoá, SHA-256 hex (64). Không dựa thứ tự khoá của `jsonb` đã lưu.
- Bỏ ký tự NUL (`\x00`) khỏi chuỗi trong `payload` trước khi ghi (Postgres `jsonb` từ chối `\u0000`), đặt `truncated=true`, không đổi khoá.
- `ReindexJob.CanTransition(from, to)`: `queued→running|failed|cancelled`; `running→succeeded|failed|cancelled`; trạng thái cuối không đổi. `interrupted` (agent) lưu `failed` + `error_code='CODEINTEL_REINDEX_INTERRUPTED'`; `cancelling` hiển thị `running` (PQ-16).
- `RepoBinding`: `IndexScope` ∈ `exact|repo_root|unresolved`; `BindingScope` sinh `scope_key` (`wt:<worktree_id>` hoặc `path:<repo_id>:<sha256 hex đường dẫn chuẩn hoá>`); hàm sinh nằm ở SOL-012 (cần chuẩn hoá đường dẫn); domain chỉ kiểm định dạng.
- `TenantSettings`: giá trị mặc định khi tạo lười lấy từ cấu hình (`TenantDefaultEnabled`, `TenantDefaultQualityGateEnabled`); các cột còn lại mặc định theo T1.

### E. Lỗi (`apperrors`, mã tiền tố `CODEINTEL_`)

| Mã | Kind | Khi |
|---|---|---|
| `CODEINTEL_NO_TENANT` | Unauthenticated | không có tenant trong ctx |
| `CODEINTEL_INVALID_PARAMS` | InvalidArgument | giá trị ngoài tập, JSON/UTF-8 hỏng (PQ-03) |
| `CODEINTEL_PAYLOAD_TOO_LARGE` | InvalidArgument | vượt giới hạn D |
| `CODEINTEL_NOT_FOUND` | NotFound | id không có trong tenant (kể cả của tenant khác) |
| `CODEINTEL_VERSION_CONFLICT` | FailedPrecondition | CAS `version` lệch |
| `CODEINTEL_REINDEX_IN_PROGRESS` | FailedPrecondition | `UNIQUE (active_key)` bị vi phạm |
| `CODEINTEL_ALREADY_EXISTS` | AlreadyExists | trùng khoá duy nhất khác |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `scope_key` thay `worktree_id` nullable trong UNIQUE | MySQL không có chỉ mục duy nhất từng phần; id worktree có ba dạng chuỗi |
| D2 | `c4_overrides`, `finding_dismissals` khoá `repo_id`; `review_states`, `graph_snapshots`, `reindex_jobs` khoá `repo_binding_id` | Tri thức của repo sống sót khi worktree/binding bị xoá (F7) |
| D3 | Chính sách bảo trì hẹp, mốc theo cấu hình ở Go và chạy theo từng tenant | Retention cấu hình bằng env; hard-code mốc trong policy sẽ lệch cấu hình |
| D4 | `reindex_jobs.active_key` UNIQUE nullable | Một job mỗi binding, đúng ở cả hai dialect |
| D5 | CHECK trên tập đóng (`status`, `mode`, `index_scope`, `disposition`, `index_policy`, `ai_review_level`); không CHECK trên `view` (PQ-15) | Tập đóng đã chốt; `view` mở rộng theo CR sau |
| D6 | Khoá lạc quan `version` | Giống nhau hai dialect |
| D7 | Không lưu bí mật; ghi chú review không che khi lưu, không vào log | Ghi chú do người dùng gõ (xem SOL-013 mục che) |

## 4. Tiêu chí chấp nhận

- [x] `0002_code_intel_core` up/down/up sạch trên Postgres và MySQL; `0002` cũng thêm chính sách bảo trì cho `outbox_events`/`processed_events` của `0001`, và `down` gỡ chúng.
- [x] Mỗi CHECK từ chối giá trị sai (chèn trực tiếp từng giá trị lỗi); MySQL < 8.0.16 bị bỏ qua CHECK nên domain là hàng rào thật (ghi trong README service).
- [x] Hai `INSERT` job cùng `active_key` → vi phạm duy nhất; `active_key` NULL nhiều dòng được phép (cả hai dialect).
- [x] Postgres role `NOSUPERUSER NOBYPASSRLS`: mỗi bảng cô lập tenant bằng SQL trực tiếp; `app.maintenance='on'` đọc chéo tenant nhưng không `INSERT`, `DELETE` chỉ khớp điều kiện hàng đã nêu.
- [x] Domain: JSON hỏng, UTF-8 hỏng, vượt giới hạn, ngoài tập → `CODEINTEL_INVALID_PARAMS`/`CODEINTEL_PAYLOAD_TOO_LARGE`; `CanonicalParamsHash` ổn định khi đổi thứ tự khoá.
- [x] Hợp đồng schema: tên bảng, cột, kiểu, tính NULL khớp bảng mong đợi ở cả hai DB (đọc `information_schema`).
- [x] `go vet`, `make lint` xanh; không `max-lines` disable; không file tên chung chung.

## 5. Kiểm thử

- **Unit (không DB):** constructor domain (bảng giá trị xấu); `CanTransition` (bảng đầy đủ); `CanonicalParamsHash` (đổi thứ tự khoá, số thực); bỏ NUL; hằng giới hạn.
- **Integration hai dialect (`-tags=integration`, `common/testutil`):** migration up/down/up; CHECK; UNIQUE (`active_key`, `scope_key`); độ dài khoá MySQL (tạo được UNIQUE); Postgres RLS role không superuser.
- **Hợp đồng schema:** so `information_schema` với danh sách mong đợi sinh từ hợp đồng T1–T7 (một nguồn trong test, tránh lệch).
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- `` `trigger` `` (C5): nếu MySQL cản dù có nháy, cần đổi tên cột (đề nghị hợp đồng).
- Kiểu `TIMESTAMP(6)` MySQL trần 2038; `expires_at` chỉ cộng giờ nên không ảnh hưởng.
- `jsonb` sắp xếp lại khoá và gộp khoá trùng; không so sánh nội dung theo thứ tự khoá.
- CHECK MySQL phụ thuộc phiên bản; TiDB chưa kiểm.
- Kích thước snapshot thật chưa đo (SOL-071); 3 MiB mặc định và 512 MiB/tenant là đề xuất.
- Migration `0002` tạo bảng cho CR sau (`graph_snapshots.view` mở); số migration của CR sau có thể dịch.

## 7. Câu hỏi mở

- **Q1.** Đổi tên cột `trigger` nếu MySQL cản (chủ hợp đồng).
- **Q2.** `reading_progress` dùng chung nhóm hay riêng người (README v7 O điểm mở; mặc định một dòng chung).
- **Q3.** Có cần CHECK `hotspot_window_days BETWEEN 30 AND 365` ở DB hay chỉ domain? Mặc định: cả hai.
- **Q4.** Nén snapshot (`bytea`/`LONGBLOB` gzip) sau khi đo (SOL-071).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` §4.1–4.3, PQ-03, PQ-05, PQ-14–16, PQ-22, PQ-24
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md`
- `/opt/repos/orca/backend-go/services/mcp-service/migrations/postgres/{0001_init,0002_authorization}.up.sql`, `internal/adapter/postgres/tenant_tx.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/{postgres,mysql}/task_sources.go`
- `/opt/repos/orca/backend-go/common/{apperrors,dbcapability,testutil}`
