# specs/emulator — Mobile Emulator Agent

Tài liệu kỹ thuật cho **Mobile Emulator Agent** (package `emulator/`) — agent
độc lập với Dev Server Agent (`agent/`), theo
[`CR-DS-009`](../../docs/crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md).
Cấu trúc mirror `specs/agent/` (tdd + bugs/solutions/tasks).

## Trạng thái tổng quan (2026-09-03)

| Phase (CR-DS-009) | Solution | Task(s) | Trạng thái |
|---|---|---|---|
| Phase 2 — Package scaffold | [SOL-EMU-002](bugs/missing-v1/solutions/SOL-EMU-002-emulator-package-scaffold.md) | [TASK-EMU-002](bugs/missing-v1/tasks/TASK-EMU-002-scaffold-emulator-workspace.md) | ✅ Done |
| Phase 2 — Capability/list handlers | [SOL-EMU-003](bugs/missing-v1/solutions/SOL-EMU-003-device-capability-and-list-handlers.md) | [TASK-EMU-003](bugs/missing-v1/tasks/TASK-EMU-003-device-capabilities-and-list-handlers.md) | ✅ Done |
| Phase 2 — RPC dispatch | (thuộc SOL-EMU-003) | [TASK-EMU-004](bugs/missing-v1/tasks/TASK-EMU-004-emulator-rpc-dispatch.md) | ✅ Done (control ops honest-stub) |
| Phase 2 — Entry point | (thuộc SOL-EMU-003) | [TASK-EMU-005](bugs/missing-v1/tasks/TASK-EMU-005-emulator-entry-stdio-debug-mode.md) | ✅ Done (stdio debug mode; giờ song song với direct-websocket, xem TASK-EMU-006) |
| Phase 1 — Shared transport extraction | [SOL-EMU-001](bugs/missing-v1/solutions/SOL-EMU-001-shared-transport-extraction.md) | [TASK-EMU-001](bugs/missing-v1/tasks/TASK-EMU-001-extract-dev-agent-transport-package.md) | ✅ DONE — verify xanh 100%, zero regression trên `agent/`, xem TDD-EMU-03 |
| Phase 2 — Wire protocol thật | (phụ thuộc SOL-EMU-001) | [TASK-EMU-006](bugs/missing-v1/tasks/TASK-EMU-006-wire-protocol-transport-integration.md) | ✅ DONE — direct-websocket mode thật nối `orca-dev-agent-transport`; end-to-end với `infra-fleet-service` thật chưa verify |
| Phase 2 — Device control ops thật (tap/gesture/attach/shutdown) | [SOL-EMU-004](bugs/missing-v1/solutions/SOL-EMU-004-device-control-handlers.md) | [TASK-EMU-010](bugs/missing-v1/tasks/TASK-EMU-010-device-control-handlers-port.md) | ✅ DONE cho Android (adb thật, unit test 61/61 xanh); iOS honest-stub có chủ đích (cần Xcode/serve-sim thật để port — chưa verify được trong sandbox) |
| Phase 3 — Backend-go `AgentKind` | [SOL-EMU-005](bugs/missing-v1/solutions/SOL-EMU-005-backend-go-agent-kind.md) | [TASK-EMU-007](bugs/missing-v1/tasks/TASK-EMU-007-proto-agent-kind.md) | ✅ DONE — proto + migration + usecase filter + `go build`/`vet`/`test` xanh cho `infra-fleet-service` và mọi service import `infrafleetv1` |
| Phase 4 — Project binding & routing | [SOL-EMU-006](bugs/missing-v1/solutions/SOL-EMU-006-project-binding-and-routing.md) | [TASK-EMU-008](bugs/missing-v1/tasks/TASK-EMU-008-project-binding-mobile-emulator-agent-id.md), [TASK-EMU-009](bugs/missing-v1/tasks/TASK-EMU-009-route-emulator-channels-by-dev-server-id.md) | ✅ DONE — `mobileEmulatorAgentId` trên project + `emulator.*` wscompat routing qua `projectId → mobileEmulatorAgentId → ResolveConnection`; `go build`/`vet`/`test` xanh cho `project-service`/`git-gateway-service`/`api-gateway` |
| Phase 5 — Frontend: chọn agent + onboarding + nối remote | [SOL-EMU-007](bugs/missing-v1/solutions/SOL-EMU-007-frontend-agent-selection-ui.md) | [TASK-EMU-011](bugs/missing-v1/tasks/TASK-EMU-011-mobile-emulator-agent-onboarding-ui.md), [TASK-EMU-012](bugs/missing-v1/tasks/TASK-EMU-012-settings-agent-picker.md), [TASK-EMU-013](bugs/missing-v1/tasks/TASK-EMU-013-emulator-pane-remote-wiring.md) | ✅ DONE — `ProjectMobileEmulatorAgentSection` (chọn agent cho project), `AddDevServerDialog` mở rộng `kind` selector (đăng ký agent), `emulator.*` calls đổi sang target động + `projectId` (`{kind:'local'}` desktop mặc định không đổi); `tsc`/`vitest`/`vite build` khớp baseline. Giới hạn: backend-go's `devServer.list`/`add` wscompat chưa đọc/trả `kind` (no-op an toàn); end-to-end thật với `infra-fleet-service` chưa verify |

**`packages/dev-agent-transport/` đã được tách** (TASK-EMU-001, DONE) —
`relay-protocol.ts` + `agent-wire.ts` (~450 dòng, codec thuần: khung nhị phân
13-byte, seq/ack, keepalive) move sang package workspace mới
`orca-dev-agent-transport`, `agent/` chuyển import sang package này với zero
regression trên test suite của `agent/`. `agent-session.ts` (hardcode
dispatcher/capabilities/PTY cleanup) **không** được trích/dùng chung — đúng
ranh giới bảo mật CR-DS-009, xem
[TDD-EMU-03](tdd/v1/03-transport-reuse-analysis.md). `emulator/` giờ tự viết
`emulator-session.ts` riêng (nhỏ hơn nhiều, chỉ `capabilities: ['device']`,
không dispatcher git/fs/pty) gọi vào package này (TASK-EMU-006, DONE) —
**chạy direct-websocket mode thật** khi `ORCA_BACKEND_URL` được set, và vẫn
giữ **stdio debug mode** khi không set (để test cục bộ không cần backend).

## Cấu trúc

- `tdd/v1/` — thiết kế kỹ thuật (kiến trúc, catalog RPC `device.*`, phân tích tái sử dụng transport, deployment)
- `bugs/missing-v1/solutions/` — SOL-EMU-XXX, mỗi solution ứng với 1 phase của CR-DS-009
- `bugs/missing-v1/tasks/` — TASK-EMU-XXX, đơn vị công việc cụ thể, có Status cập nhật theo tiến độ thật
