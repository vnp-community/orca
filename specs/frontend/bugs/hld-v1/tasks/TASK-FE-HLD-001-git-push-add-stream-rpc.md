# TASK-FE-HLD-001 — Thêm `callRuntimeRpcStream()` vào runtime RPC client

**Solution:** [SOLUTION-FE-HLD-002](../solutions/SOLUTION-FE-HLD-002-git-push-stream-auth.md)
**Bug:** [BUG-FE-HLD-002](../BUG-FE-HLD-002-git-push-stream-bearer-token-broken.md)
**File:** `frontend/src/renderer/src/runtime/runtime-rpc-client.ts`
**Estimated:** 30 phút
**Status:** ❌ KHÔNG LÀM — kế hoạch gốc sai, xem "Phát hiện" bên dưới. Fix thật nằm ở TASK-FE-HLD-003.

---

## Mục tiêu

Thêm hàm `callRuntimeRpcStream()` vào `runtime-rpc-client.ts`, tái dùng đúng `RuntimeClientTarget` (`{kind:'local'}` | `{kind:'environment', environmentId}`) và cơ chế `subscribe()` đã có trên `WebRuntimeClient`/`WebSessionClient` — đây là bước nền tảng để TASK-FE-HLD-002/003 route `git.push` streaming qua kênh RPC đã xác thực đúng, thay vì kênh HTTP+Bearer tự chế đang bị BUG-FE-HLD-002.

---

## Context

Đọc trước:
- `frontend/src/renderer/src/runtime/runtime-rpc-client.ts` — hàm `callRuntimeRpc()` hiện có, cách resolve `RuntimeClientTarget` → Electron IPC hoặc web client
- `frontend/src/renderer/src/web/web-runtime-client.ts` — hàm `subscribe()` (đã có, dùng bởi `files.watch`) — mẫu để tái dùng, không viết lại
- `frontend/src/renderer/src/web/web-preload-api.ts` — hàm lấy client hiện tại theo `environmentId` (tên hàm thật cần xác nhận qua grep, gọi tạm là `getWebClientForEnvironment` trong solution)
- [specs/frontend/tdd/v5/03-runtime-client-layer.md](../../../tdd/v5/03-runtime-client-layer.md) §2, §8 — thiết kế gốc của `callRuntimeRpc`/`runtimeGitPush`

```bash
grep -n "function callRuntimeRpc\b\|RuntimeClientTarget" frontend/src/renderer/src/runtime/runtime-rpc-client.ts
grep -n "getWebClientForEnvironment\|activeClient\|getRuntimeClientForEnvironment" frontend/src/renderer/src/web/web-preload-api.ts
```

---

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/runtime/runtime-rpc-client.ts`

Thêm hàm mới cạnh `callRuntimeRpc()` hiện có (giữ nguyên `callRuntimeRpc`, không sửa):

```typescript
// Why: git.push progress (và mọi RPC streaming khác trong tương lai) phải đi
// qua cùng kênh đã xác thực (deviceToken hoặc cookie) như mọi RPC call khác —
// xem BUG-FE-HLD-002. KHÔNG tạo 1 HTTP endpoint + header tự chế riêng.
export async function callRuntimeRpcStream<TChunk>(
  target: RuntimeClientTarget,
  method: string,
  params: unknown,
  onChunk: (chunk: TChunk) => void,
  options?: { timeoutMs?: number }
): Promise<void> {
  if (target.kind === 'local') {
    // Electron IPC streaming — window.api đã hỗ trợ callback-based streaming
    // cho các method khác (vd. terminal output); tái dùng đúng cơ chế đó.
    await new Promise<void>((resolve, reject) => {
      window.api.runtimeEnvironments.call({
        method,
        params,
        onChunk: (chunk: TChunk) => onChunk(chunk),
        onDone: () => resolve(),
        onError: (err: Error) => reject(err)
      })
    })
    return
  }

  const client = getWebClientForEnvironment(target.environmentId) // đã có trong web-preload-api.ts
  return new Promise<void>((resolve, reject) => {
    void client
      .subscribe(
        method,
        params,
        {
          onResponse: (response) => {
            if (response.ok === false) {
              reject(new Error(response.error.message))
              return
            }
            onChunk(response.result as TChunk)
          },
          onError: (error) => reject(new Error(error.message)),
          onClose: () => resolve()
        },
        options
      )
      .catch(reject)
  })
}
```

> [!IMPORTANT]
> Nếu `window.api.runtimeEnvironments.call` chưa hỗ trợ tham số `onChunk`/`onDone`/`onError` (chỉ hỗ trợ request-response đơn), dừng lại và báo — cần mở rộng `preload/api-types.ts` trước, KHÔNG tự chế 1 cơ chế polling thay thế.

---

## Verify

```bash
pnpm --filter frontend tsc --noEmit
# Xác nhận không có lỗi type mới liên quan runtime-rpc-client.ts

grep -n "callRuntimeRpcStream" frontend/src/renderer/src/runtime/runtime-rpc-client.ts
# Phải thấy đúng 1 định nghĩa hàm mới
```

---

## Definition of Done — không áp dụng, xem phát hiện bên dưới

## Phát hiện khi bắt đầu thực thi — kế hoạch gốc dựa trên giả định sai

Trước khi viết code, đọc lại `backend/src/main/runtime/rpc/methods/git.ts:238-247` — RPC method `git.push` thật ở backend:

```ts
defineMethod({
  name: 'git.push',
  params: GitPush,
  handler: async (params, { runtime }) =>
    runtime.pushRuntimeGit(params.worktree, params.publish, params.pushTarget, params.forceWithLease)
})
```

Đây là **request/response đơn**, không phải streaming — không có cơ chế nào ở backend gửi nhiều "chunk" cho `git.push` (đối chiếu thêm `runtime-rpc.ts`: cơ chế streaming nhị phân (`binaryStreamHandlers`) chỉ tồn tại cho PTY/terminal, không áp dụng cho git operations).

Endpoint `/api/rpc/stream` mà `runtime-rpc-stream.ts`'s `webStream()` gọi tới (nguồn của BUG-FE-HLD-002) — grep toàn bộ `backend/src` — **không tồn tại**. Vậy `callRuntimeRpcStream` không phải "1 transport đúng nhưng auth sai" như solution ban đầu suy luận — nó là code gọi tới 1 API chưa từng được implement ở backend, cho 1 method mà backend còn không hỗ trợ multi-chunk.

**Kế hoạch gốc (xây `callRuntimeRpcStream` mới dựa trên `client.subscribe()` của WS) do đó cũng sai** — không có gì để "subscribe" nhiều lần từ 1 RPC request/response. Xây thêm hạ tầng streaming cho 1 method không stream là làm phức tạp hoá không cần thiết.

**Phát hiện thêm:** `runtime-git-client.ts` đã có sẵn `pushRuntimeGit()` — bản push dùng đúng `callRuntimeRpc(target, 'git.push', { worktree: ..., publish, pushTarget, forceWithLease }, ...)`, khớp chính xác params shape backend thật sự nhận. Đây là fix đúng, không phải viết mới — xem TASK-FE-HLD-003 (đã gộp cả nội dung TASK-FE-HLD-002 vào đó vì hoá ra là 1 thay đổi liền mạch, không tách được thành 2 bước độc lập có ý nghĩa).
