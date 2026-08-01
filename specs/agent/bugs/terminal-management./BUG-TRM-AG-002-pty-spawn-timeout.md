# BUG-TRM-AG-002 — Relay Agent PTY Spawn Timeout

**ID:** BUG-TRM-AG-002
**Mức độ:** 🟠 High
**Module:** `relay/pty-handler.ts` — PTY spawn trên remote host
**Phát hiện:** 2026-07-31
**Status:** 🔴 Open

---

## Mô Tả

Khi Dev Server Agent đã kết nối thành công (session không null), `DevServerRelayBridge.callWithTimeout('pty.spawn', params, 30_000)` được gọi. Tuy nhiên relay agent trên remote host bị **treo hoặc crash** trong quá trình `node-pty.spawn()`, khiến request timeout sau 30 giây. Terminal pane hiển thị loading vô thời hạn cho đến khi timeout.

---

## Root Cause

**`relay/pty-handler.ts`** — PTY spawn thực sự chạy trên remote host:

```typescript
// relay/pty-handler.ts
const ptyModule = await loadPty()  // dynamic import node-pty
if (!ptyModule) {
  // node-pty không có trên remote → không có error response rõ ràng
  throw new Error('node-pty not available')
}

const pty = ptyModule.spawn(shell, [], {
  name: 'xterm-256color',
  cols, rows, cwd, env
})
```

**Các tình huống gây timeout:**

1. **`node-pty` chưa được cài** trên remote host → `loadPty()` trả về `null`
2. **`cwd` không tồn tại** → `node-pty.spawn()` throw nhưng error không được propagate về Orca đúng cách
3. **Shell binary không tồn tại** (e.g., `/bin/zsh` trên Linux không có zsh) → spawn hang
4. **Remote host hết resource** (CPU/RAM) → spawn timeout sau 30s

**Timeout handler tại [`dev-server-relay-bridge.ts:554-558`](../../../../src/main/dev-server/dev-server-relay-bridge.ts):**

```typescript
const timer = setTimeout(
  () => {
    reject(new Error(`Relay call 'pty.spawn' timed out after ${timeoutMs}ms`))
  },
  timeoutMs  // 30_000ms
)
```

30 giây là thời gian user phải chờ mà không biết lý do.

---

## Tái Hiện

**Scenario 1 — node-pty không có:**
1. Remote host thiếu native addon `node-pty` (chưa `npm install` hoặc rebuild thất bại)
2. Mở Terminal → spinner mãi → sau 30s: lỗi timeout

**Scenario 2 — CWD không tồn tại:**
1. Worktree path `/home/ubuntu/test-repo` đã bị xóa trên remote
2. `terminal.create` gửi `cwd: '/home/ubuntu/test-repo'`
3. `node-pty.spawn` fail ngay nhưng error không propagate clean

**Trace log:**
```
[TRACE] relay:agentCall devServerId=<id> method=pty.spawn → FAIL 'Relay call timed out after 30000ms'
```

---

## Hậu Quả

- User chờ **30 giây** không phản hồi trước khi thấy lỗi
- Terminal pane không có visual feedback trong thời gian chờ
- Không có thông tin debug (không biết spawn fail ở đâu trên remote)

---

## Fix Đề Xuất

### Phương án A — Giảm timeout + fail fast

```typescript
// dev-server-relay-bridge.ts
callWithTimeout('pty.spawn', params, 10_000)  // 10s thay vì 30s
```

### Phương án B — Agent gửi error response ngay khi PTY fail

```typescript
// relay/pty-handler.ts — cần thêm explicit error handling
try {
  const ptyModule = await loadPty()
  if (!ptyModule) {
    throw Object.assign(new Error('node_pty_unavailable'), {
      hint: 'node-pty native addon not installed on remote host'
    })
  }
  // kiểm tra cwd tồn tại trước khi spawn
  if (!existsSync(cwd)) {
    throw Object.assign(new Error('pty_cwd_not_found'), { cwd })
  }
  const pty = ptyModule.spawn(shell, [], { name: 'xterm-256color', cols, rows, cwd, env })
  // ...
} catch (err) {
  // propagate error về Orca ngay lập tức
  return { error: err.message, code: err.code }
}
```

### Phương án C — Preflight check trước khi spawn

Gọi `pty.preflight` để kiểm tra node-pty availability và cwd trước khi spawn thực sự.

---

## Files Liên Quan

| File | Vị trí | Vai trò |
|------|--------|---------|
| [`relay/pty-handler.ts`](../../../../src/relay/pty-handler.ts) | `loadPty()`, `pty.spawn` handler | Bug location — PTY spawn trên remote |
| [`dev-server-relay-bridge.ts`](../../../../src/main/dev-server/dev-server-relay-bridge.ts) | `callWithTimeout()` L513-590 | 30s timeout guard |
| [`relay/pty-shell-launch.ts`](../../../../src/relay/pty-shell-launch.ts) | shell resolution | Shell binary resolution |

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** dev-server-relay-bridge.ts: PTY spawn timeout reduced to 10s (from 30s). calcBackoffDelay exponential backoff 1s→30s for reconnect.
