# BE-CV-TASK-031-11: Handler gRPC `GetErd` (selector, quyền `read`, phong bì, lỗi `CODEINTEL_*`)

**From Solution:** BE-CV-SOL-031-erd-model-and-access-scan
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/get_erd_handler.go` (mới), `.../adapter/grpc/get_erd_handler_test.go` (mới); đăng ký trong `cmd/server/main.go` (sửa nhẹ)
**Depends on:** BE-CV-TASK-031-07, BE-CV-TASK-031-10; BE-CV-SOL-013 (guard, cờ, OPA); BE-CV-SOL-012 (target resolution)
**Status:** [x] DONE

---

## Context

Chuỗi xử lý của mọi RPC (hợp đồng §3 đầu mục): `internalcaller.Guard` → tenant/user từ metadata → cờ → quyền OPA → phân giải `selector` → cổng agent → quyền **trước** cache → che bí mật → audit. Handler này chỉ gắn `GetErd` vào chuỗi đó (middleware thuộc SOL-013).

## Việc cần làm

1. Re-verify: đọc `internal/adapter/grpc/` hiện có (SOL-010/013) để dùng đúng khuôn handler và interceptor; không viết lại guard.
2. `GetErd(ctx, *GetErdRequest)`: validate `service` (`^[a-z0-9][a-z0-9-]{0,63}$`), `dialect ∈ {"", postgres, mysql}`, `base_ref`/`head_ref` (không bắt đầu `-`, ≤ 255, ký tự an toàn; `head_ref` rỗng hoặc 40/64 hex), `if_none_match ≤ 80` → `CODEINTEL_INVALID_PARAMS` (`data.field`).
3. Gọi `BuildErd`; ánh xạ `ResultMeta` (`head_commit`, `stale`, `truncated`, `total_count`, `etag`, `from_cache`, `generated_at`, `not_modified`); **không** điền `dev_server_id` ra response (PQ-12).
4. Lỗi: `apperrors` → status với message `CODEINTEL_X: …`; hậu tố ` | {json}` ≤ 2 KiB cho `retryAfterMs/inProgress` khi `ErrInProgress` (PQ-02, PQ-13). Không dùng `status.Details`.
5. Kiểm `proto.Size(resp) ≤ 2 MiB` (PQ-14); vượt → cắt `tables` theo thứ tự tên + `truncated=true`; vẫn vượt → `CODEINTEL_OUTPUT_TOO_LARGE`.
6. Cờ tắt → `CODEINTEL_DISABLED` do middleware; handler không tự kiểm.
7. Audit: không ghi nội dung; chỉ `service`, `dialect`, số bảng.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/grpc/... -run GetErd` (chưa chạy) với use case giả.
- Bảng ca tham số xấu; `ErrInProgress` → message có hậu tố đúng định dạng regex `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`; `not_modified` không có `model`.
- Quyền: caller khác tenant với binding → `CODEINTEL_NOT_AUTHORIZED` (do tầng phân giải; test với resolver giả); phiên thiết bị (`DeviceID != ""`) bị từ chối.
- Kích thước: model giả > 2 MiB → cắt/`OUTPUT_TOO_LARGE`.
- Gateway: `TestChannelInventory` và `TestToolParity` thuộc `BE-CV-SOL-040-codeintel-view-channels` (kênh `codeIntel.erd`), không làm ở đây.

## Tiêu chí hoàn thành

- [x] `GetErd` gọi được qua `grpcurl`/client test trong suite.
- [x] Mọi lỗi có tiền tố `CODEINTEL_`; không mã nguồn/đường dẫn tuyệt đối trong message.
- [x] Không có đường bỏ qua kiểm quyền trước cache.

## Rủi ro và lưu ý

- Tên interceptor/guard do SOL-010/013 đặt; nếu chúng chưa merge, task chặn.
- Giải G1 (`services[]` ở response) phải được hợp đồng chấp nhận trước khi merge `codeintel_erd.proto`.
