# BE-CV-TASK-023-01: Re-verify người tiêu thụ lỗi `Exec` và các fake trước khi đổi kiểu lỗi

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** (không sửa mã; kết quả ghi vào mô tả PR) `backend-go/services/infra-fleet-service/internal/**`
**Depends on:** —
**Status:** [ ] TODO

---

## Context

SOL-023 mục 6 ghi rõ chưa grep hết nơi dùng `errors.As(…, *JSONRPCError)` hay phụ thuộc kiểu lỗi của `Exec`. Task này chốt blast radius trước khi sửa `Client.Exec`; hợp đồng đánh dấu khu vực này "chưa kiểm chứng" (agent contract §0).

## Việc cần làm

1. `grep -rn "JSONRPCError" backend-go/services/infra-fleet-service --include=*.go` và ghi lại mọi nơi ngoài `adapter/devserveragent`. Kỳ vọng: không nơi nào ngoài adapter; nếu có, liệt kê vào PR và chuyển sang dùng `errors.Is(domain.ErrAgentMethodNotFound)`.
2. `grep -rn "ErrAgentMethodNotFound" …` (đã biết: `domain/agent_relay.go`, `client.go` dòng ~437 và ~899, `methods.go` ~232, các test); xác nhận không nơi nào so sánh chuỗi lỗi thay vì `errors.Is`.
3. Liệt kê mọi gọi `agent.Exec(` ở `usecase/` (grep) để biết use case nào nhận `AgentRPCError` khi method là `codeintel.`/`quality.` (kỳ vọng chỉ `Relay`, `RelayByDevServer`).
4. Đếm fake của `usecase.DevServerAgentClient` (`grep -rln "IsConnected(devServerID string) bool"`; đã thấy 6 file test) để xác nhận quyết định **không** thêm phương thức mới vào interface đó.
5. Xác nhận `cmd/server/main.go` không dùng `internalcaller.StreamGuard` (`grep -n "internalcaller\|StreamGuard" cmd/server/main.go` rỗng) và ghi kết luận.
6. Chạy baseline: `cd backend-go/services/infra-fleet-service && go test ./... -race` và ghi test đỏ sẵn có (nếu có) để không quy cho thay đổi này.

## Kiểm thử

- Lệnh ở các bước 1–6; đầu ra dán vào PR. Không thay đổi file.

## Tiêu chí hoàn thành

- [ ] Danh sách nơi dùng `JSONRPCError`/`ErrAgentMethodNotFound`/`Exec` được ghi vào PR.
- [ ] Kết luận rõ: có/không nơi nào phải sửa ngoài `devserveragent` và `usecase/relay*.go`.
- [ ] Baseline `go test ./... -race` ghi nhận.

## Rủi ro và lưu ý

- Nếu baseline đã đỏ vì lý do khác, ghi số test đỏ để TASK-023-09 so sánh.
- Phạm vi chỉ-đọc: không chạy `go generate`, không sửa `go.mod`.
