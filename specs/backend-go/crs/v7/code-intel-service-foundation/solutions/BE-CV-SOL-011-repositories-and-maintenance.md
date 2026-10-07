# BE-CV-SOL-011-repositories-and-maintenance: Cổng repository hai dialect, ghi outbox cùng transaction và công việc bảo trì

> **📋 Proposed.** Chưa chạy build/test nào. Phần thứ hai của CR-CV-011; cần schema/domain của [`BE-CV-SOL-011-data-model-and-migrations`](./BE-CV-SOL-011-data-model-and-migrations.md).

**CR:** [CR-CV-011](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md) (mục 2.4, 2.5)
**Service:** `code-intel-service` (`internal/usecase`, `internal/adapter/{postgres,mysql}`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (cổng ở usecase, adapter hiện thực), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (mục "Multi-tenancy", "Cross-service data consistency"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "Event conventions": outbox, at-least-once), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (mục "Resilience patterns")

---

## Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| §4.1 | `withTenantTx` mọi phương thức; MySQL `WHERE tenant_id = ?`; đồng hồ DB; CAS `version`; mã vi phạm duy nhất PG `23505`, MySQL `1062` |
| §4.2 T1–T7 | Hình dạng dữ liệu mà repository đọc/ghi |
| §4.3 | Bảo trì mỗi `CODEINTEL_MAINTENANCE_INTERVAL` (10 phút), lô 500, `withMaintenanceTx` |
| §5 | Sự kiện outbox do repository ghi: `orca.codeintel.reindex.started|finished`, `orca.codeintel.review.saved`; payload `snake_case` (bảng §5) |
| PQ-03 | Mã lỗi `CODEINTEL_NOT_FOUND`, `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_REINDEX_IN_PROGRESS`, `CODEINTEL_ALREADY_EXISTS`, `CODEINTEL_PAYLOAD_TOO_LARGE` |
| PQ-05 | `Dismiss` idempotent theo `(repo_id, finding_key)`; `RESTORE` = xoá dòng |
| PQ-14 (2)(3) | Snapshot: trần theo cấu hình, TTL 7 ngày, giữ 3 commit/`(binding, view)`, 64 MiB/binding, 512 MiB/tenant; không bao giờ xoá snapshot mới nhất mỗi view |
| PQ-15 | `Put` upsert theo UNIQUE `(…, view, head_commit, params_hash)` (PG `ON CONFLICT … DO UPDATE`; MySQL `ON DUPLICATE KEY UPDATE`) |
| PQ-16 | `Finish` đặt `active_key = NULL`; orphan → `failed` + `CODEINTEL_REINDEX_ORPHANED` |
| PQ-22 | `ReviewStateRepository.Save` hỗ trợ dòng mức worktree (hai commit rỗng) |
| H10 | Test cô lập tenant mọi phương thức (CR-072) |

## Lệch giữa CR và hợp đồng

| # | CR-CV-011 nói | Hợp đồng | Xử lý |
|---|---|---|---|
| L1 | `SnapshotRepository.Put` chỉ thay thế cùng khoá | PQ-14: giữ 3 commit/`(binding, view)`, 64 MiB/binding | `Put` cắt tỉa trong cùng transaction; mới nhất mỗi view không bao giờ bị xoá |
| L2 | Hạn mức tenant 512 MiB, mặc định TTL 24 h | TTL 7 ngày (config SOL-010) | Theo cấu hình |
| L3 | `Create` job với `requested_by NOT NULL` | T7: `requested_by` NULL = hệ thống | Port nhận `*string`/rỗng = NULL |
| L4 | `UpdateProgress(ctx, id, stage, percent, now)` percent int | PQ-16: `percent` có thể `null` | Tham số `*int` |
| L5 | Bảo trì xoá toàn bộ bảng theo từng nhóm tại chỗ | Bảo trì cần chạm thêm bảng của CR sau (§4.3 liệt kê `quality_*`, `agent_turns`, `coverage_reports`) | Khung `RetentionTask` đăng ký được; CR chủ sở hữu tự thêm tác vụ của mình (SOL-082/083/085/089) |
| L6 | `CountActiveByDevServer` "thêm vào CR-CV-011 khi triển khai" (nêu ở CR-CV-013) | Hợp đồng không nêu | Thêm vào `ReindexJobRepository` ở solution này (cho SOL-013) |
| L7 | Mã `CODEINTEL_INVALID_ARGUMENT` | PQ-03 | `CODEINTEL_INVALID_PARAMS` |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-011-data-model-and-migrations` | Trước: schema và domain |
| BE | `BE-CV-SOL-010-scaffold-code-intel-service` | Trước: `withTenantTx`, `insertOutboxEvents`, AST guard |
| BE | `BE-CV-SOL-012-target-resolution-and-bindings` | Sau: dùng `RepoBindingRepository`, `ProcessedEventRepository` |
| BE | `BE-CV-SOL-013-agent-call-gate-and-quotas` | Sau: `ReindexJobRepository.CountActive*` |
| BE | `BE-CV-SOL-022-snapshot-cache`, `-024-event-distribution`, `-033-c4-*`, `-037-*`, `-052/060 (FE)`, `-082`, `-085` | Sau: dùng repository tương ứng; `RetentionTask` cho bảng của họ |
| AG / FE | — | Không có |

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `mcp-service/internal/adapter/postgres/{repository.go,tenant_tx.go,tenant_scope_guard_test.go}` (mẫu `withTenantTx`, `withRelayTx`, `FetchUnpublished` dòng 105–135), `task-service/internal/adapter/{postgres,mysql}/task_sources.go` (mã vi phạm duy nhất), `notification-service/internal/adapter/mysql/` (danh sách file: `repository.go`, `buffered_notification_repository.go`, … mẫu MySQL), `common/{outbox/outbox.go,dbcapability/capability.go,tenant/tenant.go,testutil}`, hợp đồng §4.3.

Xác nhận: `mcp-service` ghi sự kiện trong cùng transaction nghiệp vụ (dòng `INSERT INTO mcp.outbox_events` trong phương thức repository); `dbcapability` ghi `SupportsReturning=false` cho MySQL theo CR (chưa đọc lại giá trị trường này ở lần soạn này: **chưa kiểm chứng**); `MaintenanceRepository`/`app.maintenance`: chưa có ở mã hiện có (grep ra rỗng) nên toàn bộ là phần mới.

### Correction relative to CR-CV-011

| # | CR nói | Mã thật / hợp đồng | Xử lý |
|---|---|---|---|
| C1 | Mẫu `reconcile_read` làm khuôn bảo trì | Mẫu dùng cờ `app.relay` và chỉ `SELECT`; không có `DELETE` hay cờ `app.maintenance` ở bất kỳ file `.sql`/`.go` nào (grep) | Thêm `withMaintenanceTx` (cờ `app.maintenance`); chính sách hẹp theo SOL-011-data D3 |
| C2 | MySQL `DELETE … ORDER BY … LIMIT ?` | Cú pháp MySQL hợp lệ cho `DELETE` một bảng; không dùng cho đa bảng | Giữ; Postgres dùng `WHERE id IN (SELECT … LIMIT)` |
| C3 | Phát hiện nhầm ràng buộc duy nhất cùng bảng (`reindex_jobs`) | Tên ràng buộc đặt tường minh ở migration (`uq_reindex_active_key`) | Adapter đọc `PgError.ConstraintName` / chuỗi `Message` MySQL |
| C4 | `ListByProject`, `ListByRepo` giới hạn 500 | Hợp đồng không đổi | Giữ |

## 2. Giải pháp

### A. Cổng (`internal/usecase/ports.go`, mở rộng)

```go
type TenantSettingsRepository interface {
    Get(ctx context.Context) (domain.TenantSettings, error)                        // CODEINTEL_NOT_FOUND nếu chưa có
    GetOrCreate(ctx context.Context, defaults domain.TenantSettings) (domain.TenantSettings, error)
    Update(ctx context.Context, s domain.TenantSettings) error                      // SOL-073 gọi
}
type RepoBindingRepository interface {
    Upsert(ctx context.Context, b domain.RepoBinding) (domain.RepoBinding, error)   // theo (project_id, scope_key); giữ id cũ, tăng version
    Get(ctx context.Context, id string) (domain.RepoBinding, error)
    GetByScope(ctx context.Context, projectID, scopeKey string) (domain.RepoBinding, error)
    ListByProject(ctx context.Context, projectID string, limit int) ([]domain.RepoBinding, error)   // ≤ 500
    ListByPath(ctx context.Context, devServerID, pathHash string) ([]domain.RepoBinding, error)
    ListRepoRootDependents(ctx context.Context, repoID string) ([]domain.RepoBinding, error)
    SaveStatusCache(ctx context.Context, id string, statusJSON []byte, at time.Time) error
    Delete(ctx context.Context, id string) error
    DeleteByWorktreeID(ctx context.Context, projectID, worktreeID string) (int, error)
}
type SnapshotRepository interface {
    Put(ctx context.Context, s domain.GraphSnapshot, limits SnapshotLimits) error
    Get(ctx context.Context, k domain.SnapshotKey, schemaVersion int) (domain.GraphSnapshot, error)   // chưa hết hạn (đồng hồ DB) và đúng schema_version
    DeleteByBinding(ctx context.Context, bindingID, exceptHeadCommit string) error
    TenantBytes(ctx context.Context) (int64, error)
}
type ReviewStateRepository interface {
    Get(ctx context.Context, bindingID, base, head string) (domain.ReviewState, error)
    Save(ctx context.Context, s domain.ReviewState, expectedVersion int64, events []domain.OutboxRecord) (domain.ReviewState, error)   // 0 = tạo
}
type FindingDismissalRepository interface {
    Dismiss(ctx context.Context, d domain.FindingDismissal) error      // idempotent
    Restore(ctx context.Context, repoID, findingKey string) error      // xoá dòng; không lỗi khi không có
    ListByRepo(ctx context.Context, repoID string, limit int) ([]domain.FindingDismissal, error)
}
type C4OverrideRepository interface {
    Get(ctx context.Context, repoID, container string) (domain.C4Override, error)
    ListByRepo(ctx context.Context, repoID string, limit int) ([]domain.C4Override, error)
    Save(ctx context.Context, o domain.C4Override, expectedVersion int64, events []domain.OutboxRecord) (domain.C4Override, error)
    Delete(ctx context.Context, repoID, container string) error
}
type ReindexJobRepository interface {
    Create(ctx context.Context, j domain.ReindexJob, events []domain.OutboxRecord) error   // vi phạm active_key → CODEINTEL_REINDEX_IN_PROGRESS
    Get(ctx context.Context, id string) (domain.ReindexJob, error)
    ListByBinding(ctx context.Context, bindingID string, limit int) ([]domain.ReindexJob, error)   // ≤ 100
    UpdateProgress(ctx context.Context, id, stage string, percent *int) error
    Finish(ctx context.Context, id string, status domain.ReindexStatus, outcome, errorCode, message string, events []domain.OutboxRecord) error
    CountActiveByDevServer(ctx context.Context, devServerID string) (int, error)   // nối repo_bindings
    CountActiveByTenant(ctx context.Context) (int, error)
    LastSucceededFinishedAt(ctx context.Context, bindingID string) (time.Time, bool, error)   // cho cooldown (SOL-013)
}
type ProcessedEventRepository interface {   // dedup consumer (SOL-012 worktree.deleted)
    MarkProcessed(ctx context.Context, eventID, subject string) (firstTime bool, err error)
}
```

`SnapshotLimits{MaxPayloadBytes, KeepCommits=3, BindingQuotaBytes}`. Mọi phương thức lấy tenant bằng `tenant.RequireTenantID(ctx)`; `tenant_id` luôn có trong `WHERE`, kể cả Postgres có RLS (dev dùng superuser nên RLS không chạy).

### B. Hành vi theo dialect

| Chủ đề | Postgres | MySQL |
|---|---|---|
| Thời gian | `now()`; `now() + make_interval(secs => $n)` | `CURRENT_TIMESTAMP(6)`; `DATE_ADD(CURRENT_TIMESTAMP(6), INTERVAL ? SECOND)` |
| CAS | `UPDATE … SET …, version = version + 1, updated_at = now() WHERE id=$ AND tenant_id=$ AND version=$`; 0 hàng → `SELECT` phân biệt `NOT_FOUND`/`VERSION_CONFLICT` | cùng; `RowsAffected` an toàn vì `version` luôn đổi |
| Upsert | `INSERT … ON CONFLICT (…) DO UPDATE … RETURNING` | `INSERT … ON DUPLICATE KEY UPDATE …` rồi `SELECT` lại trong cùng transaction |
| Vi phạm duy nhất | `*pgconn.PgError` `23505` + `ConstraintName` | `*mysql.MySQLError` `1062` + chuỗi `Message` chứa tên khoá |
| Transaction | `withTenantTx` (`set_config('app.tenant_id', $1, true)`) | `*sql.Tx` + `tenant.RequireTenantID` |
| Ghi outbox | `insertOutboxEvents(ctx, tx, tenantID, events)` trong cùng tx (SOL-010) | cùng |
| Giới hạn danh sách | `LIMIT` bắt buộc | cùng |
| Tỉa snapshot | cùng tx với `Put`: giữ 3 `head_commit` mới nhất của `(binding, view)`; vượt `BindingQuotaBytes` thì xoá cũ nhất, **không** xoá bản vừa ghi | MySQL không cho `LIMIT` trong subquery của `IN` → dùng bảng dẫn xuất `JOIN` |

### C. Bảo trì (`internal/usecase/snapshot_maintenance.go` (mới) và khung `RetentionTask`)

```go
type RetentionTask interface {
    Name() string
    Run(ctx context.Context, now time.Time, batch int) (deleted int, err error)  // idempotent; không giữ khoá lâu
}
type MaintenanceRepository interface {      // chạy ngoài tenant, qua withMaintenanceTx (Postgres)
    ListTenantsWithData(ctx context.Context) ([]string, error)
    DeleteExpiredSnapshots(ctx context.Context, batch int) (int, error)           // xuyên tenant, điều kiện hàng expires_at < now
    DeletePublishedOutbox(ctx context.Context, olderThan time.Time, batch int) (int, error)
    DeleteOldProcessedEvents(ctx context.Context, olderThan time.Time, batch int) (int, error)
}
// Các việc theo tenant chạy trong withTenantTx của chính tenant đó (xem mục 3 D3):
type TenantMaintenanceRepository interface {
    DeleteWrongSchemaSnapshots(ctx context.Context, current func(view string) int, batch int) (int, error)
    EvictOldestSnapshotsOverQuota(ctx context.Context, quotaBytes int64, batch int) (int, error)
    DeleteOrphans(ctx context.Context, olderThan time.Time, batch int) (int, error)   // snapshot/review/reindex không còn binding
    FailOrphanedReindexJobs(ctx context.Context, staleBefore time.Time, batch int) ([]domain.ReindexJob, error)
    DeleteIdleBindings(ctx context.Context, idleBefore time.Time, batch int) (int, error)
}
```

Mỗi vòng (mặc định 10 phút; goroutine do `main.go` khởi, tắt theo `ctx`): xoá snapshot hết hạn (lô 500) → liệt kê tenant → với mỗi tenant: sai `schema_version`, vượt hạn mức (xoá cũ nhất, không xoá mới nhất mỗi view), mồ côi (`CODEINTEL_ORPHAN_RETENTION`), job mồ côi (`failed` + `error_code='CODEINTEL_REINDEX_ORPHANED'`, `active_key=NULL`, phát `orca.codeintel.reindex.finished`), binding rảnh (`COALESCE(last_status_at, updated_at)` quá `CODEINTEL_BINDING_IDLE_RETENTION`) → outbox đã publish và `processed_events` quá 7 ngày. Các `RetentionTask` đăng ký thêm chạy cuối vòng (quality 30/180/90 ngày… do SOL của CR chủ sở hữu).

`snapshot_schema_versions`: hàm `func(view string) int` tiêm từ SOL-022 (mặc định 1) để SOL-011 không biết tập view.

Nhiều bản sao chạy song song an toàn vì mọi xoá idempotent; chưa dùng khoá tư vấn (`pg_try_advisory_lock`, `GET_LOCK`), thêm nếu thấy tranh chấp.

### D. Cây file (mới)

```
internal/usecase/ports.go                      # (mở rộng từ SOL-010)
internal/usecase/snapshot_maintenance.go
internal/usecase/retention_task.go
internal/adapter/postgres/{tenant_settings,repo_binding,graph_snapshot,review_state,finding_dismissal,c4_override,reindex_job,processed_event,maintenance}_repository.go  (+ _test, _integration_test)
internal/adapter/postgres/maintenance_tx.go    # withMaintenanceTx
internal/adapter/mysql/(cùng tên bảng)_repository.go
internal/adapter/contracttest/repository_contract.go   # bộ kịch bản dùng chung hai dialect (gói test, tên cụ thể)
```

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Khoá lạc quan `version` thay `SELECT FOR UPDATE` | Giống nhau hai dialect; hai người review cùng lúc nhận `CODEINTEL_VERSION_CONFLICT` |
| D2 | Ghi sự kiện qua tham số `events` | Một transaction cho thay đổi + sự kiện (SOL-010) |
| D3 | Việc bảo trì có mốc theo cấu hình chạy **theo từng tenant** trong `withTenantTx`; chỉ việc xuyên tenant có điều kiện hàng không phụ thuộc cấu hình dùng `withMaintenanceTx` | Chính sách DB hẹp hơn; retention đổi bằng env không cần migration |
| D4 | `RetentionTask` đăng ký | §4.3 liệt kê bảng của CR sau; tránh sửa file bảo trì mỗi CR |
| D5 | `Put` tỉa trong cùng transaction | Giữ bất biến "mới nhất mỗi view tồn tại" nguyên tử |
| D6 | `ProcessedEventRepository.MarkProcessed` trả `firstTime` | Consumer idempotent (§5) |
| D7 | Danh sách luôn có `LIMIT` | Không trả tập không giới hạn |

## 4. Tiêu chí chấp nhận

- [x] `Upsert` binding hai lần cùng `(project_id, scope_key)` giữ `id`, tăng `version`; hai đường dẫn khác nhau cùng repo → hai `scope_key`.
- [x] Hai `Create` job đồng thời cùng binding: đúng một thành công, bên kia `CODEINTEL_REINDEX_IN_PROGRESS`; sau `Finish` tạo được job mới.
- [x] `Put` snapshot cùng khoá thay thế; `Get` không trả dòng hết hạn/sai `schema_version`; vượt trần `CODEINTEL_PAYLOAD_TOO_LARGE`, không ghi dòng nào; cắt tỉa giữ 3 commit và bản mới nhất; payload có `\u0000` ghi được (Postgres).
- [x] `Save` review state: `version` cũ → `VERSION_CONFLICT`; id lạ → `NOT_FOUND`; 20 `Save` đồng thời cùng `version`: đúng một thành công; sự kiện `review.saved` chỉ có khi commit.
- [x] `Dismiss` idempotent; `Restore` xoá dòng; `Save` C4 CAS.
- [x] Xoá binding rồi bảo trì: `c4_overrides`, `finding_dismissals` còn; `graph_snapshots`, `review_states`, `reindex_jobs` bị xoá sau hạn (test rút ngắn).
- [x] Bảo trì: snapshot hết hạn theo lô; vượt hạn mức → xoá cũ nhất; job `running` quá hạn → `failed` + `CODEINTEL_REINDEX_ORPHANED`, `active_key = NULL`, có sự kiện `reindex.finished`.
- [x] Mọi phương thức: tenant A không đọc/ghi được dòng tenant B (hai dialect + SQL trực tiếp Postgres với role `NOBYPASSRLS`).
- [x] Test AST bắt phương thức thiếu phạm vi tenant, quét mọi `*_repository.go`.
- [x] `go vet`, `make lint` xanh.

## 5. Kiểm thử

- **Unit:** logic cắt tỉa (hàm thuần chọn dòng cần xoá); đăng ký `RetentionTask`; thứ tự các bước vòng bảo trì với repository giả; xử lý lỗi một tác vụ không chặn tác vụ sau.
- **Integration hai dialect (`-tags=integration`):** bộ `repository_contract.go` viết một lần nhận interface repository, chạy cho Postgres và MySQL (mẫu CR-REQ-002 mục 5); Postgres thêm bộ RLS role không superuser; bộ bảo trì với đồng hồ rút ngắn.
- **Đồng thời:** 20 goroutine `Save`/`Create` cùng khoá.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- `dbcapability` `SupportsReturning` của MySQL chưa đọc lại; thiết kế không phụ thuộc `RETURNING` ở MySQL (dùng `SELECT` lại).
- Chi phí `NOT EXISTS` khi quét mồ côi trên bảng lớn chưa đo; chạy theo lô nhỏ có `LIMIT`.
- Tỉa snapshot và hạn mức binding (64 MiB) là đề xuất của hợp đồng, chưa có số liệu dùng thật.
- `repo_id` trong `c4_overrides`/`finding_dismissals` giả định ổn định qua `RebindRepoDevServer`; `AssignRepoToProject` (đổi `project_id`) chưa kiểm.
- Nhiều bản sao × `rate`/bảo trì: an toàn nhưng tốn tài nguyên trùng lặp.

## 7. Câu hỏi mở

- **Q1.** Có cần khoá tư vấn để chỉ một bản sao chạy bảo trì? Mặc định: không (idempotent).
- **Q2.** `ListRepoRootDependents` dùng ở đâu (CR-012 nêu, SOL-012 quyết định gọi): giữ trong cổng.
- **Q3.** `tenant_settings` không có `version`: `Update` last-write-wins; SOL-073 xác nhận.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` §4.1–4.3, §5, PQ-03, PQ-05, PQ-14–16, PQ-22
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/{repository.go,tenant_tx.go,tenant_scope_guard_test.go}`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/{postgres,mysql}/task_sources.go`
- `/opt/repos/orca/specs/backend-go/crs/v6/request-service-foundation/solutions/BE-REQ-SOL-002-request-data-model-and-repositories.md` (mẫu repository hai dialect)
