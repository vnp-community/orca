# BE-CV-TASK-010-02: Thêm `KindResourceExhausted` và `KindUnavailable` vào `common/apperrors`

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `common` (thay đổi duy nhất được phép, PQ-03 (8))
**File:** `backend-go/common/apperrors/apperrors.go`, `backend-go/common/apperrors/apperrors_test.go` (sửa/tạo nếu chưa có)
**Depends on:** BE-CV-TASK-010-01 (không bắt buộc; có thể song song)
**Status:** [ ] TODO

---

## Context

`apperrors.Kind` hiện có 8 giá trị (`KindUnknown`…`KindDeadlineExceeded`, `apperrors.go` dòng 22–36), `ToGRPCStatus` ánh xạ 8 `case`, mặc định `codes.Unknown`. Hợp đồng PQ-03 (8) giao CR-CV-010 thêm hai Kind để SOL-013 (`CODEINTEL_RATE_LIMITED`, `CODEINTEL_CONCURRENCY_LIMIT`, `CODEINTEL_REINDEX_COOLDOWN`) và 012 (`CODEINTEL_DEV_SERVER_OFFLINE` = `Unavailable`) dùng. Chú thích của `ToGRPCStatus` nêu 463 điểm gọi: thay đổi phải thuần cộng. Quy tắc dự án: chạy `gitnexus_impact` trước khi sửa symbol.

## Việc cần làm

1. Chạy `gitnexus_impact({target: "ToGRPCStatus", direction: "upstream"})` và `gitnexus_impact({target: "Kind"})`; ghi blast radius vào PR; nếu HIGH/CRITICAL, cảnh báo người duyệt (thay đổi vẫn thuần cộng).
2. Thêm `KindResourceExhausted` và `KindUnavailable` **ở cuối** khối `const` sau `KindDeadlineExceeded` (giữ nguyên giá trị `iota` của Kind cũ).
3. Thêm hai `case` vào `ToGRPCStatus`: `KindResourceExhausted → codes.ResourceExhausted`, `KindUnavailable → codes.Unavailable`.
4. Viết chú thích ngắn nêu lý do (hạn mức và hạ nguồn không với tới, PQ-03).
5. Test bảng: mỗi Kind (cũ và mới) → `status.Code`; test `Kind` cũ giữ giá trị số (so sánh với hằng số mong đợi); test thông điệp vẫn là `Code: Message` và không chứa `Err`.
6. Không sửa file nào khác ở `common`.

## Kiểm thử

- `cd backend-go && go test ./common/apperrors/...`
- `make build` toàn workspace (chỉ-đọc kết quả; chạy khi triển khai) để chắc không service nào vỡ do `switch` exhaustive linter.
- `make lint` cho module `common`.

## Tiêu chí hoàn thành

- [ ] Hai Kind mới ánh xạ đúng `codes`; Kind cũ không đổi giá trị/ánh xạ.
- [ ] Test bảng xanh; `make build`/`make lint` xanh trên toàn workspace.
- [ ] Mô tả PR có kết quả `gitnexus_impact`.

## Rủi ro và lưu ý

- Chạm package dùng bởi mọi service: PR riêng, nhỏ, review bởi chủ `common`.
- Không đổi định dạng `Code: Message` (gateway và client dựa vào tiền tố, PQ-02).
