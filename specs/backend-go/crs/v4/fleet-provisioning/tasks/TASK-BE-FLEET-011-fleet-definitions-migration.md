# TASK-BE-FLEET-011: Migration `0018_fleet_definitions`

**Solution:** BE-FLEET-SOL-003 §3 | **CR:** CR-FLEET-003
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-004 (migration `0017` phải merge trước — số thứ tự tiếp theo phụ thuộc nó)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `ls migrations/ | sort -t_ -k1 -n | tail -5` verify lại ngay trước khi tạo file —
> `0017_ssh_targets_host_unique` (TASK-BE-FLEET-004, đã merge trong cùng phiên này) là mới nhất → `0018` đúng
> như dự đoán, không lệch số.
>
> **Kiểm tra CR-DB-002:** đọc `docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md` —
> CR này vẫn ở trạng thái Backlog, CHƯA chọn service pilot cụ thể (2 ứng viên đề xuất là `usage-service`/
> `annotation-service`, không phải `infra-fleet-service`) → giữ nguyên `JSONB` như task đề xuất, không đổi
> sang dialect-safe type.
>
> **Test tích hợp thật đã chạy** (testcontainers postgres:16-alpine, Docker sẵn có): 3 test mới
> (`TestMigration0018_CreatesTable`, `TestMigration0018_UniqueConstraintRejectsSameTenantNameDuplicate`,
> `TestMigration0018_DownDropsTable`) — PASS khi container khởi động đúng. **Ghi nhận trung thực:** môi trường
> này có 1 flake tiền tồn tại ở tầng testcontainers (đã gặp tương tự ở TASK-BE-FLEET-004/002) — container báo
> cổng 5432 sẵn sàng trước khi Postgres phục hồi xong sau initdb, gây lỗi `"pq: the database system is
> starting up"` không đều (~25-50% lần chạy trong nhiều lần thử, không cố định) — KHÔNG liên quan tới nội dung
> SQL của migration này (chạy `up`/`down`/constraint logic đều đúng khi container thật sự sẵn sàng, xác nhận
> qua nhiều lần chạy lặp lại PASS sạch). `TestMigration0018_DownDropsTable` xác nhận `down 1` rollback sạch
> (bảng biến mất, không lỗi).
>
> **`go build ./...`/`gofmt -l`**: sạch.

---

## ⚠️ Bắt buộc verify số thứ tự migration THẬT ngay trước khi tạo file

```bash
ls backend-go/services/infra-fleet-service/migrations/ | sort -t_ -k1 -n | tail -5
```

Tại thời điểm viết solution (2026-09-09), file mới nhất là `0016_ephemeral_vm_ssh_target_host_key.up/down.sql`
— nếu TASK-BE-FLEET-004's `0017_ssh_targets_host_unique` đã merge, số đúng cho task này là **`0018`**. Nếu
TASK-BE-FLEET-004 CHƯA merge khi task này bắt đầu, xác nhận lại thứ tự thực thi (§"Blocking" bên dưới) —
KHÔNG tự đoán số dựa trên tên task, luôn `ls` thật.

## ⚠️ Kiểm tra chéo CR-DB-002 trước khi hard-code `JSONB`

Đọc `docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md` (file có tồn tại thật, đã
verify) — nếu `infra-fleet-service` nằm trong danh sách service pilot multi-dialect **tại thời điểm task này
thực thi**, đổi kiểu cột `JSONB` → kiểu dialect-safe theo capability layer CR-DB-002 cung cấp (dùng đúng
helper/kiểu đã chuẩn hoá, không tự chọn `JSON`/`TEXT` tuỳ tiện). Nếu `infra-fleet-service` KHÔNG nằm trong
danh sách đó, giữ `JSONB` như dưới đây.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/migrations/0018_fleet_definitions.up.sql` (MỚI — hoặc số đúng sau khi verify)
2. `backend-go/services/infra-fleet-service/migrations/0018_fleet_definitions.down.sql` (MỚI)

## Nội dung migration

```sql
-- 0018_fleet_definitions.up.sql
CREATE TABLE infra.fleet_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    servers JSONB NOT NULL,
    provision JSONB,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
```

```sql
-- 0018_fleet_definitions.down.sql
DROP TABLE infra.fleet_definitions;
```

## Test cases cần cover

- `TestMigration0018_CreatesTable` — chạy migration lên test DB, xác nhận bảng tồn tại đúng cột.
- `TestMigration0018_UniqueConstraintRejectsSameTenantNameDuplicate` — insert 2 row cùng `(tenant_id, name)`
  → lỗi.
- `TestMigration0018_DownDropsTable` — chạy `down` sau `up`, xác nhận bảng không còn tồn tại (rollback sạch).

## Verify

```bash
ls backend-go/services/infra-fleet-service/migrations/ | sort -t_ -k1 -n | tail -5   # verify lần cuối trước khi tạo file
cd backend-go/services/infra-fleet-service && <lệnh migration up thật của repo>
<lệnh migration down thật của repo>   # xác nhận rollback sạch, không lỗi
```

## gitnexus

Không áp dụng — file `.sql`, không phải symbol Go.

## Blocking

TASK-BE-FLEET-012 (repository/CRUD usecase) phụ thuộc bảng này tồn tại để test tầng Postgres adapter chạy
được (unit test usecase với fake repository không phụ thuộc, nhưng test tích hợp/adapter thì có).
