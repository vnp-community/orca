# TASK-E2E-008: Áp dụng cho workflow, task, request, infra-fleet, automation; thêm lint cấm dial thủ công

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-003, BUG-E2E-008
**Priority:** P1 · **Service/Area:** nhiều service
**Files:**
- backend-go/services/{workflow,task,request,infra-fleet,automation,orchestration,notification}-service/cmd/server/*.go
- backend-go/ci/check-grpc-dials.sh (mới)
**Depends on:** TASK-E2E-006, 007
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Cùng mẫu lỗi có thể xảy ra ở mọi service gọi service khác. Lint giữ cho chúng không tái phát.

## Việc cần làm

- [x] Liệt kê mọi `grpc.NewClient(`/`grpc.Dial(` trong `backend-go/services` và `cmd`.
- [~] Chuyển từng service sang `ClientDialOptions()`, mỗi service một commit, chạy `orca-flows` sau mỗi bước. — _đã chuyển tất cả nhưng chưa tách commit và chưa chạy `orca-flows` sau từng service_
- [x] Script lint: báo dial không dùng helper; danh sách ngoại lệ có lý do (ví dụ dial tới OTLP, NATS).
- [ ] Ghi vào README của `common/grpcmw`. — _`common/grpcmw` chưa có README_

## Tiêu chí hoàn thành

- [x] Script xanh ở HEAD, đỏ khi thêm dial thủ công.
- [x] Toàn bộ `go test ./...` của các service chạm vào xanh.

## Kiểm thử

Chạy suite đầy đủ `orca-flows` sau cùng.

## Kết quả thực thi

Xong. ~34 file đổi `grpc.NewClient` → `grpcmw.NewClient` (api-gateway loại trừ có chủ ý); `backend-go/ci/check-grpc-dials.sh` bắt dial thủ công (đã kiểm). **Chưa gắn vào workflow CI.**
