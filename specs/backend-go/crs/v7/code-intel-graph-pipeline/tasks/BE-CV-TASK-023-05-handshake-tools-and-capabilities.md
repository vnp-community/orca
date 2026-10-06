# BE-CV-TASK-023-05: Công bố `tools[]`, `capabilities`, `sessionId` từ handshake tới `usecase.HandshakeInfo`

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go` (sửa `inboundHandshakeParams` dòng 73–81 và dựng `HandshakeInfo` dòng ~205–213), `.../adapter/devserveragent/session.go` (sửa `HandshakeInfo`, dòng 47–60), `.../adapter/devserveragent/client.go` (sửa `LastHandshakeInfo`, dòng ~481–493), `.../usecase/ports.go` (sửa `HandshakeInfo`, dòng 85–90), test
**Depends on:** TASK-023-01
**Status:** [ ] TODO

---

## Context

Agent gửi `tools: tools.map(t => t.name)` trong `agent.handshake` (agent contract §0) nhưng Go không đọc; README v7 mục 1 ("Go lưu `tools[]` qua `LastHandshakeInfo`") lệch code (PQ-18). `session.go` đã có `SessionID` và `Capabilities` trong `HandshakeInfo`, nhưng `usecase.HandshakeInfo` chỉ có 4 trường.

## Việc cần làm

1. `inboundHandshakeParams`: thêm `Tools []string \`json:"tools"\``; `HandshakeInfo` (devserveragent): thêm `Tools []string \`json:"tools"\``; `agentwsserver` điền `Tools: params.Tools` khi dựng `info`.
2. `usecase.HandshakeInfo`: thêm `SessionID string`, `Capabilities []string`, `Tools []string` (cập nhật comment: vẫn là bản sao thuộc usecase, không import adapter).
3. `Client.LastHandshakeInfo`: chuyển đủ trường mới. Sao chép slice (không chia sẻ bộ nhớ với `session.handshakeInfo`).
4. Không đổi DB: mọi nơi tiêu thụ `HandshakeInfo` hiện có (`UpdateProvisionResult`, `establish_connection.go` ~123) bỏ qua trường mới; kiểm bằng biên dịch.
5. `relay-websocket`: `Tools` rỗng (chấp nhận, D2).

## Kiểm thử

- `cd backend-go/services/infra-fleet-service && go test ./internal/adapter/agentwsserver/ ./internal/adapter/devserveragent/ ./internal/usecase/ -run 'Handshake|LastHandshakeInfo' -race`.
- Mở rộng `last_handshake_info_test.go`: handshake có `tools:["gitnexus","codegraph","git"]`, `capabilities:["pty","fs","git","codeintel","codeintel.gitnexus"]` → `LastHandshakeInfo` trả đủ; sửa slice trả về không đổi dữ liệu trong session.
- `server_test.go` của `agentwsserver`: handshake thiếu `tools` → `Tools == nil`, không lỗi.

## Tiêu chí hoàn thành

- [ ] `tools`, `capabilities`, `sessionId` tới được `usecase.HandshakeInfo`.
- [ ] Test hiện có không đổi kết quả; không migration.
- [ ] Không đổi `AgentCapability` union ở agent (việc của AG-CV-SOL-001).

## Rủi ro và lưu ý

- `tools[]` chỉ chốt lúc handshake; cài/gỡ `gitnexus` sau đó không cập nhật tới khi nối lại (`codeintel.status` là nguồn sự thật, agent contract §1.3).
- Fake `LastHandshakeInfo` ở 6 file test dùng struct literal có thể cần cập nhật nếu dùng tham số vị trí (kiểm khi biên dịch).
