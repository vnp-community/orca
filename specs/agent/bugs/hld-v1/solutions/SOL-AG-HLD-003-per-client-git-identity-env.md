# SOL-AG-HLD-003 — Git author/committer identity scoped per RPC client, không dùng `git config --global`

**Fixes:** [BUG-AG-HLD-003](../BUG-AG-HLD-003-git-author-identity-global-mutable.md)
**TDD Ref:** TDD-AG-10 §2 (`git.exec` whitelist/validation rules), §4 (Validation Rules table) — cộng với `docs/hld/dev-server-architecture.md:192` ("Agent Isolation Model") mà bug trích dẫn làm nguồn thiết kế gốc cho cam kết "Injected từ `ctx.userEmail`, không thể bị override"
**File:** `agent/src/relay/preflight-handler.ts`, `agent/src/relay/git-handler.ts`, `agent/src/relay/agent-git-handler.ts`, mới: `agent/src/relay/git-identity-registry.ts`
**Effort:** 3-5 giờ
**Status:** 🔴 TODO

---

## Phân Tích

`preflight.setGitIdentity` (`preflight-handler.ts:290-293`) và `GitHandler` (`git-handler.ts`) đều chạy trong **cùng một relay daemon process** (`RelayDispatcher` từ `dispatcher.ts`), và daemon này phục vụ **nhiều client connection đồng thời** — `RelayDispatcher` giữ `clients: Map<number, RelayClient>` với `clientId` riêng cho mỗi kết nối (`dispatcher.ts:16-19,73`). Đây chính là kịch bản multi-user mà `dev-server-architecture.md:192` mô tả.

`setGitIdentity` hiện tại ghi `git config --global user.name/user.email` — một side effect toàn cục trên filesystem của dev server, không gắn với `clientId` nào. `GitHandler.commit()` (`git-handler.ts:596-607`) sau đó chạy `git commit` mà không truyền `GIT_AUTHOR_*`/`GIT_COMMITTER_*` nào cả — nó hoàn toàn phụ thuộc vào global config tại thời điểm exec. Hai client cùng gọi `setGitIdentity` rồi `commit` xen kẽ nhau sẽ ghi nhầm tác giả.

Xác nhận qua GitNexus (`impact({target: "setGitIdentity", direction: "upstream"})`): risk **LOW**, duy nhất 1 caller trực tiếp — `PreflightHandler.registerHandlers` (đăng ký RPC), không ai khác gọi hàm này trực tiếp trong code — an toàn để đổi chữ ký (thêm tham số `context`).

**Gap phụ:** `agent/src/relay/agent-git-handler.ts` (dùng bởi Dev Server Agent qua WebSocket, `agent-rpc-dispatch.ts`) có `validateGitArgs()` cho phép subcommand `'config'` (dòng 63) **không có ràng buộc nào** trên `--global` — khác với `GitHandler.exec()` (relay daemon, `git-exec-validator.ts`) vốn đã chặn `--global`/`--system` qua `CONFIG_WRITE_FLAGS`. Đường WS-agent này vẫn có thể bị lợi dụng để override identity global bằng `git.exec` với `args: ['config', '--global', 'user.name', 'attacker']`. Bug đề xuất đúng: "`git.exec` nên từ chối override các biến này nếu caller cố truyền `--global`" — cần vá riêng ở đây.

## Thay Đổi Cần Thực Hiện

### 1. File mới: `agent/src/relay/git-identity-registry.ts`

Registry nhỏ, module-scope, key theo `clientId` — không ghi filesystem, sống trong bộ nhớ process và dọn dẹp khi client detach.

```typescript
// agent/src/relay/git-identity-registry.ts
// Per-RPC-client git author/committer identity — BUG-AG-HLD-003.
//
// Why: `git config --global` is a single mutable file shared by every
// client connected to this relay daemon. Scoping identity to `clientId`
// instead means one user's `preflight.setGitIdentity` call can never leak
// into another concurrently-connected user's `git commit`.

export interface GitIdentity {
  readonly name: string
  readonly email: string
}

const identityByClientId = new Map<number, GitIdentity>()

export function setClientGitIdentity(clientId: number, identity: GitIdentity): void {
  identityByClientId.set(clientId, identity)
}

export function getClientGitIdentity(clientId: number): GitIdentity | undefined {
  return identityByClientId.get(clientId)
}

export function clearClientGitIdentity(clientId: number): void {
  identityByClientId.delete(clientId)
}

/** Per-invocation env override — never touches global git config. */
export function buildGitIdentityEnv(identity: GitIdentity | undefined): NodeJS.ProcessEnv {
  if (!identity) return {}
  return {
    GIT_AUTHOR_NAME: identity.name,
    GIT_AUTHOR_EMAIL: identity.email,
    GIT_COMMITTER_NAME: identity.name,
    GIT_COMMITTER_EMAIL: identity.email
  }
}
```

### 2. `agent/src/relay/preflight-handler.ts` — `setGitIdentity` ghi vào registry, không ghi global config

```diff
+import { setClientGitIdentity, clearClientGitIdentity } from './git-identity-registry'
+import type { RequestContext } from './dispatcher'
 import type { RelayDispatcher } from './dispatcher'
```

```diff
   constructor(dispatcher: RelayDispatcher) {
     this.dispatcher = dispatcher
     this.registerHandlers()
+    // Why: BUG-AG-HLD-003 — identity is scoped to the connection that set
+    // it; without this, a stale entry would outlive a disconnected client.
+    this.dispatcher.onClientDetached?.((clientId) => clearClientGitIdentity(clientId))
   }

   private registerHandlers(): void {
     ...
     // TASK-021: Set git identity on the remote dev server
     this.dispatcher.onRequest('preflight.setGitIdentity', (p) =>
-      this.setGitIdentity(p as { name: string; email: string })
+      this.setGitIdentity(p as { name: string; email: string }, p as unknown as RequestContext)
     )
   }
```

> Lưu ý: `onRequest`'s `MethodHandler` signature là `(params, context) => Promise<unknown>` (`dispatcher.ts:22-25`) — sửa lại đúng chữ ký thay vì ép kiểu `params`:

```diff
-    this.dispatcher.onRequest('preflight.setGitIdentity', (p) =>
-      this.setGitIdentity(p as { name: string; email: string }, p as unknown as RequestContext)
+    this.dispatcher.onRequest('preflight.setGitIdentity', (p, context) =>
+      this.setGitIdentity(p as { name: string; email: string }, context)
     )
```

```diff
   /**
    * Set `git config --global user.name` and `user.email` on the remote machine.
-   * Why: errors propagate so the IPC caller can show a meaningful message.
+   * Why: BUG-AG-HLD-003 — identity is stored per-`clientId`, not written to
+   * `git config --global`, so two users sharing this relay daemon can never
+   * clobber each other's commit authorship. Errors propagate so the IPC
+   * caller can show a meaningful message.
    */
-  private async setGitIdentity(params: { name: string; email: string }): Promise<void> {
-    await execFileAsync('git', ['config', '--global', 'user.name', params.name])
-    await execFileAsync('git', ['config', '--global', 'user.email', params.email])
+  private async setGitIdentity(
+    params: { name: string; email: string },
+    context: RequestContext
+  ): Promise<void> {
+    if (!params.name || !params.email) {
+      throw new Error('name and email are required')
+    }
+    setClientGitIdentity(context.clientId, { name: params.name, email: params.email })
   }
```

### 3. `agent/src/relay/git-handler.ts` — `commit()` truyền identity vào `git()` theo `context.clientId`

```diff
 import { GIT_RESPONSE_STREAM_THRESHOLD } from './protocol'
 import { endSubprocessStdin } from '../shared/subprocess-stdin-write'
+import { getClientGitIdentity, buildGitIdentityEnv } from './git-identity-registry'
```

```diff
-    this.dispatcher.onRequest('git.commit', (p) => this.commit(p))
+    this.dispatcher.onRequest('git.commit', (p, context) => this.commit(p, context))
```

`git()` helper nhận thêm `extraEnv` để merge sau `buildRelayGitEnv()` (không thay global, chỉ env của tiến trình con này):

```diff
   private async git(
     args: string[],
     cwd: string,
     opts?: {
       maxBuffer?: number
       disableOptionalLocks?: boolean
       signal?: AbortSignal
       nonInteractive?: boolean
       stdin?: string
       timeout?: number
+      extraEnv?: NodeJS.ProcessEnv
     }
   ): Promise<{ stdout: string; stderr: string }> {
     const env = buildRelayGitEnv()
+    if (opts?.extraEnv) {
+      Object.assign(env, opts.extraEnv)
+    }
     if (opts?.disableOptionalLocks) {
```

`commit()` bind một wrapper truyền `extraEnv` cho `commitChangesRelay`:

```diff
   private async commit(
-    params: Record<string, unknown>
+    params: Record<string, unknown>,
+    context: RequestContext
   ): Promise<{ success: boolean; error?: string }> {
     this.clearGitMutationReadCaches()
     const worktreePath = params.worktreePath as string
     const message = params.message as string
+    // Why: BUG-AG-HLD-003 — author/committer come from this connection's
+    // preflight.setGitIdentity call (if any), never from global git config.
+    const identityEnv = buildGitIdentityEnv(getClientGitIdentity(context.clientId))
     try {
-      return await commitChangesRelay(this.git.bind(this), worktreePath, message)
+      return await commitChangesRelay(
+        (args, cwd) => this.git(args, cwd, { extraEnv: identityEnv }),
+        worktreePath,
+        message
+      )
     } finally {
       this.clearGitMutationReadCaches()
     }
   }
```

Nếu không có identity cho client đó (`buildGitIdentityEnv` trả `{}`), hành vi fallback y hệt trước — dùng `user.name`/`user.email` theo config hiện có trên host (local hoặc global cũ nếu còn), không breaking change cho single-user setups chưa gọi `setGitIdentity`.

### 4. `agent/src/relay/agent-git-handler.ts` — chặn `git config --global` qua WS-agent's `git.exec`

```diff
 export function validateGitArgs(args: string[]): void {
   if (args.length === 0) {
     throw new GitValidationError('GIT_NO_SUBCOMMAND', 'git args must not be empty — provide a subcommand')
   }

   if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
     throw new GitValidationError(
       'GIT_DISALLOWED_SUBCOMMAND',
       `git subcommand not allowed: "${args[0]}". Allowed: ${[...ALLOWED_GIT_SUBCOMMANDS].sort().join(', ')}`
     )
   }

+  // Why: BUG-AG-HLD-003 — identity must come from preflight.setGitIdentity's
+  // per-client registry, not a global config write that leaks to every
+  // other client sharing this dev server agent.
+  if (
+    args[0] === 'config' &&
+    (args.includes('--global') || args.includes('--system')) &&
+    (args.includes('user.name') || args.includes('user.email'))
+  ) {
+    throw new GitValidationError(
+      'GIT_SHELL_METACHARACTER_IN_ARG',
+      'git config --global/--system user.name|user.email is not allowed via git.exec — use preflight.setGitIdentity'
+    )
+  }
+
   for (const arg of args) {
```

## Verification

```bash
cd agent
npx vitest run src/relay/__tests__/preflight-handler.test.ts
npx vitest run src/relay/__tests__/git-handler.test.ts
```

Thêm test case mới:
- `preflight.setGitIdentity` từ client A không làm thay đổi `git config --global user.name` (mock `execFileAsync`/spy: không còn lời gọi ghi global).
- Hai `clientId` khác nhau gọi `setGitIdentity` với identity khác nhau → `commit()` của mỗi client dùng đúng `GIT_AUTHOR_EMAIL` tương ứng (assert trên env truyền vào `execFile`/`execFileAsync` mock).
- Client detach (`dispatcher.detachClient`) → `getClientGitIdentity` trả `undefined` sau đó.
- `agent-git-handler.ts`: `validateGitArgs(['config', '--global', 'user.name', 'x'])` → ném `GitValidationError`.

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/git-identity-registry.ts` | **Mới** — registry per-`clientId`, không ghi filesystem |
| `agent/src/relay/preflight-handler.ts` | `setGitIdentity` ghi vào registry thay vì `git config --global` |
| `agent/src/relay/git-handler.ts` | `commit()` đọc registry theo `context.clientId`, truyền `GIT_AUTHOR_*`/`GIT_COMMITTER_*` qua env cho lần exec đó |
| `agent/src/relay/agent-git-handler.ts` | Chặn `git config --global/--system user.name\|user.email` trong `validateGitArgs` (đường WS-agent riêng, chưa có ràng buộc này) |
| `agent/src/relay/dispatcher.ts` | Nguồn `RequestContext.clientId` và `onClientDetached` dùng để dọn registry |
| `docs/hld/dev-server-architecture.md:192` | Thiết kế gốc — "Git author | Injected từ `ctx.userEmail`, không thể bị override" |
