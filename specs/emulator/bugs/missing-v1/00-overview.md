# Mobile Emulator Agent — Missing-v1 Overview

Mirrors `specs/backend-go/bugs/missing-v1/` convention: "missing-v1" nghĩa là
chức năng **chưa tồn tại** (không phải bug trong code sẵn có) — SOL/TASK ở
đây theo dõi việc xây `emulator/` từ đầu theo
[CR-DS-009](../../../../docs/crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md).

Xem bảng trạng thái đầy đủ tại [../../README.md](../../README.md).

| SOL | Task(s) | Trạng thái |
|---|---|---|
| [SOL-EMU-001](solutions/SOL-EMU-001-shared-transport-extraction.md) | [TASK-EMU-001](tasks/TASK-EMU-001-extract-dev-agent-transport-package.md) | ✅ DONE |
| [SOL-EMU-002](solutions/SOL-EMU-002-emulator-package-scaffold.md) | [TASK-EMU-002](tasks/TASK-EMU-002-scaffold-emulator-workspace.md) | ✅ DONE |
| [SOL-EMU-003](solutions/SOL-EMU-003-device-capability-and-list-handlers.md) | [TASK-EMU-003](tasks/TASK-EMU-003-device-capabilities-and-list-handlers.md), [TASK-EMU-004](tasks/TASK-EMU-004-emulator-rpc-dispatch.md), [TASK-EMU-005](tasks/TASK-EMU-005-emulator-entry-stdio-debug-mode.md) | ✅ DONE |
| — | [TASK-EMU-006](tasks/TASK-EMU-006-wire-protocol-transport-integration.md) | ✅ DONE (direct-websocket mode; end-to-end với `infra-fleet-service` thật chưa verify — xem task doc) |
| [SOL-EMU-004](solutions/SOL-EMU-004-device-control-handlers.md) | [TASK-EMU-010](tasks/TASK-EMU-010-device-control-handlers-port.md) | ✅ DONE cho Android (adb thật); iOS honest-stub có chủ đích |
| [SOL-EMU-005](solutions/SOL-EMU-005-backend-go-agent-kind.md) | [TASK-EMU-007](tasks/TASK-EMU-007-proto-agent-kind.md) | ✅ DONE |
| [SOL-EMU-006](solutions/SOL-EMU-006-project-binding-and-routing.md) | [TASK-EMU-008](tasks/TASK-EMU-008-project-binding-mobile-emulator-agent-id.md), [TASK-EMU-009](tasks/TASK-EMU-009-route-emulator-channels-by-dev-server-id.md) | ✅ DONE |
| [SOL-EMU-007](solutions/SOL-EMU-007-frontend-agent-selection-ui.md) | [TASK-EMU-011](tasks/TASK-EMU-011-mobile-emulator-agent-onboarding-ui.md), [TASK-EMU-012](tasks/TASK-EMU-012-settings-agent-picker.md), [TASK-EMU-013](tasks/TASK-EMU-013-emulator-pane-remote-wiring.md) | ✅ DONE |
