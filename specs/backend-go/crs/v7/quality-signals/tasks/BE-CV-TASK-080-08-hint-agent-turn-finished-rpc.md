# BE-CV-TASK-080-08: RPC `HintAgentTurnFinished` (P1, tuỳ chọn)

**From Solution:** BE-CV-SOL-080
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/hint_agent_turn_finished.go`, `internal/adapter/grpc/` (handler), dòng `rpc` trong `codeintel.proto` (mới)
**Depends on:** BE-CV-TASK-080-05, BE-CV-SOL-040-codeintel-write-and-stream-channels
**Status:** [x] DONE

## Context
C-DM §3.1: `HintAgentTurnFinished (mới, P1, tuỳ chọn)`, action `read`, kênh `codeIntel.hintAgentTurnFinished`. PQ-35. Bỏ qua nếu đã có refresh trong cửa sổ debounce.

## Việc cần làm
1. Request `{selector, agent_pane_key?}`; response rỗng; thêm message và dòng `rpc` (cập nhật bảng C-DM §3.1 trong PR hợp đồng riêng).
2. Use case: kiểm cờ/quyền `read`, phân giải `selector`, `UpsertSlot` (event id = hash pane+thời điểm làm tròn 10 s).
3. Từ chối phiên thiết bị (`CODEINTEL_NOT_AUTHORIZED`).

## Kiểm thử
- Unit: trùng với slot sẵn có chỉ chạm; cờ tắt ⇒ `CODEINTEL_DISABLED`.

## Tiêu chí hoàn thành
- [x] `buf breaking` xanh; kênh gateway đăng ký ở SOL-040.

## Rủi ro
Lạm dụng gọi liên tục: giới hạn bởi debounce và hạn mức.
