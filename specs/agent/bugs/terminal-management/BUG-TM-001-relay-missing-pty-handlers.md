# BUG-TM-001 [AGENT]: relay dispatch thiếu handler `pty.create` — BL-TM-01 không thể tạo terminal trên Dev Server

## Mức độ: 🔴 CRITICAL

## Tóm tắt

Flow BL-TM-01 yêu cầu Orca Server gọi `relay.call('pty.create', {...})` đến Dev Server để tạo PTY session.

Kiểm tra `src/relay/agent-rpc-dispatch.ts`:
```
case 'git.exec':         ✅
case 'git.execStream':   ✅
case 'fs.readDir':       ✅
case 'fs.readFile':      ✅
case 'agent.spawn':      ✅
case 'shell.eval':       ✅
```

**`case 'pty.create'` → KHÔNG TỒN TẠI trong relay dispatch.**

Relay dispatch trên Dev Server không có handler cho:
- `pty.create` (tạo PTY)
- `pty.destroy` (đóng PTY)
- `pty.resize` (resize terminal)
- `pty.scrollback` (lấy scrollback buffer)
- `pty.write` (gửi input vào PTY)

Terminal trên remote Dev Server **hoàn toàn không thể hoạt động**.

## Root Cause

`src/relay/agent-rpc-dispatch.ts` chỉ xử lý agent/git/fs operations, không có PTY management handlers. PTY chỉ được tạo bởi `agent.spawn` (spawn agent process trong PTY), nhưng không có generic PTY creation cho shell terminal.

## Thực tế

Hệ thống thực tế dùng `agent.spawn` với shell command để tạo "terminal". Browser kết nối qua `remote-runtime-pty-transport.ts` nhưng không phải qua relay.call — mà qua một protocol khác (Runtime RPC qua WebSocket trực tiếp từ Orca Server).

## Fix đề xuất

Thêm PTY management handlers vào relay dispatch:
```typescript
// src/relay/agent-rpc-dispatch.ts
case 'pty.create': {
  const { cwd, cols, rows, env } = rpc.params
  const pty = ptyManager.create({ cwd, cols, rows, env })
  return makeOk(rpc.id, { ptyId: pty.id })
}
case 'pty.write': {
  const { ptyId, data } = rpc.params
  ptyManager.write(ptyId, data)
  return makeOk(rpc.id, {})
}
case 'pty.resize': { ... }
case 'pty.destroy': { ... }
case 'pty.scrollback': { ... }
```

## Files liên quan

- `src/relay/agent-rpc-dispatch.ts`: thiếu pty.* handlers
- `src/relay/agent-spawner.ts`: chỉ có agent spawn, không có generic PTY
- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`: browser transport

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** pty-agent-bridge.ts: PtyAgentBridge class với pty.create/write/resize/destroy/scrollback handlers. agent-rpc-dispatch.ts routes these methods via bridge.
