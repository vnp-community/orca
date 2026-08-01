# BUG-AG-ORCH-011: PTY_REGISTRY module-level singleton → orphaned PTYs khi WS disconnect

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`agent-spawner.ts:50`:
```typescript
const PTY_REGISTRY = new Map<string, {
  pty:    nodePty.IPty
  taskId: string
  userId: string
}>()
```

Registry này là **module-level singleton** (tồn tại global trong relay process).

## Vấn đề 1: Orphaned PTYs khi WS đóng

Khi WebSocket giữa Orca và Dev Server đóng (`ws.on('close')`), `agent-session.ts:177` chỉ gọi `session.stop()` → xóa keepalive interval. **Không có cleanup nào cho các PTYs đang running.**

Kết quả: PTY process vẫn chạy trên Dev Server nhưng output không đến được Orca (vì WS đóng). Không có cách kill chúng nếu không biết ptyId.

## Vấn đề 2: ptyId stale sau WS reconnect

Khi relay restart (do SSH reconnect hoặc relay-ws reconnect), `PTY_REGISTRY` bị xóa (process restart). Nhưng Orca server vẫn lưu ptyId từ lần connect trước → gọi `agent.kill({ ptyId })` → PTY_REGISTRY.get(ptyId) = undefined → "already dead".

## Vấn đề 3: Không có per-userId isolation

HLD mô tả `ptySessionStore[userId + worktreeId] = ptyHandle` (composite key per user).
Thực tế PTY_REGISTRY dùng `ptyId = ${req.taskId}-${Date.now()}` — không phân biệt userId.

Nếu 2 users spawn agent với cùng taskId (race condition) → collision.

## Ảnh hưởng

1. Relay process leak PTY processes khi WS disconnects
2. Dev Server resource leak (CPU/memory từ orphaned AI agent processes)
3. `agent.kill` sẽ silent fail cho stale ptyIds

## Fix đề xuất

Cleanup PTYs khi WS đóng:
```typescript
// agent-session.ts - thêm vào stop()
stop(ws: WebSocket): void {
  clearInterval(keepaliveTimer)
  // Kill all PTYs owned by this session
  for (const [ptyId, entry] of PTY_REGISTRY.entries()) {
    entry.pty.kill('SIGTERM')
    PTY_REGISTRY.delete(ptyId)
    log.info(`Session closed: killed orphaned PTY ${ptyId}`)
  }
}
```

## Liên quan đến luồng

- **BL-AG-01**: PTY lifecycle không được managed properly
- **BL-AG-02**: Stop agent - stale PTYs not cleaned up

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** PTY_REGISTRY.delete(ptyId) called on pty.onExit AND handleAgentKill. ws disconnect handler iterates PTY_REGISTRY and kills all owned PTYs.
