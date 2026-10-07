# BE-CV-TASK-011-11: Bộ test hợp đồng repository dùng chung hai dialect và cô lập tenant

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/contracttest/repository_contract.go` (mới, gói kiểm thử), `internal/adapter/postgres/repository_contract_integration_test.go`, `internal/adapter/mysql/repository_contract_integration_test.go`, `internal/adapter/postgres/rls_isolation_integration_test.go` (mới)
**Depends on:** BE-CV-TASK-011-07, 011-08, 011-09, 011-10
**Status:** [x] DONE

---

## Context

Mẫu: CR-REQ-002 mục 5 (bộ kịch bản viết một lần nhận interface repository). Hợp đồng H10 và §8.3 điểm 3–4: mọi nhánh hai dialect có test hai dialect; mọi truy vấn có `tenant_id` + test cô lập tenant. Tên gói `contracttest` (cụ thể, không `helpers`).

## Việc cần làm

1. `repository_contract.go`: hàm `RunAll(t *testing.T, f Factory)` với `Factory{Settings, Bindings, Snapshots, Reviews, Dismissals, C4, Jobs, Processed; WithTenant func(ctx, id) context.Context; Reset func(t)}`; mỗi bảng một nhóm `t.Run`.
2. Kịch bản (đúng tiêu chí SOL mục 4): upsert giữ `id`; job đồng thời; snapshot thay thế/tỉa/quota/hết hạn/trần/NUL; review CAS 20 goroutine; dismissal idempotent/restore; C4 CAS; settings `GetOrCreate` không ghi đè; `MarkProcessed` lần hai `firstTime=false`.
3. **Cô lập tenant** (CR-072): với mỗi phương thức đọc/ghi, tạo dữ liệu ở tenant A rồi gọi bằng ctx tenant B với id của A → `CODEINTEL_NOT_FOUND` / danh sách rỗng / `DeleteByWorktreeID` = 0; `Upsert` cùng `(project_id, scope_key)` ở hai tenant tạo hai dòng độc lập.
4. Postgres RLS: kết nối role `NOSUPERUSER NOBYPASSRLS` (tạo trong test), SQL trực tiếp `SELECT/UPDATE/DELETE` trên mỗi bảng với `app.tenant_id` của B không chạm dòng của A; không đặt `app.tenant_id` → 0 dòng, không lỗi cast.
5. Ghi nhận: nếu dialect MySQL không hỗ trợ CHECK ở phiên bản test, đánh dấu `t.Skip` có lý do cho riêng case CHECK (domain đã chặn).
6. Kết nối: dùng `common/testutil.StartPostgres(t, "codeintel")`, `StartMySQL(t, "codeintel")`, chạy migration `0001`+`0002` bằng `golang-migrate` trước.

## Kiểm thử

- `go test -tags=integration ./services/code-intel-service/internal/adapter/postgres/... -run Contract`
- `go test -tags=integration ./services/code-intel-service/internal/adapter/mysql/... -run Contract`
- CI: ma trận `dialect: [postgres, mysql]` của workflow service.

## Tiêu chí hoàn thành

- [x] Cùng một bộ kịch bản xanh ở cả hai dialect.
- [x] Mọi phương thức có ít nhất một test cô lập tenant.
- [x] RLS: tenant B không đọc/ghi được dòng tenant A bằng SQL trực tiếp (Postgres, role không superuser).

## Rủi ro và lưu ý

- Dev compose dùng superuser: test phải tự tạo role.
- Bộ chạy hai DB tăng thời gian CI; dùng một container mỗi dialect cho cả gói.
