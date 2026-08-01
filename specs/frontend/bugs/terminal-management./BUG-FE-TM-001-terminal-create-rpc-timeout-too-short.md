# BUG-FE-TM-001: Browser `terminal.create` RPC timeout quá ngắn (15s) — không đủ cho cold start

## Mức độ: MEDIUM

## Tóm tắt

`remote-runtime-pty-transport.ts` gọi `window.api.runtimeEnvironments.call()` với `timeoutMs: 15_000` (15 giây). Nhưng theo HLD, cold start flow có nhiều bước có thể mất tổng cộng hơn 15 giây:

- `SessionManager.getOrSpawnUserProcess()`: timeout 30s
- `relay:agentCall` (`pty.spawn`): timeout 30s
- Reconnect queue: timeout 20s

Nếu user chưa có User Process (cold start) + Agent cần được đợi, tổng thời gian có thể lên đến 30-60s. Browser sẽ timeout ở 15s và hiển thị lỗi dù Backend vẫn đang xử lý.

## File liên quan

- [`src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts) — Lines 247-255

## Code

```typescript
// Lines 247-254
async function callRuntime<TResult>(method: string, params?: unknown): Promise<TResult> {
  const response = await window.api.runtimeEnvironments.call({
    selector: currentRuntimeEnvironmentId,
    method,
    params,
    timeoutMs: 15_000  // ← 15s quá ngắn cho cold start
  })
  return unwrapRuntimeRpcResult(response as RuntimeRpcResponse<TResult>)
}
```

## Ảnh hưởng

Cold start scenario:
1. User mở terminal lần đầu sau khi login → cold User Process start (tối đa 30s)
2. Browser timeout 15s → hiện thông báo lỗi
3. Thực ra Backend vẫn đang spawn → PTY được tạo nhưng Browser không nhận được handle
4. PTY zombie trên Dev Server (không có client attach)

## Cách fix đề xuất

Tăng timeout lên 60s cho `terminal.create` và `terminal.subscribe`, hoặc implement progress indicator:

```typescript
// Riêng cho terminal.create, dùng timeout dài hơn
const response = await window.api.runtimeEnvironments.call({
  selector: currentRuntimeEnvironmentId,
  method: 'terminal.create',
  params,
  timeoutMs: 60_000  // Đủ cho cold start
})
```

Hoặc tốt hơn: implement retry với exponential backoff và loading state.

## Liên quan đến luồng

- **BL-TM-01**: Browser bước — `callRuntime('terminal.create', ...)`.
- **Trace span**: `wsSession:route` `FAIL 'spawn timeout 30s'` — vẫn đang retry nhưng Browser đã timeout.
