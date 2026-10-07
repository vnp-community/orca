# TASK-REQ-025-05: Kịch bản E20 (cờ), workflow CI ma trận hai dialect và job e2e

**From Solution:** BE-REQ-SOL-025
**Priority:** P0
**Service:** `request-service`, `.github`
**File:** `backend-go/services/request-service/e2e/feature_flag_test.go` (mới), `.github/workflows/backend-go-request-service.yml` (sửa: thêm bước e2e; file do CR-REQ-001 tạo)
**Depends on:** TASK-REQ-025-02, TASK-REQ-025-04
**Status:** `[x] DONE`

---

## Context

- Workflow mẫu `backend-go-issue-status-sync.yml`: `strategy.matrix.dialect: [postgres, mysql]`, `actions/setup-go@v5` go `1.25`, `go build`, `go vet`, `go test ./...`, rồi tích hợp với `-tags=integration`; `paths` lọc theo `backend-go/common/**`, `backend-go/services/<svc>/**`, `backend-go/go.work`.
- CR-REQ-001 tạo `backend-go-request-service.yml` sao `backend-go-task-service.yml`; kiểm nội dung thật trước khi sửa (chưa có file tại thời điểm viết: `ls .github/workflows | grep request` rỗng).
- Bảng hành vi khi tắt cờ: SOL-025 mục 2.2 / CR-REQ-025 mục 2.2.
- E20: tắt, bật, tắt giữa chừng; tenant A bật không ảnh hưởng tenant B.

## Việc cần làm

1. `feature_flag_test.go`: (a) cờ tắt: mọi RPC "đi tiếp" trả `REQUEST_FLOW_DISABLED`, đọc và thoát an toàn (`CancelRequest`, `ReturnToBacklog`, `Reject`, `Cancel` Approval) vẫn chạy; (b) Request đang `executing`, tắt cờ: callback `ReportTaskOutcome` vẫn ghi kết quả; (c) bật lại: tiếp tục từ chỗ cũ; (d) hai tenant: A bật, B tắt; (e) biến tổng tắt nhưng tenant bật vẫn bị chặn; (f) lỗi đọc cờ giả lập (repo trả lỗi) thì chặn.
2. Workflow: thêm bước `E2E tests (${{ matrix.dialect }})` chạy `go test -tags=e2e ./e2e/...` với `E2E_DIALECT=${{ matrix.dialect }}`, `timeout-minutes` rõ (ví dụ 25, đo rồi chỉnh).
3. Bước kiểm đăng ký (task 06) gọi trong cùng workflow, một lần (không lặp theo ma trận).
4. Cập nhật `paths` để thay đổi `proto/orca/request/**` cũng kích hoạt.

## Kiểm thử

- Local: `E2E_DIALECT=postgres go test -tags=e2e ./e2e/... -run Flag`, `mysql`.
- CI: mở PR thử, kiểm cả hai ô ma trận chạy e2e (chưa kiểm chứng; ghi kết quả vào PR).

## Tiêu chí hoàn thành

- [x] E20 xanh trên hai dialect.
- [x] Workflow chạy e2e hai dialect và chặn PR khi đỏ.
- [x] Chạy nhanh hơn ngân sách đặt (ghi thời gian thực đo).

## Rủi ro và lưu ý

- Nếu chạy e2e quá chậm trong CI, tách job riêng (`needs: test`) thay vì cắt kịch bản.
- Không thêm `continue-on-error` cho T1.
