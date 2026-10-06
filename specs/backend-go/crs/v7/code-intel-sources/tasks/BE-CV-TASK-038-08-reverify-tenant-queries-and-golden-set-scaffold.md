# BE-CV-TASK-038-08: Re-verify truy vấn thiếu `tenant_id` và dựng khung bộ vàng 100 truy vấn

**From Solution:** BE-CV-SOL-038-static-tenant-filter-rule
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/sqltenant/README.txt`, `testdata/sqltenant/golden/queries.yaml` (khung, mới); `testdata/sqltenant/go/*.go` (mẫu rút gọn, mới)
**Depends on:** BE-CV-SOL-031-erd-model-and-access-scan (`tenantScoped`)
**Status:** [ ] TODO

---

## Context

Số đo CR (1 214 chuỗi, 466 không có `tenant_id`, 116 trên bảng có cột, ~94 sau bỏ outbox, 6 mạnh nhất) là regex thô, bỏ sót nối `+`. Bước này đo lại có kiểm soát và dựng khung nhãn tay. Chỉ đọc.

## Việc cần làm

1. Đọc lại `folder_workspace_repository.go` bản Postgres và MySQL (`Update`, `Delete`) và các **use case gọi** (`project-service/internal/usecase/*folder*`): ghi vào PR use case có kiểm tenant trước khi gọi hay không. Nếu có, hạ ghi chú "ứng viên mạnh nhất" ở solution §7.
2. Đọc `scm-integration-service/.../webhook_delivery_repository.go` (`SELECT 1 … WHERE provider = $1 AND delivery_id …`): ghi có nên thuộc tầng `low`.
3. Xác nhận `mcp-service/.../tenant_tx.go` là nơi duy nhất đặt `set_config('app.tenant_id'` (grep chỉ-đọc trên `internal/adapter/postgres`); ghi danh sách service có literal đó.
4. Dựng `testdata/sqltenant/go/`: tệp Go rút gọn (hợp lệ cú pháp) chứa 20 chuỗi SQL đại diện: literal đơn, nối `+` với `const`, `Sprintf`, `strings.Builder`, CTE, `RETURNING`, `INSERT` thiếu cột, truy vấn theo `token_hash`, `outbox_events`, hàm `*Relay*`, có/không `codeintel:allow`.
5. `golden/queries.yaml`: khung 100 mục `{id, service, dialect, file, func, sqlNormalized, expectedTier, truth: real|false|unknown}`; điền ≥ 20 mục từ mẫu ở bước 4 (nhãn do người), 80 mục còn lại **để trống chờ nhãn tay** và ghi người phân loại (không tự gán nhãn thay người).
6. `README.txt`: cách lấy mẫu có chủ ý (50 từ `high`+`medium`, 50 từ nhóm đã lọc), cách ghi precision/recall.

## Kiểm thử

- Không test Go. Kiểm tay: YAML hợp lệ; mỗi tệp Go mẫu parse được bằng `go/parser`.

## Tiêu chí hoàn thành

- [ ] Kết luận về use case `folder_workspaces` và `webhook_delivery`.
- [ ] Khung bộ vàng 100 mục, 20 mục đã gán nhãn.
- [ ] 20 chuỗi mẫu phủ các dạng trích xuất.

## Rủi ro và lưu ý

- Không dùng mã/SQL thật nhạy cảm; rút gọn và đổi tên khi chép.
