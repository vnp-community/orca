# Terminal Bug Index — Kết quả rà soát Terminal Create Flow

> **Cơ sở:** Đối chiếu [`terminal-create-flow.md`](../docs/flows/code/terminal-management/terminal-create-flow.md) với source code thực tế trong `src/`.  
> **Ngày rà soát:** 2026-08-01  
> **Phạm vi:** Toàn bộ luồng `terminal.create` — Browser → Backend → Dev Server Agent

---

## Tổng quan

| ID | Thành phần | Mức độ | Tóm tắt |
|----|-----------|--------|---------|
| [BUG-BE-TM-001](backend/bugs/terminals/BUG-BE-TM-001-ws-session-router-binary-frame-forwarding.md) | Backend | 🔴 HIGH | WsSessionRouter corrupt binary frame khi forward về Browser |
| [BUG-BE-TM-002](backend/bugs/terminals/BUG-BE-TM-002-ws-session-router-keepalive-corrupts-unix-socket.md) | Backend | 🟡 MEDIUM | Keepalive `\n` corrupt JSON-RPC stream trên Unix socket |
| [BUG-BE-TM-003](backend/bugs/terminals/BUG-BE-TM-003-session-manager-auth-token-injection-diverges-from-hld.md) | Backend | 🟡 MEDIUM | Auth token injection cơ chế khác HLD — magic string hack |
| [BUG-BE-TM-004](backend/bugs/terminals/BUG-BE-TM-004-agent-ws-server-port-mismatch-hld.md) | Backend | 🔴 HIGH | Agent WS server dùng port 6769 thay vì 6768/agent (HLD) |
| [BUG-BE-TM-005](backend/bugs/terminals/BUG-BE-TM-005-direct-ws-missing-disconnect-handler.md) | Backend | 🔴 HIGH | directWebSocket mode không xử lý disconnect — session leak |
| [BUG-BE-TM-006](backend/bugs/terminals/BUG-BE-TM-006-terminal-create-missing-rbac-check.md) | Backend | 🟡 MEDIUM | `terminal.create` thiếu RBAC `checkScopedTokenPermission` |
| [BUG-AG-TM-001](agent/bugs/terminals/BUG-AG-TM-001-pty-spawn-missing-context-verifier.md) | Agent | 🔴 HIGH | `pty.spawn` thiếu HMAC-SHA256 ContextVerifier |
| [BUG-AG-TM-002](agent/bugs/terminals/BUG-AG-TM-002-pty-spawn-missing-securefs-path-validation.md) | Agent | 🔴 HIGH | `pty.spawn` thiếu SecureFs path traversal validation |
| [BUG-AG-TM-003](agent/bugs/terminals/BUG-AG-TM-003-pty-spawn-shell-resolve-ignores-env-shell.md) | Agent | 🟡 MEDIUM | Shell resolve dùng relay process $SHELL, không phải env.SHELL từ Backend |
| [BUG-AG-TM-004](agent/bugs/terminals/BUG-AG-TM-004-pty-spawn-response-missing-fields.md) | Agent | 🟡 MEDIUM | `pty.spawn` response thiếu `handle`, `cols`, `rows`, `cwd` |
| [BUG-FE-TM-001](frontend/bugs/terminal/BUG-FE-TM-001-terminal-create-rpc-timeout-too-short.md) | Frontend | 🟡 MEDIUM | RPC timeout 15s quá ngắn cho cold start (30-60s) |
| [BUG-FE-TM-002](frontend/bugs/terminal/BUG-FE-TM-002-browser-missing-scrollback-snapshot-save-restore.md) | Frontend | 🔴 HIGH | Thiếu `terminal.snapshot.save/restore` — BL-TM-03 không hoạt động |
| [BUG-FE-TM-003](frontend/bugs/terminal/BUG-FE-TM-003-terminal-create-hardcoded-background-presentation.md) | Frontend | 🟢 LOW | `presentation` hardcode `'background'` bỏ qua focused intent |
| [BUG-FE-TM-004](frontend/bugs/terminal/BUG-FE-TM-004-default-viewport-size-mismatch.md) | Frontend | 🟢 LOW | Default viewport `80x24` không match HLD `120x40` |

---

## Phân loại theo mức độ

### 🔴 HIGH (6 bugs) — Cần fix sớm

| ID | Thành phần | Ảnh hưởng |
|----|-----------|---------|
| BUG-BE-TM-001 | Backend | Binary frame corruption → Terminal hiển thị garbage |
| BUG-BE-TM-004 | Backend | Port mismatch → Agent không connect được |
| BUG-BE-TM-005 | Backend | Session leak sau disconnect → Terminal zombie |
| BUG-AG-TM-001 | Agent | Thiếu HMAC verification → Security gap |
| BUG-AG-TM-002 | Agent | Thiếu path validation → Path traversal vulnerability |
| BUG-FE-TM-002 | Frontend | Scrollback không lưu → history mất khi đóng tab |

### 🟡 MEDIUM (5 bugs) — Nên fix trong sprint

| ID | Thành phần | Ảnh hưởng |
|----|-----------|---------|
| BUG-BE-TM-002 | Backend | Keepalive corrupt stream mỗi 15s |
| BUG-BE-TM-003 | Backend | Auth token mechanism không match HLD |
| BUG-BE-TM-006 | Backend | Thiếu RBAC → unauthorized terminal creation |
| BUG-AG-TM-003 | Agent | Shell resolve sai → wrong shell spawned |
| BUG-AG-TM-004 | Agent | Response thiếu cols/rows → viewport sync issue |
| BUG-FE-TM-001 | Frontend | Timeout quá ngắn → false error on cold start |

### 🟢 LOW (2 bugs) — Technical debt

| ID | Thành phần | Ảnh hưởng |
|----|-----------|---------|
| BUG-FE-TM-003 | Frontend | presentation mode không propagate đúng |
| BUG-FE-TM-004 | Frontend | Default viewport size không match HLD |

---

## Mapping với Business Logic

| Business Rule | Bugs liên quan |
|---|---|
| **BL-TM-01** Tạo PTY Session | BUG-BE-TM-001, 004, 005, 006; BUG-AG-TM-001, 002, 003, 004 |
| **BL-TM-03** Scrollback Persistence | BUG-FE-TM-002 |
| **BL-TM-04** Shell Integration (OSC 133) | BUG-AG-TM-003 (shell resolve sai) |
| **BR-TM-01** Cleanup on close | BUG-BE-TM-005 (session leak) |
| **BR-TM-02** Resize propagation | BUG-FE-TM-004 (default viewport) |
| **BR-TM-04** Shell path resolve | BUG-AG-TM-003 |

---

## Files đã rà soát

| File | Thành phần | Bugs tìm thấy |
|------|-----------|--------------|
| `src/main/session/ws-session-router.ts` | Backend | BUG-BE-TM-001, 002, 003 |
| `src/main/session/session-manager.ts` | Backend | BUG-BE-TM-003 |
| `src/main/dev-server/agent-ws-server.ts` | Backend | BUG-BE-TM-004 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | Backend | BUG-BE-TM-004, 005 |
| `src/main/runtime/rpc/methods/terminal.ts` | Backend | BUG-BE-TM-006 |
| `src/relay/pty-handler.ts` | Agent | BUG-AG-TM-001, 002, 003, 004 |
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | Frontend | BUG-FE-TM-001, 002, 003, 004 |
