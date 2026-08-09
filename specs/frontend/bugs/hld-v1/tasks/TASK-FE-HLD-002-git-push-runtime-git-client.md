# TASK-FE-HLD-002 — Thêm `runtimeGitPush()` dùng stream RPC vào Git client

**Solution:** [SOLUTION-FE-HLD-002](../solutions/SOLUTION-FE-HLD-002-git-push-stream-auth.md)
**Bug:** [BUG-FE-HLD-002](../BUG-FE-HLD-002-git-push-stream-bearer-token-broken.md)
**File:** `frontend/src/renderer/src/runtime/runtime-git-client.ts`
**Estimated:** 20 phút
**Status:** ✅ ĐÃ CÓ SẴN — không cần làm gì, xem ghi chú bên dưới
**Phụ thuộc:** ~~TASK-FE-HLD-001~~ (task đó bị huỷ, xem file của nó)

---

## Mục tiêu

Thêm/hoàn thiện `runtimeGitPush()` trong `runtime-git-client.ts` (đúng vị trí đã đặc tả ở [tdd/v5/03-runtime-client-layer.md §8](../../../tdd/v5/03-runtime-client-layer.md#L284)), gọi `callRuntimeRpcStream()` vừa thêm ở TASK-FE-HLD-001 — thay vì để `useGit.ts` tự gọi thẳng 1 kênh HTTP riêng.

---

## Context

```bash
grep -n "runtimeGitPush\|async function runtimeGit" frontend/src/renderer/src/runtime/runtime-git-client.ts
```

Đọc trước:
- `frontend/src/renderer/src/runtime/runtime-git-client.ts` — các hàm `runtimeGitStatus`/`runtimeGitLog`/`runtimeGitDiff`/`runtimeGitCommit` đã có, dùng làm mẫu style (cùng file, cùng convention truyền `target`/`repoPath`)
- TASK-FE-HLD-001 (đã xong) — `callRuntimeRpcStream()`

---

## Thay đổi cần thực hiện

**Nếu `runtimeGitPush` CHƯA tồn tại**, thêm mới, đặt cạnh `runtimeGitCommit`:

```typescript
// runtime-git-client.ts
export type GitPushArgs = {
  remote?: string
  branch?: string
  force?: boolean
}

export type GitPushProgressChunk =
  | { type: 'progress'; message: string; percent?: number }
  | { type: 'end'; success: true }

// Why: dùng callRuntimeRpcStream (TASK-FE-HLD-001) thay vì 1 kênh HTTP+Bearer
// riêng — xem BUG-FE-HLD-002. Auth (deviceToken/cookie) đã được xử lý đúng ở
// tầng client bên trong callRuntimeRpcStream, hàm này không cần biết gì về nó.
export async function runtimeGitPush(
  target: RuntimeClientTarget,
  repoPath: string,
  args: GitPushArgs,
  onProgress?: (chunk: GitPushProgressChunk) => void
): Promise<void> {
  await callRuntimeRpcStream<GitPushProgressChunk>(
    target,
    'git.push',
    { repoPath, ...args },
    (chunk) => onProgress?.(chunk)
  )
}
```

**Nếu `runtimeGitPush` ĐÃ tồn tại nhưng đang gọi trực tiếp `fetch`/`runtime-rpc-stream`:** sửa lại phần thân hàm để gọi `callRuntimeRpcStream` như trên, giữ nguyên chữ ký hàm (không đổi call site ở nơi khác nếu không cần).

---

## Verify

```bash
pnpm --filter frontend tsc --noEmit

grep -n "runtimeGitPush" frontend/src/renderer/src/runtime/runtime-git-client.ts
# Xác nhận thân hàm gọi callRuntimeRpcStream, KHÔNG gọi fetch/sessionStorage trực tiếp

grep -rn "runtime-rpc-stream" frontend/src/renderer/src/runtime/runtime-git-client.ts
# Phải KHÔNG có import nào từ file cũ (sẽ bị xoá ở TASK-FE-HLD-003)
```

---

## Definition of Done

- [x] Xác nhận `pushRuntimeGit()` đã tồn tại đầy đủ trong `runtime-git-client.ts:474-500` — đúng transport (`callRuntimeRpc(target, 'git.push', ...)` / `window.api.git.push`), đúng params shape backend thật cần (`worktree`, `publish`, `pushTarget`, `forceWithLease`)
- [x] Không cần thêm `GitPushArgs`/`GitPushProgressChunk`/streaming callback — backend `git.push` không streaming (xem TASK-FE-HLD-001), nên không có "progress chunk" nào để định nghĩa type cho nó
- [x] Không sửa file này — 0 thay đổi

## Ghi chú

Kế hoạch gốc giả định cần "thêm" `runtimeGitPush()` dùng cơ chế stream mới — nhưng hàm đã tồn tại, đúng và đủ dùng, chỉ là **chưa được `useGit.ts` gọi tới** (xem TASK-FE-HLD-003, nơi thực hiện việc cutover thật). Không có thay đổi nào ở `runtime-git-client.ts` trong bug fix series này.
