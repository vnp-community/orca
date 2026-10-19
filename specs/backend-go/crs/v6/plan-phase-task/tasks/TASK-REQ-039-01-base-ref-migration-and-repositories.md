# TASK-REQ-039-01: Migration `0099_request_base_ref` và repository hai dialect

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `request-service`
**File:** `services/request-service/migrations/{postgres,mysql}/0099_request_base_ref.{up,down}.sql` (mới), `internal/adapter/{postgres,mysql}/request_repository.go`, `request_scan.go` (sửa), `internal/adapter/contracttest/expected_columns_base_ref.go` (mới), `internal/domain/request.go` (sửa nhỏ)
**Depends on:** Không
**Status:** [ ] TODO

---

## Context

- Migration mới nhất là `0098_execution_mode` ở cả hai dialect (kiểm lại số khi bắt đầu; nếu có CR khác giữ `0099` thì lấy số kế tiếp ở cả hai dialect và ghi vào PR).
- Mẫu: `0098_execution_mode.up.sql` thêm cột có CHECK; repository đọc cột trong `request_scan.go`; `expected_columns_execution_mode.go` khai báo cột cho test hợp đồng.

## Việc cần làm

1. `up` Postgres: `ALTER TABLE request.requests ADD COLUMN base_ref TEXT NOT NULL DEFAULT '' CHECK (char_length(base_ref) <= 255)`. MySQL: `ADD COLUMN base_ref VARCHAR(255) NOT NULL DEFAULT ''` (kiểm chế độ strict của MySQL để độ dài vượt bị từ chối, không cắt).
2. `down` cho cả hai dialect (`DROP COLUMN`).
3. `domain.Request.BaseRef string`; `request_scan.go` thêm cột vào danh sách và `Scan` (Postgres và MySQL).
4. `RequestRepository.SetBaseRef(ctx, id string, baseRef string, expectedVersion int64) (domain.Request, error)`: `UPDATE ... SET base_ref=$3, version=version+1, updated_at=now() WHERE id=$1 AND tenant_id=$2 AND version=$4`; 0 dòng bị ảnh hưởng thì phân biệt `NotFound` và `ErrVersionConflict`. Hàm nhận transaction của người gọi để task 02 khoá hàng và ghi trong cùng transaction.
5. `RequestRepository.GetForUpdate(ctx, id)` nếu chưa có (khoá hàng trong transaction) cho cả hai dialect.
6. Không backfill: Request cũ có `''`.

## Kiểm thử

- Hợp đồng (`request_repository_contract` hiện có): thêm ca `base_ref` mặc định rỗng, ghi và đọc lại, `SetBaseRef` đúng phiên bản và sai phiên bản, `SetBaseRef` khi `id` không tồn tại, CHECK độ dài (256 ký tự bị từ chối).
- Chạy migration xuôi rồi ngược rồi xuôi trên Postgres 16 và MySQL 8; Request tạo trước migration đọc được.
- `cd backend-go && go test ./services/request-service/internal/adapter/... -run 'Contract' -v`.

## Tiêu chí hoàn thành

- [ ] `0099` chạy xuôi và ngược trên cả hai dialect.
- [ ] `SetBaseRef` có khoá lạc quan và test chứng minh.
- [ ] Request cũ đọc được, `BaseRef==""`.
- [ ] Test hợp đồng hai dialect xanh.

## Rủi ro và lưu ý

- Số migration có thể bị CR khác giữ; kiểm trước khi viết.
- MySQL non-strict mode có thể cắt âm thầm; test CHECK/độ dài phải chạy ở chế độ đúng của triển khai.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
