# BUG-BE-SSH-001: SSH Auto-Reconnect thiếu exponential backoff và `ssh:unreachable` alert sau 5 lần thất bại

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-SSH-001  
**Note:** dev-server-relay-bridge.ts: exponential backoff 2s→60s with jitter  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-SSH-03) mô tả:
```
[SshManager.handleDisconnect()]
    Start reconnect loop:
        attempt 1: wait 2s → ssh2.connect()
        attempt 2: wait 4s → ssh2.connect()
        attempt 3: wait 8s → ssh2.connect()
        ...exponential backoff, max 60s
    Nếu fail 5 lần: emit: ssh:unreachable → alert user
```

Nhưng `SshRelaySession.reconnect()` thực tế:
```typescript
// ssh-relay-session.ts:438-570
async reconnect(conn: SshConnection, graceTimeSeconds?: number): Promise<void> {
  if (this._state !== 'ready' && this._state !== 'reconnecting') { return }
  this.abortController?.abort()
  this._state = 'reconnecting'
  // ...deployAndLaunchRelay() (single attempt)
  // Không có retry loop
  // Không có exponential backoff
  // Không có attempt counter
  // Không có 'ssh:unreachable' emit
}
```

Grep toàn bộ `src/main/ssh/`:
```
exponential.*backoff  → No results
backoff               → No results
attempt.*count        → No results  
ssh:unreachable       → No results
max.*attempt          → No results
wait.*2s              → No results
```

`reconnect()` chỉ thực hiện **1 lần deploy/connect** — nếu fail sẽ throw, không retry. Caller (bên ngoài trong `ssh.ts`) phải trigger lại `session.reconnect()` mỗi lần SSH connection reconnect.

## File liên quan

- [`src/main/ssh/ssh-relay-session.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/ssh/ssh-relay-session.ts) — Lines 438-570: `reconnect()` method

## Ảnh hưởng

1. Khi SSH connection drops và relay reconnect fails → không có retry với backoff → user thấy "disconnected" ngay lập tức.
2. Không có alert "Server unreachable" sau 5 lần thất bại.
3. Nếu network flap ngắn (< 2s) nhưng relay chưa sẵn sàng → reconnect có thể fail mà không retry.
4. HLD backoff behavior (2s, 4s, 8s, max 60s) không được implement.

## Lưu ý

SSH connection bản thân có thể tự retry (phụ thuộc vào `ssh2` hoặc higher-level ssh.ts logic), nhưng relay reconnect (deploy + connect relay) **không có internal retry**.

## Liên quan đến luồng

- **BL-SSH-03**: SSH Auto-Reconnect — backoff và unreachable alert missing.
