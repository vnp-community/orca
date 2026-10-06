# CR-CV-011 — Mô hình dữ liệu, migration và repository của `code-intel-service`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-011 |
| **Tên** | Schema, entity miền và repository (Postgres + MySQL) cho binding repo, snapshot đồ thị, trạng thái review, bỏ qua phát hiện, ghi đè C4, job làm mới index, cờ tenant |
| **Loại** | Feature (nền dữ liệu) |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010 |
| **Mở khoá** | CR-CV-012, 013, 022, 024, 033, 036, 037, 052, 059, 060, 073 |
| **Tác động** | `backend-go/services/code-intel-service/{internal/domain, internal/usecase/ports.go, internal/adapter/postgres, internal/adapter/mysql, migrations/postgres, migrations/mysql}` (mới). Không đụng service khác |

---

## 1. Bối cảnh và vấn đề

CR-CV-010 dựng khung, `outbox_events` và `processed_events`. README v7 mục 3.5 liệt kê các bảng nghiệp vụ và trường chính nhưng chưa có kiểu cột, ràng buộc, chỉ mục, giới hạn kích thước, cách dọn, khác biệt hai dialect. Khi đọc code thật (project-service, git-gateway, api-gateway) phát hiện ba điều làm mô hình ở README cần chỉnh:

1. **`worktree_id` không phải lúc nào cũng là UUID.** Id worktree mà frontend và gateway đang dùng có ba dạng: `project.worktrees.id` (UUID, worktree do Orca tạo); id tổng hợp `<repoId>::<path>` cho worktree "external" chưa được project-service ghi (`api-gateway/internal/adapter/wscompat/channels_worktree.go` dòng 792 đến 827 (id ở dòng 824), `view.ID = repoID + "::" + info.GetPath()`); và id thư mục làm việc `...::workspace:<uuid>` (`frontend/src/shared/worktree-id.ts`). Ngoài ra checkout chính (`Repo.url`) cũng có thể không có dòng worktree riêng. Cột `worktree_id UUID` của README không chứa được các dạng sau; xem CR-CV-012 mục 2.2.
2. **Binding theo worktree sẽ làm mất dữ liệu dùng chung.** `repo_bindings` có một dòng cho mỗi worktree; khi worktree bị xoá (project-service phát `orca.project.worktree.deleted`, `project-service/internal/usecase/lifecycle_events.go` dòng 22 đến 25) binding bị xoá. `c4_overrides` và `finding_dismissals` theo `repo_binding_id` (README) sẽ mất cùng, trong khi ghi đè C4 và "bỏ qua phát hiện" là tri thức của cả repo.
3. **RLS có hiệu lực hay không phụ thuộc cách viết.** Xem CR-CV-010 mục 1.1 điểm 1, 2: dùng `set_config('app.tenant_id')` trong transaction + `FORCE ROW LEVEL SECURITY` (mẫu `mcp-service`), không theo `task-service`.

CR này chỉ sở hữu **schema và repository**. Ý nghĩa nghiệp vụ nằm ở CR khác: binding (CR-CV-012), cờ và hạn mức (CR-CV-013, 073), cache (CR-CV-022), C4 (CR-CV-033), review (CR-CV-052, 060), phát hiện (CR-CV-037, 059).

## 2. Giải pháp đề xuất

### 2.1 Migration `0002_code_intel_core` (up và down, hai dialect)

Kiểu: Postgres `UUID`, `TIMESTAMPTZ`, `JSONB`, `BOOLEAN`; MySQL `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `TINYINT(1)`. Id do ứng dụng sinh (`uuid.NewString()`). Postgres: mỗi bảng `ENABLE` và `FORCE ROW LEVEL SECURITY` với `tenant_isolation` theo `NULLIF(current_setting('app.tenant_id', true), '')::uuid` (cả `USING` và `WITH CHECK`) và các chính sách bảo trì ở mục 2.5. Bảng nào cũng có `tenant_id NOT NULL`, không FK sang bảng service khác; giữa các bảng của service này cũng không FK (dọn bằng ứng dụng, mục 2.5), giống quy ước v6.

Các cột đánh dấu **(bổ sung)** là thêm so với README v7 mục 3.5; xem mục 7 để ghi vào README.

**`tenant_settings`** (bổ sung; README mục 3.5 chưa có, nhưng O8 cần nơi lưu cờ `code_intel_enabled`, cùng mẫu `mcp.tenant_settings` và `request.tenant_settings` của CR-REQ-025)

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `tenant_id` | uuid | PK |
| `code_intel_enabled` | boolean | NOT NULL, không DEFAULT ngầm: use case luôn chèn giá trị từ `CODEINTEL_TENANT_DEFAULT_ENABLED` (như `mcp-service` D6) |
| `updated_by` | uuid | NULL (NULL = hệ thống) |
| `updated_at` | timestamptz | NOT NULL |

CR-CV-073 sở hữu RPC bật/tắt và rollout; CR-CV-013 chỉ đọc (mục 2.3 của CR đó).

**`repo_bindings`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `project_id` | uuid | NOT NULL, id project của `project-service` |
| `repo_id` | uuid | NOT NULL (bổ sung), id `Repo` của `project-service` |
| `worktree_id` | uuid | NULL; chỉ có giá trị khi worktree do Orca ghi (`project.worktrees.id`) |
| `scope_key` | varchar(128) | NOT NULL (bổ sung). `wt:<worktree_id>` hoặc `path:<repo_id>:<sha256 hex của đường dẫn đã chuẩn hoá>` |
| `dev_server_id` | uuid | NOT NULL |
| `workspace_root` | text | NOT NULL, đường dẫn tuyệt đối trên dev server (không chỉ mục) |
| `path_hash` | char(64) | NOT NULL (bổ sung), SHA-256 hex của đường dẫn đã chuẩn hoá (CR-CV-012 mục 2.4); dùng tra binding từ `codeintel.indexChanged {workspaceRoot}` |
| `gitnexus_repo` | varchar(255) | NOT NULL DEFAULT '' (tên trong registry GitNexus; '' = chưa có/không có) |
| `codegraph_path` | text | NOT NULL DEFAULT '' |
| `index_scope` | varchar(16) | NOT NULL (bổ sung), CHECK IN (`exact`,`repo_root`,`unresolved`) |
| `last_status` | jsonb/json | NULL (bổ sung), bản `IndexStatus` gần nhất, ≤ 64 KiB |
| `last_status_at` | timestamptz | NULL (bổ sung) |
| `created_at`, `updated_at` | timestamptz | NOT NULL, đồng hồ DB |
| `version` | bigint | NOT NULL DEFAULT 1 (bổ sung; khoá lạc quan) |

Duy nhất `(tenant_id, project_id, scope_key)`. Chỉ mục `(tenant_id, dev_server_id, path_hash)`, `(tenant_id, repo_id)`. Cột `scope_key` thay cho "UNIQUE trên cột nullable" vì MySQL không có chỉ mục duy nhất có điều kiện và `NULL` không bao giờ trùng (cùng vấn đề `task_sources` phải dùng `COALESCE`, `task-service/migrations/postgres/0012_task_sources.up.sql` dòng 19 đến 21).

**`graph_snapshots`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `repo_binding_id` | uuid | NOT NULL |
| `view` | varchar(32) | NOT NULL, tên view chuẩn (`overview`, `subgraph`, `impact`, `erd`, `c4`, ...; tập giá trị do CR-CV-022 chốt, không CHECK) |
| `commit` | varchar(64) | NOT NULL, SHA đầu vào của kết quả (HEAD worktree hoặc commit index) |
| `params_hash` | char(64) | NOT NULL, SHA-256 hex của tham số chuẩn hoá (JSON khoá sắp xếp) |
| `schema_version` | int | NOT NULL (bổ sung), phiên bản dạng snapshot; khác phiên bản hiện hành thì bỏ qua và dọn |
| `payload` | jsonb/json | NOT NULL, ≤ `CODEINTEL_SNAPSHOT_MAX_BYTES` |
| `payload_bytes` | int | NOT NULL, CHECK `<= 16777216` (trần khung 16 MiB, `research/view-code/01` mục 3) |
| `truncated` | boolean | NOT NULL |
| `tool_versions` | jsonb/json | NOT NULL, ví dụ `{"gitnexus":"1.6.9","codegraph":"1.4.1"}` |
| `created_at` | timestamptz | NOT NULL |
| `expires_at` | timestamptz | NOT NULL |

Duy nhất `(tenant_id, repo_binding_id, view, commit, params_hash)`; ghi lại cùng khoá là thay thế (upsert, cập nhật cả `schema_version`, `payload`, `created_at`, `expires_at`). Chỉ mục `(expires_at)` cho dọn, `(tenant_id, repo_binding_id, created_at)` cho loại bỏ theo binding. Kích thước khoá MySQL: khoảng 1 KiB (utf8mb4), dưới giới hạn 3072 byte của InnoDB; chưa chạy.

**`review_states`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `repo_binding_id` | uuid | NOT NULL (thay `worktree_id` của README; xem mục 7) |
| `base_commit`, `head_commit` | varchar(64) | NOT NULL (O7: merge-base và HEAD) |
| `reading_progress` | jsonb/json | NOT NULL, ≤ 64 KiB, ví dụ `{"seen":["file:...","symbol:..."]}` |
| `notes` | jsonb/json | NOT NULL, ≤ 256 KiB và ≤ 500 mục |
| `status` | varchar(32) | NOT NULL; tập giá trị do CR-CV-052/060 chốt, kiểm ở domain, không CHECK |
| `updated_by` | uuid | NOT NULL |
| `updated_at` | timestamptz | NOT NULL |
| `version` | bigint | NOT NULL DEFAULT 1 |

Duy nhất `(tenant_id, repo_binding_id, base_commit, head_commit)`. Trạng thái dùng chung cho cả nhóm (một dòng cho mỗi cặp commit); tiến độ riêng từng người là câu hỏi mở Q3.

**`finding_dismissals`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `repo_id` | uuid | NOT NULL (bổ sung; khoá chính của nghiệp vụ, xem mục 1 điểm 2) |
| `repo_binding_id` | uuid | NOT NULL (giữ làm xuất xứ) |
| `finding_key` | varchar(255) | NOT NULL; khoá ổn định do CR-CV-037 sinh (quy tắc + `SymbolRef.key`, đã băm nếu dài) |
| `reason` | text | NOT NULL DEFAULT '', ≤ 1000 ký tự |
| `dismissed_by` | uuid | NOT NULL |
| `at` | timestamptz | NOT NULL |

Duy nhất `(tenant_id, repo_id, finding_key)`. "Khôi phục" là xoá dòng (sự kiện audit ghi ở CR-CV-013).

**`c4_overrides`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `repo_id` | uuid | NOT NULL (bổ sung) |
| `repo_binding_id` | uuid | NOT NULL (xuất xứ) |
| `container` | varchar(255) | NOT NULL, tên container C4 (thường là tên service) |
| `document` | text (MySQL `MEDIUMTEXT`) | NOT NULL, YAML/JSON, ≤ 128 KiB; parse và kiểm schema do CR-CV-033 |
| `updated_by` | uuid | NOT NULL |
| `updated_at` | timestamptz | NOT NULL |
| `version` | bigint | NOT NULL DEFAULT 1 |

Duy nhất `(tenant_id, repo_id, container)`.

**`reindex_jobs`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `repo_binding_id` | uuid | NOT NULL |
| `mode` | varchar(16) | NOT NULL; tập giá trị do CR-CV-004 |
| `status` | varchar(16) | NOT NULL, CHECK IN (`queued`,`running`,`succeeded`,`failed`,`cancelled`) |
| `stage` | varchar(32) | NOT NULL DEFAULT '' |
| `percent` | smallint | NOT NULL DEFAULT 0, CHECK 0 đến 100 (bổ sung) |
| `requested_by` | uuid | NOT NULL (bổ sung) |
| `active_key` | uuid | NULL, UNIQUE (bổ sung): bằng `repo_binding_id` khi `status` ∈ {`queued`,`running`}, NULL khi kết thúc |
| `message` | varchar(500) | NOT NULL DEFAULT '' (bổ sung; đã che bí mật, cắt) |
| `created_at` | timestamptz | NOT NULL (bổ sung) |
| `started_at`, `finished_at` | timestamptz | NULL |
| `updated_at` | timestamptz | NOT NULL (bổ sung; dùng phát hiện job mồ côi) |
| `error_code` | varchar(64) | NOT NULL DEFAULT '' |
| `version` | bigint | NOT NULL DEFAULT 1 |

`UNIQUE (active_key)` thực thi "mỗi repo một job đang chạy" (README v7 mục 3.2, `codeintel.reindex` "một lần/tại một thời điểm") ở cả hai dialect: cột `NULL` không vi phạm tính duy nhất ở cả Postgres lẫn MySQL, nên không cần chỉ mục từng phần (MySQL không có). Chỉ mục `(tenant_id, repo_binding_id, created_at)`; `(status, updated_at)` cho dọn.

Down: `DROP TABLE` ngược thứ tự tạo (chính sách trước, bảng sau).

### 2.2 Domain (`internal/domain`, tất cả mới)

Tên file theo khái niệm: `repo_binding.go` (`RepoBinding`, `BindingScope`, `IndexScope`), `graph_snapshot.go` (`GraphSnapshot`, `SnapshotKey`), `review_state.go`, `finding_dismissal.go`, `c4_override.go`, `reindex_job.go` (`ReindexJob`, `ReindexStatus`, hàm `CanTransition`), `tenant_settings.go`, `payload_limits.go` (hằng giới hạn), `code_intel_errors.go`. Mỗi constructor kiểm tenant, JSON hợp lệ (`json.Valid`), UTF-8 và kích thước trước khi chạm DB.

Lỗi dùng `common/apperrors` (mã tiền tố `CODEINTEL_`, README v7 mục 3.3):

| Mã | Kind | Khi |
|----|------|-----|
| `CODEINTEL_NO_TENANT` | Unauthenticated | không có tenant trong context |
| `CODEINTEL_INVALID_ARGUMENT` | InvalidArgument | giá trị ngoài tập, JSON hỏng, UTF-8 hỏng |
| `CODEINTEL_PAYLOAD_TOO_LARGE` | InvalidArgument | vượt giới hạn mục 2.3 |
| `CODEINTEL_NOT_FOUND` | NotFound | không có id trong tenant (kể cả id của tenant khác) |
| `CODEINTEL_VERSION_CONFLICT` | FailedPrecondition | CAS `version` không khớp |
| `CODEINTEL_REINDEX_IN_PROGRESS` | FailedPrecondition | `UNIQUE (active_key)` bị vi phạm; trùng tên mã agent ở README v7 mục 3.3, một ý nghĩa |
| `CODEINTEL_ALREADY_EXISTS` | AlreadyExists | trùng khoá duy nhất khác (ví dụ binding cùng `scope_key`) |

`apperrors` không có `Kind` cho `ResourceExhausted`/`Unavailable` (`common/apperrors/apperrors.go`, bảng ánh xạ trong `ToGRPCStatus` chỉ có 8 loại); xem CR-CV-013 mục 2.6 cho lỗi hạn mức.

### 2.3 Giới hạn kích thước

| Dữ liệu | Giới hạn mặc định | Cấu hình | Hành vi khi vượt |
|---------|-------------------|----------|-------------------|
| `graph_snapshots.payload` | 8 MiB | `CODEINTEL_SNAPSHOT_MAX_BYTES` (tối đa 12 MiB, dưới khung 16 MiB) | `Put` trả `CODEINTEL_PAYLOAD_TOO_LARGE`; CR-CV-022 quyết định có trả kết quả không cache hay không |
| Tổng `payload_bytes` mỗi tenant | 512 MiB | `CODEINTEL_SNAPSHOT_TENANT_QUOTA_BYTES` | Công việc bảo trì xoá snapshot cũ nhất trước (mục 2.5) |
| `review_states.notes` | 256 KiB, 500 mục | cố định | `CODEINTEL_PAYLOAD_TOO_LARGE` |
| `review_states.reading_progress` | 64 KiB | cố định | như trên |
| `c4_overrides.document` | 128 KiB | cố định | như trên |
| `finding_dismissals.reason` | 1000 ký tự | cố định | `CODEINTEL_INVALID_ARGUMENT` |
| `reindex_jobs.message` | 500 ký tự | cố định | cắt, không lỗi |
| `repo_bindings.last_status` | 64 KiB | cố định | bỏ trường dài (`languages`, `pendingChanges`) rồi cắt; không lỗi |

Hai điều đặc thù JSON: Postgres `jsonb` từ chối chuỗi chứa `\u0000` ("unsupported Unicode escape sequence"); `codeintel.symbol` trả mã nguồn, có thể chứa byte NUL. Tầng lưu bỏ ký tự NUL trước khi ghi `payload` và đặt `truncated=true`, không đổi khoá. MySQL `JSON` từ chối UTF-8 không hợp lệ; domain kiểm `utf8.Valid` ở cả hai dialect để hành vi giống nhau. Cả hai điều là hành vi đã biết của hai DB, chưa chạy thử với dữ liệu thật của repo.

### 2.4 Cổng repository (`internal/usecase/ports.go`) và hành vi

Mọi phương thức ghi nhận `events []domain.OutboxRecord` khi README v7 mục 3.8 có sự kiện tương ứng và ghi outbox trong cùng transaction (mẫu CR-CV-010 mục 2.7). Mọi phương thức lấy tenant bằng `tenant.RequireTenantID(ctx)` và đưa `tenant_id` vào mọi `WHERE`, kể cả Postgres có RLS (phòng thủ chiều sâu; dev dùng superuser nên RLS không chạy).

| Cổng | Phương thức |
|------|-------------|
| `TenantSettingsRepository` | `Get(ctx)`, `GetOrCreate(ctx, defaultEnabled)`, `Set(ctx, enabled, updatedBy)` |
| `RepoBindingRepository` | `Upsert(ctx, RepoBinding)` (theo `(project_id, scope_key)`; giữ `id` cũ), `Get(ctx, id)`, `GetByScope(ctx, projectID, scopeKey)`, `ListByProject(ctx, projectID)`, `ListByPath(ctx, devServerID, pathHash)`, `ListRepoRootDependents(ctx, repoID)` (binding có `index_scope='repo_root'`), `SaveStatusCache(ctx, id, statusJSON, at)`, `Delete(ctx, id)`, `DeleteByWorktreeID(ctx, projectID, worktreeID)` |
| `SnapshotRepository` | `Put(ctx, GraphSnapshot)`, `Get(ctx, SnapshotKey, schemaVersion, now)` (chỉ trả dòng chưa hết hạn và đúng `schema_version`), `DeleteByBinding(ctx, bindingID, exceptCommit)`, `DeleteExpired(ctx, limit)`, `TenantBytes(ctx)` |
| `ReviewStateRepository` | `Get(ctx, bindingID, base, head)`, `Save(ctx, ReviewState, expectedVersion, events)` (`expectedVersion=0` nghĩa là tạo mới) |
| `FindingDismissalRepository` | `Dismiss(ctx, d)` (idempotent theo khoá), `Restore(ctx, repoID, findingKey)`, `ListByRepo(ctx, repoID)` |
| `C4OverrideRepository` | `Get(ctx, repoID, container)`, `ListByRepo(ctx, repoID)`, `Save(ctx, o, expectedVersion, events)`, `Delete(ctx, repoID, container)` |
| `ReindexJobRepository` | `Create(ctx, job, events)` (đặt `active_key`; vi phạm duy nhất → `CODEINTEL_REINDEX_IN_PROGRESS`), `Get`, `ListByBinding(ctx, bindingID, limit)`, `UpdateProgress(ctx, id, stage, percent, now)`, `Finish(ctx, id, status, errorCode, message, events)` (đặt `active_key=NULL`) |
| `MaintenanceRepository` | dùng riêng bởi công việc bảo trì, chạy ngoài tenant (mục 2.5) |

Hành vi theo dialect:

- **Thời gian:** so sánh hết hạn và đặt `expires_at` dùng đồng hồ DB (`now() + make_interval(secs => $n)` ở Postgres; `DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? SECOND)` ở MySQL), như quy ước F6 của v6.
- **CAS:** `UPDATE ... SET ..., version = version + 1, updated_at = now() WHERE id = ? AND tenant_id = ? AND version = ?`; 0 hàng thì `SELECT` để phân biệt `CODEINTEL_NOT_FOUND` với `CODEINTEL_VERSION_CONFLICT`. `version` luôn đổi nên `RowsAffected` của MySQL (chỉ tính hàng thật sự đổi) không gây nhầm.
- **Upsert:** Postgres `INSERT ... ON CONFLICT (...) DO UPDATE ... RETURNING`; MySQL `INSERT ... ON DUPLICATE KEY UPDATE` rồi `SELECT` lại trong cùng transaction (MySQL không có `RETURNING`; `dbcapability.Capabilities.SupportsReturning=false`).
- **Phát hiện vi phạm duy nhất:** Postgres `*pgconn.PgError` mã `23505` (như `task-service/internal/adapter/postgres/task_sources.go` dòng 14 đến 15); MySQL `*mysql.MySQLError` `Number == 1062`. Chưa kiểm chứng có nhiều ràng buộc duy nhất cùng bảng gây nhầm: `reindex_jobs` có PK và `active_key`; phân biệt bằng tên ràng buộc ở Postgres và `Message` ở MySQL.
- **Postgres tenant:** mọi phương thức chạy trong `withTenantTx` (`set_config('app.tenant_id', $1, true)`); công việc bảo trì chạy trong `withMaintenanceTx` (`app.maintenance='on'`), relay trong `withRelayTx` (CR-CV-010).
- **Danh sách:** không có danh sách không giới hạn; `ListByProject`, `ListByRepo` giới hạn 500, `ListByBinding` giới hạn do tham số (tối đa 100).

### 2.5 Dọn snapshot hết hạn và dữ liệu mồ côi

Công việc bảo trì (`internal/usecase/snapshot_maintenance.go`, mới) chạy mỗi `CODEINTEL_MAINTENANCE_INTERVAL` (10 phút) trong một goroutine do `main.go` khởi (tắt theo `ctx`). Mỗi vòng, theo lô 500 hàng để không giữ khoá lâu:

| Việc | Postgres | MySQL |
|------|----------|-------|
| Xoá snapshot hết hạn | `DELETE FROM codeintel.graph_snapshots WHERE id IN (SELECT id FROM codeintel.graph_snapshots WHERE expires_at < now() ORDER BY expires_at LIMIT $1)` | `DELETE FROM graph_snapshots WHERE expires_at < CURRENT_TIMESTAMP(6) ORDER BY expires_at LIMIT ?` (MySQL không cho `LIMIT` trong truy vấn con của `IN`, nên viết trực tiếp) |
| Xoá snapshot sai `schema_version` | cùng cách | cùng cách |
| Vượt hạn mức tenant | tìm tenant có `SUM(payload_bytes) > quota`, xoá snapshot cũ nhất đến khi dưới hạn | như vậy |
| Dữ liệu mồ côi (bảng `graph_snapshots`, `review_states`, `reindex_jobs` mà `repo_binding_id` không còn trong `repo_bindings`, quá 7 ngày tuổi, `CODEINTEL_ORPHAN_RETENTION=168h`) | `DELETE ... WHERE NOT EXISTS (SELECT 1 FROM codeintel.repo_bindings b WHERE b.id = x.repo_binding_id) AND created_at < now() - interval` | tương tự có `LIMIT` |
| Job mồ côi: `status IN ('queued','running')` và `updated_at` quá `CODEINTEL_REINDEX_STALE_AFTER` (45 phút) không có tiến độ | `Finish(failed, 'CODEINTEL_REINDEX_ORPHANED')`, `active_key=NULL`, sự kiện `orca.codeintel.reindex.finished` | như vậy |
| `repo_bindings` rảnh: `COALESCE(last_status_at, updated_at)` quá `CODEINTEL_BINDING_IDLE_RETENTION` (2160h = 90 ngày); yêu cầu của CR-CV-012 mục 2.6 (binding dạng id tổng hợp và project bị xoá không có sự kiện) | `DELETE` theo lô; bảng phụ thuộc dọn theo dòng mồ côi ở trên | như vậy |
| `outbox_events` đã publish quá 7 ngày; `processed_events` quá 7 ngày | `DELETE` theo lô | như vậy |

Postgres + `FORCE ROW LEVEL SECURITY`: công việc bảo trì cần thấy dữ liệu mọi tenant; thêm chính sách hẹp theo mẫu `reconcile_read` của `mcp-service` (`mcp-service/migrations/postgres/0002_authorization.up.sql` dòng 59 đến 64): `FOR SELECT` và `FOR DELETE` (và `FOR UPDATE` cho `reindex_jobs`) với `current_setting('app.maintenance', true) = 'on'` kèm điều kiện hàng (`expires_at < now()`, `published_at IS NOT NULL AND published_at < now() - interval '7 days'`, `status IN ('queued','running')`), không bao giờ `INSERT`. Nhiều bản sao service chạy song song là an toàn vì xoá idempotent; chưa dùng khoá tư vấn (`pg_try_advisory_lock`, `GET_LOCK`), có thể thêm nếu thấy tranh chấp.

`CODEINTEL_SNAPSHOT_TTL` mặc định 24 giờ; CR-CV-022 có thể đặt `expires_at` ngắn hơn theo từng view. Khi `codeintel.indexChanged` đến (CR-CV-024), `DeleteByBinding(bindingID, exceptCommit='')` xoá snapshot của binding đó ngay, không chờ hết hạn.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| `scope_key` thay vì `worktree_id` nullable trong khoá duy nhất | Khoá duy nhất giống nhau ở hai dialect; hỗ trợ cả worktree không có dòng `project.worktrees` (xem mục 1 điểm 1) |
| `c4_overrides`, `finding_dismissals` khoá theo `repo_id` | Sống sót khi worktree/binding bị xoá; ghi đè và bỏ qua mang tính của repo |
| `review_states` khoá theo `repo_binding_id`, không phải `worktree_id` | Id worktree nhiều dạng chuỗi; `repo_binding_id` là id nội bộ ổn định. Review gắn với một worktree cụ thể nên mất khi worktree bị xoá sau thời gian lưu mồ côi (7 ngày) |
| `reindex_jobs.active_key` UNIQUE nullable | Thực thi "một job mỗi repo" không cần chỉ mục từng phần (MySQL không hỗ trợ) |
| Snapshot lưu `jsonb`/`json`, không nén | Giữ README; `jsonb` đã nén TOAST. Nếu đo thấy lớn, chuyển sang `bytea`/`LONGBLOB` nén gzip (Q2) |
| Khoá lạc quan `version` thay `SELECT FOR UPDATE` | Giống nhau hai dialect; hai người review cùng lúc nhận `CODEINTEL_VERSION_CONFLICT` và hợp nhất phía client |
| Không FK, dọn bằng ứng dụng | Quy ước README v7 mục 6; `repo_bindings` là gốc của các bảng còn lại |
| Chỉ ghi `workspace_root`, không ghi bí mật | Đường dẫn không phải bí mật; không ghi DSN, token hay nội dung file |

## 4. Tiêu chí chấp nhận

- [ ] `0002_code_intel_core` up/down chạy sạch bằng golang-migrate trên Postgres và MySQL; up lại sau down không lỗi.
- [ ] Mỗi CHECK ở mục 2.1 từ chối giá trị sai (chèn trực tiếp từng giá trị lỗi; với MySQL < 8.0.16 CHECK bị bỏ qua nên kiểm ở domain, xem mục 6).
- [ ] Hai `Create` job đồng thời cùng `repo_binding_id`: đúng một thành công, bên kia nhận `CODEINTEL_REINDEX_IN_PROGRESS`; sau `Finish` tạo được job mới.
- [ ] `Upsert` binding hai lần cùng `(project_id, scope_key)` giữ nguyên `id`, tăng `version`; hai đường dẫn khác nhau cùng repo cho hai `scope_key` khác nhau.
- [ ] `Put` snapshot cùng khoá thay thế dòng cũ; `Get` không trả dòng hết hạn hoặc sai `schema_version`; `Put` vượt `CODEINTEL_SNAPSHOT_MAX_BYTES` trả `CODEINTEL_PAYLOAD_TOO_LARGE` và không ghi dòng nào.
- [ ] Payload chứa `\u0000`: Postgres ghi được (đã bỏ NUL, `truncated=true`); chuỗi UTF-8 hỏng bị từ chối ở cả hai dialect với cùng lỗi.
- [ ] `Save` review state với `version` cũ trả `CODEINTEL_VERSION_CONFLICT`; id lạ trả `CODEINTEL_NOT_FOUND`; 20 `Save` đồng thời cùng `version` chỉ một thành công.
- [ ] `DeleteBinding` rồi chạy bảo trì: `c4_overrides` và `finding_dismissals` vẫn còn; `graph_snapshots`, `review_states`, `reindex_jobs` của binding bị xoá sau 7 ngày (test dùng hạn rút ngắn).
- [ ] Bảo trì xoá snapshot hết hạn theo lô, vượt hạn mức tenant thì xoá cũ nhất trước; job `running` không có tiến độ quá hạn thành `failed` với `CODEINTEL_REINDEX_ORPHANED` và `active_key=NULL`.
- [ ] Tenant A không đọc, không cập nhật được dòng tenant B qua repository (cả hai dialect) và qua SQL trực tiếp trên Postgres với role không phải superuser (RLS); chính sách bảo trì chỉ hoạt động khi `app.maintenance='on'` và không cho `INSERT`.
- [ ] Test AST bảo đảm mọi phương thức xuất của repository Postgres dùng `withTenantTx`/`withMaintenanceTx`/`withRelayTx` (CR-CV-010).
- [ ] `go vet`, `make lint` xanh; không có tên file `helpers`, `utils`, `common`, `misc`.

## 5. Kiểm thử

- **Unit (không DB):** constructor domain (JSON hỏng, UTF-8 hỏng, vượt kích thước, tập giá trị); `CanTransition` của `ReindexJob`; chuẩn hoá `params_hash` (khoá JSON sắp xếp, số thực); sinh `scope_key` (đường dẫn có `/` cuối, Windows `C:\repo` so với `c:\repo`: chưa chốt quy tắc chữ hoa/thường, xem CR-CV-012).
- **Integration, hai dialect (`-tags=integration`, `common/testutil`):** toàn bộ mục 4; bộ kịch bản viết một lần trong `repository_contract_test.go` nhận interface repository và chạy lại cho Postgres và MySQL (mẫu CR-REQ-002 mục 5). Postgres chạy thêm bộ RLS với role `NOSUPERUSER NOBYPASSRLS` (tạo trong test, mẫu `mcp-service/README.md`).
- **Hợp đồng schema:** đọc `information_schema` ở cả hai DB, so tên bảng, tên cột, tính NULL với danh sách mong đợi.
- **Chưa chạy bất kỳ test nào ở thời điểm viết CR.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên cột `view`, `commit`, `at` trùng từ khoá không-dành-riêng ở MySQL/Postgres; chưa chạy migration để xác nhận không cần dấu trích. Nếu bị chặn, đổi tên thành `view_name`, `commit_sha`, `dismissed_at` ở toàn series.
- CHECK trên MySQL cần 8.0.16 trở lên (bản cũ phân tích rồi bỏ qua); chưa xác định phiên bản MySQL/TiDB mục tiêu, chưa kiểm chứng TiDB.
- `TIMESTAMP(6)` của MySQL có trần năm 2038; `expires_at` chỉ cộng giờ nên không ảnh hưởng, nhưng các migration hiện có (`outbox_events`) đã dùng cùng kiểu.
- Postgres `jsonb` sắp xếp lại khoá và gộp khoá trùng; `params_hash` và so sánh nội dung không dựa vào thứ tự khoá của dòng đã lưu.
- Chưa đo kích thước snapshot thật: tổng hạn mức 512 MiB/tenant và trần 8 MiB là mặc định đề xuất; số liệu đồ thị Orca (247 556 nút, 644 157 cạnh) cho thấy `overview` đã cắt có thể vài MiB, chưa đo (xem CR-CV-071).
- `UNIQUE (active_key)` ở MySQL: nhiều `NULL` được phép trong chỉ mục duy nhất của InnoDB; chưa chạy thử.
- Chưa kiểm chứng chi phí `NOT EXISTS` khi quét dữ liệu mồ côi trên bảng lớn; chạy theo lô nhỏ và có `LIMIT`.
- `repo_id` trong `c4_overrides` giả định `Repo.id` của `project-service` ổn định khi đổi `dev_server_id` (`RebindRepoDevServer` không đổi `id`; chưa kiểm chứng `AssignRepoToProject`, vốn đổi `project_id` của repo).

## 7. Câu hỏi mở

- **Q1.** Điều chỉnh README v7 mục 3.5: thêm `tenant_settings`; `repo_bindings` thêm `repo_id`, `scope_key`, `path_hash`, `index_scope`, `last_status`, `last_status_at`, `version`; `graph_snapshots` thêm `schema_version`; `review_states` dùng `repo_binding_id` thay `worktree_id`; `finding_dismissals` và `c4_overrides` thêm `repo_id` và khoá theo đó; `reindex_jobs` thêm `percent`, `requested_by`, `active_key`, `message`, `created_at`, `updated_at`, `version`; `processed_events` khoá `(tenant_id, event_id)`. Cần cập nhật README.
- **Q2.** Có chấp nhận lưu snapshot dạng `jsonb`/`json` không nén ở MVP, hay đi thẳng sang `bytea`/`LONGBLOB` gzip? Mặc định: JSON, đo lại ở CR-CV-071.
- **Q3.** `reading_progress` dùng chung cả nhóm hay riêng từng người? Mặc định theo README (một dòng, `updated_by`), gây xung đột `version` khi hai người bấm "đã xem"; nếu cần riêng người, khoá thêm `user_id` vào `review_states`.
- **Q4.** `status` của `review_states` và tập `mode` của `reindex_jobs` do CR-CV-052/060 và CR-CV-004 chốt; CR này để kiểm ở domain.
- **Q5.** Thời gian lưu mồ côi 7 ngày và TTL snapshot 24 giờ là mặc định đề xuất, chưa có số liệu sử dụng.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` mục 3.5, 3.8, 6
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-002-request-data-model-and-repositories.md` (mẫu cấu trúc, CAS, keyset)
- `/opt/repos/orca/backend-go/services/mcp-service/migrations/postgres/0001_init.up.sql`, `0002_authorization.up.sql` (RLS FORCE, chính sách `app.relay`, `reconcile_read`), `internal/adapter/postgres/tenant_tx.go`, `repository.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql`, `0014_task_sources_site.up.sql`, `internal/adapter/postgres/task_sources.go`
- `/opt/repos/orca/backend-go/services/project-service/internal/usecase/lifecycle_events.go` (`orca.project.worktree.deleted`)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` (dạng id worktree), `/opt/repos/orca/frontend/src/shared/worktree-id.ts`
- `/opt/repos/orca/backend-go/common/{apperrors/apperrors.go,dbcapability/capability.go,tenant/tenant.go,testutil}`
- `/opt/repos/orca/docs/research/view-code/05-graph-schemas.md` mục 2.8, 3 (`IndexStatus`, `SymbolRef`), `06-gaps-risks-roadmap.md` mục 2
- CR liên quan: [CR-CV-010](./CR-CV-010-scaffold-code-intel-service.md), [CR-CV-012](./CR-CV-012-project-worktree-to-repo-binding.md), [CR-CV-013](./CR-CV-013-authorization-audit-and-quotas.md)
