# BE-CV-TASK-093-01: infra-fleet: ngoại lệ timeout `ai.complete` 120 s trong `execTimeoutForMethod`

**From Solution:** BE-CV-SOL-093-ai-review-summary
**Priority:** P2
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (sửa, `execTimeoutForMethod` :412), `client_test.go` (sửa)
**Depends on:** — (độc lập; phối hợp BE-CV-SOL-023-infra-fleet-codeintel-transport nếu CR-023 cũng sửa hàm này)
**Status:** [x] DONE

## Context
Hiện chỉ `agent.execPrompt` có ngoại lệ (15 phút); mọi method khác trả 0 ⇒ mặc định 30 s (đã đọc). Hợp đồng agent §2.5: `ai.complete` Go **120 s**. Đổi này cũng tác động `git-gateway` `GenerateCommitMessage`.

## Việc cần làm
1. Hằng `aiCompleteTimeout = 120 * time.Second`; `case "ai.complete"` (nếu CR-023 chuyển sang bảng theo method, thêm hàng, không tạo cơ chế thứ hai).
2. Cập nhật comment của hàm (không còn "the one method").
3. Test bảng: `agent.execPrompt`=15m, `ai.complete`=120s, `ports.scan`/`git.status`=0.
4. Báo chủ `git-gateway` về thay đổi hành vi.

## Kiểm thử
- `go test ./services/infra-fleet-service/internal/adapter/devserveragent/...` (chưa chạy); test `callWithTimeout` dùng giá trị 120 s với đồng hồ giả nếu có.

## Tiêu chí hoàn thành
- [x] bảng timeout đúng; [ ] không đổi method khác.

## Rủi ro
- Biên 120 s trùng agent (Q2 SOL-093); deadline gRPC của caller ngắn hơn vẫn cắt.
