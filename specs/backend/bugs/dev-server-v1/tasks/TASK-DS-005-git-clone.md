# TASK-DS-005 — Implement `git.clone` (Async với execFile)

**Solution:** [SOL-DS-002](../solutions/SOL-DS-002-agent-relay-rpc-methods.md)  
**Bug:** [BUG-DS-002](../BUG-DS-002-rpc-method-mismatch.md)  
**File:** `deploy/dev/agent/agent.js`  
**Phụ thuộc:** TASK-DS-002 (helpers cần có sẵn)  
**Estimated:** 20 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm `git.clone` RPC handler vào `dispatchRpc()`. Handler này **async** (git clone có thể mất vài phút) và gửi response trực tiếp về `ws` sau khi hoàn thành, KHÔNG qua return value của dispatchRpc.

---

## Context

Đọc trước:
- `deploy/dev/agent/agent.js` — cách `dispatchRpc()` gửi response: xem variable `response` và phần cuối hàm
- `src/main/ipc/repo-remote-ipc.ts` dòng 103-125 — format: `relay.call('git.clone', { url, targetDir })` timeout 30s

**Format response:**
```javascript
// git.clone success:
{ path: string }  // đường dẫn đến thư mục đã clone

// git.clone error:
{ code: -32000, message: string }  // stderr từ git
```

> [!WARNING]
> `git clone` timeout trong `callWithTimeout` là 30s. Với repo lớn, 30s có thể không đủ. Agent cần gửi response TRƯỚC khi server timeout. Nếu cần, thêm `--depth 1` để shallow clone.

---

## Thay Đổi Cần Thực Hiện

**File:** `deploy/dev/agent/agent.js`

### Bước 1: Đảm bảo `execFile` đã được import

Tìm dòng require `child_process` (đã thêm ở TASK-DS-002):
```javascript
const { execSync: _execSync, execFile: _execFile } = require('child_process');
```
Nếu chưa có `execFile`, thêm vào.

### Bước 2: Thêm case trong `dispatchRpc()`

Thêm SAU các `fs.*` cases:

```javascript
    case 'git.clone': {
      const { url, targetDir, branch, shallow = false } = msg.params ?? {};
      if (!url || !targetDir) {
        response = {
          jsonrpc: '2.0', id: msg.id,
          error: { code: -32602, message: 'params.url and params.targetDir are required' },
        };
        break;
      }

      // git.clone là async (mất vài phút với repo lớn).
      // Gửi response sau khi clone xong, KHÔNG qua return value.
      // Đặt response = null để dispatchRpc KHÔNG tự gửi.
      response = null;

      const args = ['clone', url, targetDir];
      if (branch)  args.push('--branch', branch);
      if (shallow) args.push('--depth', '1');

      const cloneStart = Date.now();
      log.info(`git.clone start: ${url} → ${targetDir}`);

      _execFile('git', args, {
        cwd:     WORK_DIR ?? process.env.HOME ?? '/home/ubuntu',
        timeout: 300_000,  // 5 phút max (server timeout là 30s, nhưng agent không cần match)
        env:     { ...process.env, GIT_TERMINAL_PROMPT: '0' },  // không prompt password
      }, (err, _stdout, stderr) => {
        const elapsed = Date.now() - cloneStart;
        const result  = err
          ? {
              jsonrpc: '2.0', id: msg.id,
              error: { code: -32000, message: (stderr || err.message).trim() },
            }
          : {
              jsonrpc: '2.0', id: msg.id,
              result: { path: targetDir },
            };
        log.info(`git.clone ${err ? 'FAIL' : 'OK'} (${elapsed}ms): ${targetDir}`);
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify(result)));
        }
      });
      break;
    }
```

### Bước 3: Đảm bảo `dispatchRpc()` KHÔNG gửi response nếu `response = null`

Tìm phần cuối của `dispatchRpc()` — nơi gửi response. Thêm guard:

```javascript
// Cuối dispatchRpc(), trước khi gửi:
if (response !== null && ws.readyState === WebSocket.OPEN) {
  ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify(response)));
}
```

Nếu đoạn gửi response hiện tại không có guard `response !== null`, thêm điều kiện vào.

---

## Verify

```bash
# Deploy + restart
bash deploy/dev/scripts/connect-agent.sh --deploy

# Trong Orca UI: Remote Workspace → Clone Repository
# Nhập URL + target dir → Clone
# Expected: clone starts, kết quả hiện sau khi hoàn thành ✅

# Verify qua log:
bash deploy/dev/scripts/connect-agent.sh --logs | grep "git.clone"
# Expected: "git.clone start: ..." rồi "git.clone OK (Xms)"
```

---

## Definition of Done

- [x] `case 'git.clone'` gửi response SAU khi `execFile` callback (async)
- [x] `response = null` ngăn dispatchRpc gửi response ngay
- [x] `GIT_TERMINAL_PROMPT=0` tránh git prompt password (block vô hạn)
- [x] Error case trả về stderr message rõ ràng
- [x] `shallow` param support (`--depth 1`) cho repo lớn
- [x] Guard `response !== null && response !== undefined` được thêm để tránh double-send
- [x] agent.js syntax check OK (`node --check`)
