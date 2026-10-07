# BE-CV-TASK-021-01: Re-verify nền collector (trailer qua otelgrpc, mẫu client, trạng thái 010/012/013/023)

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `code-intel-service`
**File:** (không sửa mã) `backend-go/services/{code-intel-service,git-gateway-service,infra-fleet-service}`, `backend-go/common/apperrors`
**Depends on:** —
**Status:** [x] DONE

---

## Context

Hợp đồng đánh dấu chưa kiểm chứng: `grpc.Trailer` qua `otelgrpc`, hình `data` của agent, `KindUnavailable`. Task chốt các điểm này trước khi viết adapter.

## Việc cần làm

1. Đọc lại `git-gateway-service/internal/adapter/grpcclient/{resolver.go,relay_executor.go,tenant_forwarding.go}` và ghi các hàm sẽ viết lại (không import chéo service).
2. Xác nhận `common/apperrors` đã có `KindUnavailable`/`KindResourceExhausted` (SOL-010); nếu chưa, **dừng** task 04 và ghi phụ thuộc.
3. Xác nhận SOL-023 đã merge (`grep -n StreamCodeIntelEvents backend-go/proto/orca/infrafleet/v1/infrafleet.proto`); nếu chưa, task 09 chạy ở chế độ đỏ.
4. Xác nhận SOL-012 đã cung cấp `AgentTarget`/`ResolveTarget`; nếu chưa, định nghĩa struct tạm trong `agent_code_intel_gateway.go` và ghi vào PR.
5. Viết thử nhỏ (không commit) một server gRPC giả đặt `grpc.SetTrailer` rồi client có `otelgrpc.NewClientHandler()` đọc `grpc.Trailer`; ghi kết quả.

## Kiểm thử

- `cd backend-go/services/code-intel-service && go build ./...` (nếu service đã có).
- Lệnh grep ở bước 2–4; kết quả dán vào PR.

## Tiêu chí hoàn thành

- [x] Có kết luận rõ về trailer qua `otelgrpc` (có/không): Có, gRPC trailer qua được bình thường.
- [x] Danh sách phụ thuộc chưa sẵn (010/012/013/023) ghi trong PR (SOL-010 KindUnavailable/KindResourceExhausted đã thêm vào apperrors; SOL-012 AgentTarget định nghĩa tại usecase; SOL-023 hoàn tất).

## Rủi ro và lưu ý

- Nếu trailer không qua được, phương án dự phòng: `data` ngoài message (CR-023 nêu, chưa chọn); báo chủ hợp đồng.
