# Bug Reports — Terminal (Agent Side)

**Module:** `src/relay/pty-handler.ts` + `relay/agent-entry.ts` + `DevServerRelayBridge`
**Phát hiện:** 2026-07-31
**Phiên bản Orca:** b15.openledger.vn (production)
**Ngữ cảnh:** Phân tích code review luồng `terminal.create` trên project `test-repo`

---

## Danh Sách Bugs

| ID | Mức độ | Tiêu đề | Files liên quan | Status |
|----|--------|---------|-----------------|--------|
| [BUG-TRM-AG-001](./BUG-TRM-AG-001-relay-session-null.md) | 🔴 Critical | Dev server agent chưa kết nối → `relay session null` khi terminal.create | `dev-server-relay-bridge.ts`, `agent-ws-server.ts` | 🔴 Open |
| [BUG-TRM-AG-002](./BUG-TRM-AG-002-pty-spawn-timeout.md) | 🟠 High | Relay agent bị treo / crash → `pty.spawn` timeout 30s | `relay/pty-handler.ts`, `dev-server-relay-bridge.ts` | 🔴 Open |

---

## Phân Loại theo Priority

### 🔴 Critical — Chặn tính năng Terminal hoàn toàn
- **BUG-TRM-AG-001**: Agent chưa connect inbound → mọi terminal.create đều fail với "Not connected"

### 🟠 High — Terminal treo / timeout
- **BUG-TRM-AG-002**: node-pty hang trên remote host → user thấy terminal loading mãi không dừng

---

## Tham Khảo

- [Terminal Create Flow](../../../../docs/flows/terminal-create-flow.md)
- [Agent Connection Modes Flow](../../../../docs/flows/agent-connection-modes.md)
- [Dev Server Connection Types](../../../../docs/flows/dev-server-connection-types.md)
