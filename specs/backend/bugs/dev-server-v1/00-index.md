# Bug Reports — Dev Server v1 (Agent Integration)

**Module:** `src/main/dev-server/` + `deploy/dev/agent/agent.js`  
**Phát hiện:** 2026-07-26  
**Phiên bản Orca:** 1.4.138  
**Ngữ cảnh:** Phân tích sau khi triển khai agent direct-websocket trên dev server (172.20.2.31)

---

## Danh Sách Bugs

| ID | Mức độ | Tiêu đề | Files liên quan | Status |
|----|--------|---------|-----------------|--------|
| [BUG-DS-001](./BUG-DS-001-relay-ws-handshake-token.md) | 🔴 Critical | relay-websocket handshake token không khớp | `agent.js`, `ws-handshake.ts`, `dev-server-relay-bridge.ts` | 🔴 Open |
| [BUG-DS-002](./BUG-DS-002-rpc-method-mismatch.md) | 🔴 Critical | Agent không implement relay RPC methods | `agent.js`, `onboarding-ipc.ts`, `repo-remote-ipc.ts` | 🔴 Open |
| [BUG-DS-003](./BUG-DS-003-orca-url-literal.md) | 🟠 High | `orcaUrl` emit với literal `<orca-host>` | `dev-server-relay-bridge.ts` | 🔴 Open |
| [BUG-DS-004](./BUG-DS-004-inmemory-state-lost-on-restart.md) | 🟠 High | In-memory state mất sau Orca Server restart | `dev-server-manager.ts` | 🔴 Open |
| [BUG-DS-005](./BUG-DS-005-relay-ws-no-reconnect.md) | 🟠 High | relay-websocket không có auto-reconnect | `dev-server-relay-bridge.ts` | 🔴 Open |
| [BUG-DS-006](./BUG-DS-006-curl-timeout-conflict.md) | 🟡 Low | curl trong start.sh không có `--max-time`, xung đột TimeoutStopSec | `start.sh`, `start-agent-direct.sh` | 🔴 Open |
| [BUG-DS-007](./BUG-DS-007-service-file-inconsistency.md) | 🟡 Low | 2 systemd service files không đồng bộ | `orca-agent.service`, `start-agent-direct.sh` | 🔴 Open |
| [BUG-DS-008](./BUG-DS-008-keepalive-margin.md) | 🟡 Low | Keepalive interval 8s, server timeout 20s — margin mỏng | `agent.js`, `relay-protocol.ts` | 🔴 Open |

---

## Phân Loại theo Priority

### 🔴 Critical — Chặn tính năng core
- **BUG-DS-001**: relay-websocket mode hoàn toàn không kết nối được
- **BUG-DS-002**: Onboarding flow, remote workspace, terminal — tất cả fail

### 🟠 High — Ảnh hưởng UX nghiêm trọng
- **BUG-DS-003**: URL display sai cho user setup thủ công
- **BUG-DS-004**: UI không phản ánh agent state sau server restart
- **BUG-DS-005**: relay-ws cần thao tác thủ công sau mỗi Orca restart

### 🟡 Low — Technical debt / Edge cases
- **BUG-DS-006**: Token leak tiềm ẩn khi curl bị kill bởi systemd
- **BUG-DS-007**: Log ở 2 chỗ khác nhau, khó debug
- **BUG-DS-008**: Tiềm ẩn timeout trên high-latency connection

---

## Tham Khảo

- [Agent Connection Modes Flow](../../../../docs/flows/agent-connection-modes.md)
- [SOL-AG-004 direct-websocket](../crs/v2/agent/solutions/SOL-AG-004-direct-websocket.md)
- [SOL-AG-003 relay-websocket](../crs/v2/agent/solutions/SOL-AG-003-relay-websocket.md)
- [SOL-AG-002 ws-transport](../crs/v2/agent/solutions/SOL-AG-002-ws-transport-adapter.md)
