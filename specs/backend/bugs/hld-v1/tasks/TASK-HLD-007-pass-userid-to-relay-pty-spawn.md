# TASK-HLD-007: Truyền `userId` vào `relay.call('pty.spawn', ...)` cho GitHub/GitLab auth

**Priority:** 🔴 CRITICAL — thiếu cách ly credential per-user trên relay
**Effort:** ~20 phút
**Status:** ✅ DONE — 2026-08-09 (`userId: ctx.userId` thêm vào cả 4 vị trí `pty.spawn` trong `github-auth.ts`/`gitlab-auth.ts`, cộng phần tuỳ chọn `preflight.check`. `tsc --noEmit` sạch cho cả 3 file — xác nhận `relay.call()` không strict-type payload nên field mới không gây lỗi kiểu. Chưa có tác dụng thực tế cho tới khi TASK-HLD-008 xong — đúng như ghi chú dependency.)
**Bug refs:** BUG-BE-HLD-005 (phần Backend)
**Solution ref:** [SOLUTION-github-gitlab-relay-exact.md](../solutions/SOLUTION-github-gitlab-relay-exact.md)
**Depends on:** không có — nhưng **KHÔNG có tác dụng thực tế nếu TASK-HLD-008 chưa làm** (Agent-side `pty-handler.ts` hiện không đọc `userId`). Khuyến nghị làm song song hoặc ngay trước TASK-HLD-008.

---

## Mục tiêu

Sửa `github-auth.ts`/`gitlab-auth.ts` (path thật: `backend/src/main/runtime/rpc/methods/github-auth.ts` và `backend/src/main/runtime/rpc/methods/gitlab-auth.ts` — 2 file `backend/src/main/github/github-auth.ts` và `backend/src/main/gitlab/gitlab-auth.ts` mà bug report gốc trích dẫn **không tồn tại** ở path đó) để truyền `userId` từ `RpcContext` vào `relay.call('pty.spawn', ...)`.

Hiện tại `github.startAuthLogin` (`backend/src/main/runtime/rpc/methods/github-auth.ts:45-56`) gọi:

```typescript
const ptyId = await relay.call<string>('pty.spawn', {
  command: 'gh',
  args,
  env: {},          // ← rỗng — không userId, không GH_CONFIG_DIR
  cols: 120,
  rows: 30
})
```

`RpcContext` (`backend/src/main/runtime/rpc/core.ts:84-87`) **đã có sẵn `userId`** — bug chỉ là handler không đọc nó:

```typescript
// Why: credential-store reads are scoped per authenticated Orca user.
// Each user-process in ORCA_MULTI_USER=1 mode has a distinct userId injected
// via ORCA_USER_ID env var and forwarded here from the session router.
userId?: string
```

`gitlab-auth.ts`'s `gitlab.startAuthLogin` có pattern giống hệt với `command: 'glab'` và cũng cần sửa tương tự.

## File cần sửa/tạo

```
backend/src/main/runtime/rpc/methods/github-auth.ts   (sửa)
backend/src/main/runtime/rpc/methods/gitlab-auth.ts    (sửa)
backend/src/main/runtime/rpc/methods/preflight.ts      (sửa, tùy chọn nếu Agent cần biết user hiện tại ở preflight.check)
```

Không tạo file mới.

## Thay đổi cụ thể

### `backend/src/main/runtime/rpc/methods/github-auth.ts` — `github.startAuthLogin`

```typescript
defineMethod({
  name: 'github.startAuthLogin',
  params: StartAuthLoginParams,
  handler: async (params, ctx) => {
    if (!ctx.devServerManager) {
      throw new Error(
        'github.startAuthLogin requires Web Server mode (devServerManager not available)'
      )
    }
    const relay = ctx.devServerManager.getRelay(params.devServerId)
    if (!relay) {
      throw new Error(
        `Dev server '${params.devServerId}' relay is not connected. ` +
        `Connect to the dev server first.`
      )
    }

    const args = ['auth', 'login']
    if (params.host) {
      args.push('--hostname', params.host)
    }

    // FIX BUG-BE-HLD-005: forward the authenticated user so the Agent can
    // namespace GH_CONFIG_DIR per user (see external-api-connector.ts buildGhEnv).
    const ptyId = await relay.call<string>('pty.spawn', {
      command: 'gh',
      args,
      env: {},
      userId: ctx.userId,
      cols: 120,
      rows: 30
    })

    return { ptyId, devServerId: params.devServerId }
  }
}),
```

Áp dụng thay đổi tương tự (thêm `userId: ctx.userId` vào payload `pty.spawn`) cho `github.revokeAuth` trong cùng file.

### `backend/src/main/runtime/rpc/methods/gitlab-auth.ts` — `gitlab.startAuthLogin` và `gitlab.revokeAuth`

Áp dụng pattern giống hệt trên, chỉ đổi `command: 'gh'` → `command: 'glab'`.

### `backend/src/main/runtime/rpc/methods/preflight.ts` (tùy chọn — nếu `preflight.check` phía Agent cần biết user hiện tại)

```typescript
// backend/src/main/runtime/rpc/methods/preflight.ts
const result = await relay.call<Record<string, unknown>>(
  'preflight.check',
  { traceId: span.id, userId: ctx.userId },
  30_000
)
```

## Verification

```bash
pnpm tsc --noEmit

# Xác nhận userId được truyền trong payload pty.spawn:
grep -n "userId: ctx.userId" backend/src/main/runtime/rpc/methods/github-auth.ts
grep -n "userId: ctx.userId" backend/src/main/runtime/rpc/methods/gitlab-auth.ts
# Expected: mỗi file có ít nhất 2 kết quả (startAuthLogin + revokeAuth)

pnpm vitest run backend/src/main/runtime/rpc/methods/github-auth.test.ts
pnpm vitest run backend/src/main/runtime/rpc/methods/gitlab-auth.test.ts
```

Cập nhật/thêm test: mock `relay.call` và assert payload `pty.spawn` chứa `userId` đúng bằng `ctx.userId` được truyền vào handler, cho cả 4 handler (`github.startAuthLogin`, `github.revokeAuth`, `gitlab.startAuthLogin`, `gitlab.revokeAuth`).

**Lưu ý quan trọng:** verification đầy đủ end-to-end (login thực sự có `GH_CONFIG_DIR` đúng theo user) chỉ đạt được sau khi TASK-HLD-008 hoàn thành — task này chỉ đảm bảo `userId` được gửi đi trên relay, chưa đảm bảo Agent dùng nó.
