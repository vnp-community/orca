# BE-CV-TASK-011-07: Repository Postgres: settings, binding, review state, dismissal, C4, processed events

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/{tenant_settings,repo_binding,review_state,finding_dismissal,c4_override,processed_event}_repository.go` (+ `_integration_test.go`) (mới)
**Depends on:** BE-CV-TASK-011-02, 011-04, 011-06
**Status:** [ ] TODO

---

## Context

Mẫu: `mcp-service/internal/adapter/postgres/repository.go` và `tenant_tx.go`. Mọi phương thức chạy trong `withTenantTx(ctx, tenantID, fn)` và vẫn có `tenant_id = $n` ở `WHERE`. Vi phạm duy nhất: `*pgconn.PgError` `23505` (như `task-service/.../task_sources.go:14`). Tiền tố bảng `codeintel.`.

## Việc cần làm

1. `tenant_settings`: `Get`, `GetOrCreate` (`INSERT … ON CONFLICT (tenant_id) DO NOTHING` rồi `SELECT`), `Update`.
2. `repo_binding`: `Upsert` (`INSERT … ON CONFLICT (tenant_id, project_id, scope_key) DO UPDATE SET … version = repo_bindings.version + 1, updated_at = now() RETURNING …`; giữ `id`, `created_at`); `Get`, `GetByScope`, `ListByProject` (`LIMIT`), `ListByPath`, `ListRepoRootDependents`, `SaveStatusCache`, `Delete`, `DeleteByWorktreeID`.
3. `review_state`: `Get`; `Save` (`expectedVersion=0` → `INSERT`, vi phạm duy nhất → `CODEINTEL_VERSION_CONFLICT`; `>0` → CAS `UPDATE … WHERE version=$n`, 0 hàng thì `SELECT` phân biệt `NOT_FOUND`/`VERSION_CONFLICT`); ghi `events` (`orca.codeintel.review.saved`) cùng tx; hỗ trợ dòng mức worktree (hai commit rỗng).
4. `finding_dismissal`: `Dismiss` (`ON CONFLICT (tenant_id, repo_id, finding_key) DO UPDATE` cập nhật `disposition/reason/note/dismissed_by/at` — idempotent), `Restore` (`DELETE`, 0 hàng không lỗi), `ListByRepo`.
5. `c4_override`: `Get`, `ListByRepo`, `Save` (CAS như review), `Delete`.
6. `processed_event`: `MarkProcessed` (`INSERT … ON CONFLICT DO NOTHING`, `RowsAffected()==1` → `firstTime`).
7. Chuẩn hoá lỗi: `pgx.ErrNoRows` → `CODEINTEL_NOT_FOUND`; `23505` → mã theo ràng buộc; payload vượt giới hạn do domain đã chặn trước (không dựa lỗi DB).
8. Không log nội dung `notes`, `document`, `finding_key` dài; chỉ id và kích thước.

## Kiểm thử

- Integration (`-tags=integration`, Postgres, `common/testutil.StartPostgres`): từng phương thức (đường vui + lỗi); CAS 20 goroutine; xoá binding rồi dismissal/C4 còn.
- Test AST (010-06) xanh với các file mới.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/postgres/... -run 'TenantSettings|RepoBinding|ReviewState|Dismissal|C4|Processed'`.

## Tiêu chí hoàn thành

- [ ] Mọi phương thức qua `withTenantTx`, có `tenant_id` ở `WHERE`.
- [ ] CAS và idempotency đúng (xem tiêu chí SOL mục 4).
- [ ] Sự kiện outbox chỉ tồn tại khi commit.

## Rủi ro và lưu ý

- `jsonb` sắp xếp lại khoá: không so sánh nội dung theo thứ tự khoá.
- `RETURNING` dùng được ở Postgres; bản MySQL (011-09) không có.
