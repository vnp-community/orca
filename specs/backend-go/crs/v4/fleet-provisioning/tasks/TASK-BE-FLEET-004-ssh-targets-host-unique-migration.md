# TASK-BE-FLEET-004: Migration `0017_ssh_targets_host_unique` — idempotency constraint

**Solution:** BE-FLEET-SOL-001 | **CR:** CR-FLEET-001
**Service:** `infra-fleet-service`
**Depends on:** Không (độc lập — có thể chạy song song TASK-BE-FLEET-001/002)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** verify lại `ls migrations/ | sort -t_ -k1 -n | tail -5`
> ngay trước khi tạo file — `0016_ephemeral_vm_ssh_target_host_key` vẫn là mới
> nhất, KHÔNG lệch so với ghi chú trong task → dùng đúng số `0017`. Tạo
> `0017_ssh_targets_host_unique.up.sql`/`.down.sql` đúng nội dung đề xuất.
> `provisionOne`'s `isUniqueViolation` handling **chưa áp dụng ở đây** vì
> `bulk_provision_fleet.go` (TASK-BE-FLEET-001) chưa tồn tại tại thời điểm
> task này chạy (đúng thứ tự Wave 1 trước Wave 2 trong README) — logic đó
> được đưa thẳng vào lúc implement TASK-BE-FLEET-001 (xem file trạng thái của
> task đó), tránh viết 2 lần.
>
> **Test thật đã chạy** (integration, testcontainers postgres:16-alpine —
> Docker sẵn có trong môi trường này):
> `go test -tags=integration ./internal/adapter/postgres/... -run
> TestMigration0017 -v -count=1` → 2/2 PASS
> (`TestMigration0017_UniqueConstraintRejectsSameTenantHostDuplicate`,
> `TestMigration0017_AllowsSameHostDifferentTenant`). Lần chạy đầu tiên có 1
> lần FAIL do flake tiền tồn tại của testcontainers ("database system is
> starting up" — container báo cổng 5432 sẵn sàng trước khi Postgres phục hồi
> xong), KHÔNG liên quan tới SQL của migration này — chạy lại `-count=1` pass
> sạch 2/2, và test `TestRepository_Delete_*` (TASK-BE-FLEET-002) vẫn PASS sau
> khi có thêm migration 0017 (không phá test cũ).

---

## Mục tiêu

Thêm unique constraint `(tenant_id, host)` trên `infra.ssh_targets` để chặn `BulkProvisionFleet` (TASK-BE-FLEET-001)
tạo `SshTarget` trùng lặp khi chạy lại cùng 1 `FleetSpec`. Đây là hướng **DB constraint**, không phải
`FindByHost` ở tầng usecase — xem BE-FLEET-SOL-001 §5 cho lý do chọn hướng này (an toàn hơn dưới concurrency
của N goroutine song song trong `BulkProvisionFleet`).

## ⚠️ Bắt buộc verify số thứ tự migration THẬT trước khi tạo file

```bash
ls backend-go/services/infra-fleet-service/migrations/ | sort -t_ -k1 -n | tail -5
```

Tại thời điểm viết solution (2026-09-09), file mới nhất là `0016_ephemeral_vm_ssh_target_host_key.up/down.sql`
— nghĩa là `0017` là số đúng. **Chạy lại lệnh trên ngay trước khi tạo file** — nếu đã có `0017_*` khác (từ 1
CR/task song song khác đã merge trước), dùng số tiếp theo thật, KHÔNG ghi đè file đã có.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/migrations/0017_ssh_targets_host_unique.up.sql` (MỚI — hoặc số đúng sau khi verify lại)
2. `backend-go/services/infra-fleet-service/migrations/0017_ssh_targets_host_unique.down.sql` (MỚI)
3. `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go` (MODIFY — xử lý `unique_violation`, nếu TASK-BE-FLEET-001 đã merge trước; nếu chưa, ghi chú lại cho TASK-BE-FLEET-001 xử lý)

## Nội dung migration

```sql
-- 0017_ssh_targets_host_unique.up.sql
ALTER TABLE infra.ssh_targets ADD CONSTRAINT ssh_targets_tenant_host_unique UNIQUE (tenant_id, host);
```

```sql
-- 0017_ssh_targets_host_unique.down.sql
ALTER TABLE infra.ssh_targets DROP CONSTRAINT ssh_targets_tenant_host_unique;
```

**Trước khi áp dụng `up.sql` lên 1 DB đã có dữ liệu:** kiểm tra không có row trùng `(tenant_id, host)` đã tồn
tại từ trước (constraint sẽ fail migration nếu có). Nếu môi trường test/CI seed dữ liệu test có host trùng
lặp giữa các tenant khác nhau, xác nhận `tenant_id` khác nhau đủ để pass — chỉ trùng cấm khi CÙNG tenant.

## Xử lý `unique_violation` ở usecase layer

`provisionOne` (trong `bulk_provision_fleet.go`, TASK-BE-FLEET-001) coi Postgres error code `23505` như
"server này đã tồn tại", không phải fail cứng:

```go
import "github.com/jackc/pgx/v5/pgconn" // driver thật đã verify — infra-fleet-service dùng jackc/pgx/v5
                                          // (go.mod dòng 8; adapter/postgres/dev_server_group_grant_repository.go
                                          // import trực tiếp pgx/v5, pgx/v5/pgxpool), KHÔNG phải lib/pq

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" // unique_violation
}

func (uc *BulkProvisionFleet) provisionOne(ctx context.Context, server FleetSpecServer) BulkProvisionServerResult {
	// ...code hiện có (TASK-BE-FLEET-001)...
	sshTarget, err := uc.createSshTarget.Execute(ctx, CreateSshTargetInput{...})
	if err != nil {
		if isUniqueViolation(err) {
			return BulkProvisionServerResult{Host: server.Host, Status: "SUCCEEDED", Error: "already_exists"}
		}
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}
	// ...
}
```

**Unwrap qua `apperrors`:** lỗi từ `CreateSshTarget.Execute` đã được `apperrors.New(...)` bọc lại
(`INFRA_CREATE_SSH_TARGET_FAILED`) — `errors.As` chỉ tìm được `*pgconn.PgError` bên trong nếu `apperrors.New`
implement `Unwrap() error` trả về error gốc đã truyền vào (tham số cuối `err` của `apperrors.New`). Xác nhận
`apperrors`'s struct có `Unwrap()` trước khi khoá code này — đọc `common/apperrors` package, không giả định.

## Test cases cần cover

- `TestMigration0017_UniqueConstraintRejectsSameTenantHostDuplicate` — insert trực tiếp 2 row cùng
  `(tenant_id, host)` khác `id` → constraint violation (test tầng migration/DB, dùng test DB thật hoặc
  testcontainer theo convention hiện có của repo).
- `TestMigration0017_AllowsSameHostDifferentTenant` — 2 row cùng `host`, khác `tenant_id` → không lỗi.
- `TestBulkProvisionFleet_DuplicateHost_ReturnsAlreadyExistsNotFailed` (ở `bulk_provision_fleet_test.go`,
  TASK-BE-FLEET-001's phạm vi nếu task đó merge sau — nếu merge trước, thêm test này vào file test đã có) —
  chạy `BulkProvisionFleet` 2 lần với cùng `FleetSpec` → lần 2 server đã tồn tại có `Status: "SUCCEEDED"`,
  `Error: "already_exists"`, KHÔNG có `SshTarget` thứ 2 trong DB.

## Verify

```bash
ls backend-go/services/infra-fleet-service/migrations/ | sort -t_ -k1 -n | tail -5   # verify số thứ tự lần cuối
cd backend-go/services/infra-fleet-service && <lệnh chạy migration thật của repo — kiểm tra Makefile/README> up
go test ./internal/usecase/... -run BulkProvisionFleet -v
```

## gitnexus

Migration file không phải symbol Go — không cần `impact()` cho chính file `.sql`. Nếu task này sửa
`provisionOne` (nếu TASK-BE-FLEET-001 đã tồn tại), chạy `impact({target: "BulkProvisionFleet", direction: "upstream"})`
trước khi sửa, xác nhận không phá test/caller đã viết ở TASK-BE-FLEET-001.

## Blocking

Không task nào phụ thuộc cứng migration này để build (constraint chỉ ảnh hưởng runtime/test DB) — nhưng
`TestBulkProvisionFleet_DuplicateHost_ReturnsAlreadyExistsNotFailed` cần constraint này tồn tại để test có ý
nghĩa (không có constraint, insert trùng vẫn "thành công" ở DB layer, che mất bug thật nếu usecase logic
`isUniqueViolation` viết sai).
