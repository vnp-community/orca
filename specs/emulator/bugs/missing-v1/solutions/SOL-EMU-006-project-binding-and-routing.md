# SOL-EMU-006: `mobileEmulatorAgentId` trên project & định tuyến `emulator.*`

**Resolves:** CR-DS-009 §3.2–3.3, Phase 4
**Service:** `backend-go/services/project-service`, `backend-go/services/api-gateway`
**Status:** ✅ DONE
**Task(s):** [TASK-EMU-008](../tasks/TASK-EMU-008-project-binding-mobile-emulator-agent-id.md), [TASK-EMU-009](../tasks/TASK-EMU-009-route-emulator-channels-by-dev-server-id.md)

## Giải pháp (đã triển khai)

- `project-service`: thêm field `mobileEmulatorAgentId` song song
  `devServerId` hiện có (F34) — nhưng KHÔNG dùng cùng cơ chế guarded-rebind
  của `devServerId` (`RebindDevServer`); đi qua `UpdateProject` như một
  field cập nhật thường, vì không có active-execution concern gắn với việc
  đổi Mobile Emulator Agent (xem TASK-EMU-008's "quyết định thiết kế").
- `channels_emulator_folderworkspace_host.go`'s `registerEmulatorChannels`:
  đổi nguồn id từ `connectionId` kiểu git/worktree client-gửi sang
  `projectId` client-gửi, resolve qua `GetProject(projectId).mobileEmulatorAgentId`
  rồi `ResolveConnection{dev_server_id: mobileEmulatorAgentId}` để lấy
  `connectionId` thật. `usecase.EmulatorRelay` ở `infra-fleet-service`
  **không sửa gì** — xác nhận đúng dự đoán, `Relay`/`RelayByDevServer` vốn
  generic theo `devServerId`/`connectionId` + method string.

## Vì sao pass trước "chưa làm"

Phụ thuộc trực tiếp vào SOL-EMU-005 (field `kind` phải tồn tại trước để
biết đâu là hàng `AGENT_KIND_MOBILE_EMULATOR` hợp lệ để bind) — SOL-EMU-005
đã DONE trong cùng pass này, nên SOL-EMU-006 triển khai ngay sau, đúng thứ
tự phụ thuộc CR-DS-009's lộ trình đã ghi. `go build`/`go vet`/`go test`
xanh cho `project-service` và `api-gateway` (2 service dùng field/routing
này); test mới xác nhận cả đường đi thật (resolve → relay) lẫn đường đi cũ
(honest stub khi chưa bind). Xem TASK-EMU-008/TASK-EMU-009 để có lệnh +
output thật.
