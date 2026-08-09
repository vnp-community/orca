# SOLUTION: BUG-FE-HLD-002 — `git.push` streaming dùng `sessionStorage` Bearer token

**Source-verified:** ✅ Dựa trên source code thực tế
**TDD tham chiếu:** [tdd/v5/03-runtime-client-layer.md](../../../tdd/v5/03-runtime-client-layer.md) §8 "Runtime Git Client" — thiết kế chính thức đã có sẵn `runtimeGitPush(target, repoPath, args)` đi qua `RuntimeClientTarget` (`{kind:'local'}` Electron IPC hoặc `{kind:'environment', environmentId}` WebSocket), **không phải** 1 kênh HTTP stream riêng biệt với header tự chế. `runtime-rpc-stream.ts` không nằm trong TDD — đây là code phát sinh ngoài kiến trúc đã thiết kế.

---

## Root cause

`useGit.ts` gọi `callRuntimeRpcStream('git.push', ...)` → `runtime-rpc-stream.ts`'s `webStream()`, tự dựng 1 luồng `fetch('/api/rpc/stream', { headers: { Authorization: 'Bearer ' + sessionStorage.getItem('orca_session_token') } })` — 1 transport **song song, không liên quan** tới `runtime-rpc-client.ts`/`callRuntimeRpc()` đã thiết kế trong TDD. Vì không có nơi nào set `orca_session_token`, header luôn gửi rỗng.

## Fix — route qua transport đã thiết kế, không phát minh thêm

### Bước 1: Xác nhận `WebRuntimeClient`/`WebSessionClient` đã hỗ trợ streaming response

Theo audit code trước đó (`web-runtime-client.ts`), class này đã có cơ chế `subscribe()` cho streaming (dùng bởi `files.watch`, PTY output) — dùng đúng `deviceToken`/cookie đã xác thực của kết nối WS hiện có, **không cần** 1 HTTP endpoint + auth riêng.

**File cần sửa:** `frontend/src/renderer/src/runtime/runtime-git-client.ts` (theo TDD §8, hàm `runtimeGitPush` nên tồn tại ở đây — nếu chưa implement streaming progress, đây là nơi thêm, không phải file `runtime-rpc-stream.ts` mới).

```ts
// Why: git.push progress must ride the same authenticated WS/IPC channel as
// every other runtime RPC (TDD-FE-03 §2/§8) — a parallel HTTP+Bearer channel
// duplicates auth state and, as BUG-FE-HLD-002 showed, silently drifts out of
// sync with how the rest of the app authenticates.
export async function runtimeGitPush(
  target: RuntimeClientTarget,
  repoPath: string,
  args: GitPushArgs,
  onProgress?: (chunk: GitPushProgressChunk) => void
): Promise<void> {
  // callRuntimeRpc() already resolves target → Electron IPC | WebRuntimeClient | WebSessionClient
  // and already carries the correct auth (deviceToken or cookie) for that target.
  await callRuntimeRpcStream(target, 'git.push', { repoPath, ...args }, onProgress)
}
```

Nếu `callRuntimeRpcStream` (biến thể streaming của `callRuntimeRpc`) chưa tồn tại trong `runtime-rpc-client.ts`, thêm nó theo đúng pattern `subscribe()` đã có ở `WebRuntimeClient`:

```ts
// runtime-rpc-client.ts
export async function callRuntimeRpcStream<TChunk>(
  target: RuntimeClientTarget,
  method: string,
  params: unknown,
  onChunk: (chunk: TChunk) => void
): Promise<void> {
  if (target.kind === 'local') {
    // Electron IPC streaming — dùng window.api.runtimeEnvironments.call() với callback đã có
  } else {
    const client = getWebClientForEnvironment(target.environmentId) // WebRuntimeClient | WebSessionClient
    const handle = await client.subscribe(method, params, {
      onResponse: (r) => { if (r.ok) onChunk(r.result as TChunk) }
    })
    // caller unsubscribe khi push xong (response type 'end')
  }
}
```

### Bước 2: Xoá `runtime-rpc-stream.ts`

Xoá file cùng với `getSessionToken()`/`sessionStorage.getItem('orca_session_token')` — không còn nơi nào cần đọc token thủ công vì `client.subscribe()` đã tự đính kèm đúng `deviceToken`/cookie của chính kết nối đang mở.

### Bước 3: `useGit.ts`

```diff
- import { callRuntimeRpcStream } from '../runtime/runtime-rpc-stream'
+ import { runtimeGitPush } from '../runtime/runtime-git-client'
  ...
- await callRuntimeRpcStream('git.push', { repoPath, ...args })
+ await runtimeGitPush(target, repoPath, args, onProgress)
```

## Test cần thêm

- `runtime-git-client.test.ts`: `runtimeGitPush` với `target.kind === 'environment'` gọi đúng `client.subscribe('git.push', ...)`, không tham chiếu `sessionStorage`/`Authorization` header nào.
- `useGit.test.ts`: xác nhận UI hiển thị progress đúng khi `onChunk` được gọi nhiều lần, và lỗi network hiển thị đúng khi `subscribe` reject.
- Regression: grep toàn repo sau khi fix, xác nhận `orca_session_token` không còn xuất hiện ở bất kỳ đâu.

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `runtime/runtime-rpc-stream.ts` | **Xoá** |
| `runtime/runtime-git-client.ts` | Thêm/hoàn thiện `runtimeGitPush()` dùng `callRuntimeRpcStream` qua `RuntimeClientTarget` |
| `runtime/runtime-rpc-client.ts` | Thêm `callRuntimeRpcStream()` nếu chưa có, tái dùng `client.subscribe()` |
| `hooks/useGit.ts` | Đổi import, gọi `runtimeGitPush` thay vì `callRuntimeRpcStream('git.push', ...)` trực tiếp |
