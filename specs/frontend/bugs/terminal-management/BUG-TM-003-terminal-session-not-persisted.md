# BUG-TM-003 [FRONTEND]: `remote-runtime-pty-transport.ts` — Terminal session không persist khi reconnect (không có sessionId in DB)

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts:83-88`:
```typescript
let connected = false
let destroyed = false
let handle: string | null = null
let remotePtyId: string | null = null
// ...
```

Terminal session state chỉ được lưu **in-memory** ở browser side. Khi browser reload hoặc WebSocket reconnect:
1. `remotePtyId` bị mất
2. Browser không thể reattach vào PTY cũ trên Dev Server
3. PTY process tiếp tục chạy trên Dev Server (resource leak) nhưng browser không kết nối được

BL-TM-03 (Scrollback) yêu cầu `orca_terminal_sessions` lưu `ptyId` để restore. Nhưng không có cơ chế lưu session vào DB từ Browser side.

## Ảnh hưởng

1. Browser refresh → mất terminal session (mất lịch sử output)
2. Orphan PTY processes trên Dev Server (resource leak)
3. Scrollback restore (BL-TM-03) không có trigger — ai gọi `orca_terminal_sessions` INSERT?

## Root Cause

Flow BL-TM-01 yêu cầu Orca Server INSERT `orca_terminal_sessions` sau khi `relay.call('pty.create')` nhưng:
1. Không có `pty.create` relay handler (BUG-TM-001)
2. Không có INSERT `orca_terminal_sessions` trong `WorkspaceService`

## Fix đề xuất

Orca Server phải:
1. Nhận `terminal.create` RPC từ Browser
2. `relay.call('pty.create')` → Dev Server
3. INSERT `orca_terminal_sessions { ptyId, userId, projectId }` → DB
4. Return `{ ptyId, sessionId }` → Browser

Browser lưu `sessionId` và dùng để reattach sau reconnect.

## Files liên quan

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`: in-memory only state
- `src/main/workspace/WorkspaceService.ts`: thiếu orca_terminal_sessions INSERT
