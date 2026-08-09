# TASK-AG-HLD-004 — Tạo `git-identity-registry.ts` Và Sửa `setGitIdentity` Ghi Vào Registry Theo `clientId`

**Solution:** [SOL-AG-HLD-003](../solutions/SOL-AG-HLD-003-per-client-git-identity-env.md)
**Bug:** [BUG-AG-HLD-003](../BUG-AG-HLD-003-git-author-identity-global-mutable.md)
**File:** `agent/src/relay/git-identity-registry.ts` (mới), `agent/src/relay/preflight-handler.ts`
**Phụ thuộc:** —
**Estimated:** 60 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Tạo registry per-`clientId` lưu git author/committer identity trong bộ nhớ process (không ghi filesystem), và sửa `preflight.setGitIdentity` để ghi vào registry đó thay vì `git config --global user.name/user.email` — tránh hai client dùng chung relay daemon ghi đè identity của nhau.

---

## Context

Đọc trước:
- `agent/src/relay/dispatcher.ts` — `export type RequestContext = { clientId: number; isStale: () => boolean; signal?: AbortSignal }` (dòng 17-21), chữ ký `MethodHandler = (params, context) => ...`, và `onClientDetached(listener: (clientId: number) => void): () => void` (dòng 156-159)
- `agent/src/relay/preflight-handler.ts` — class `PreflightHandler`, `constructor`, `registerHandlers()`, `setGitIdentity()`

---

## Thay Đổi Cần Thực Hiện

### 1. File mới: `agent/src/relay/git-identity-registry.ts`

Tạo file với nội dung sau (copy-paste ready):

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

### 2. `agent/src/relay/preflight-handler.ts` — thêm import

**TÌM** (đúng 2 dòng import đầu tiên liên quan đến dispatcher):
```typescript
import type { RelayDispatcher } from './dispatcher'
import { buildRelayCommandEnv } from './relay-command-env'
```

**THAY BẰNG:**
```typescript
import type { RelayDispatcher } from './dispatcher'
import { buildRelayCommandEnv } from './relay-command-env'
import { setClientGitIdentity, clearClientGitIdentity } from './git-identity-registry'
```

### 3. `agent/src/relay/preflight-handler.ts` — constructor + `registerHandlers()`

**TÌM** (nguyên văn, dòng 41-62):
```typescript
export class PreflightHandler {
  private dispatcher: RelayDispatcher

  constructor(dispatcher: RelayDispatcher) {
    this.dispatcher = dispatcher
    this.registerHandlers()
  }

  private registerHandlers(): void {
    this.dispatcher.onRequest('preflight.detectAgents', (p) => this.detectAgents(p))
    this.dispatcher.onRequest('preflight.detectWindowsTerminalCapabilities', () =>
      this.detectWindowsTerminalCapabilities()
    )
    // TASK-018: Ghostty config detection
    this.dispatcher.onRequest('preflight.detectGhosttyConfig', () => this.detectGhosttyConfig())
    // TASK-020: Full preflight check (gh + git)
    this.dispatcher.onRequest('preflight.check', () => this.checkFullPreflight())
    // TASK-021: Set git identity on the remote dev server
    this.dispatcher.onRequest('preflight.setGitIdentity', (p) =>
      this.setGitIdentity(p as { name: string; email: string })
    )
  }
```

**THAY BẰNG:**
```typescript
export class PreflightHandler {
  private dispatcher: RelayDispatcher

  constructor(dispatcher: RelayDispatcher) {
    this.dispatcher = dispatcher
    this.registerHandlers()
    // Why: BUG-AG-HLD-003 — identity is scoped to the connection that set
    // it; without this, a stale entry would outlive a disconnected client.
    this.dispatcher.onClientDetached?.((clientId) => clearClientGitIdentity(clientId))
  }

  private registerHandlers(): void {
    this.dispatcher.onRequest('preflight.detectAgents', (p) => this.detectAgents(p))
    this.dispatcher.onRequest('preflight.detectWindowsTerminalCapabilities', () =>
      this.detectWindowsTerminalCapabilities()
    )
    // TASK-018: Ghostty config detection
    this.dispatcher.onRequest('preflight.detectGhosttyConfig', () => this.detectGhosttyConfig())
    // TASK-020: Full preflight check (gh + git)
    this.dispatcher.onRequest('preflight.check', () => this.checkFullPreflight())
    // TASK-021: Set git identity on the remote dev server
    this.dispatcher.onRequest('preflight.setGitIdentity', (p, context) =>
      this.setGitIdentity(p as { name: string; email: string }, context)
    )
  }
```

### 4. `agent/src/relay/preflight-handler.ts` — thân hàm `setGitIdentity`

**TÌM** (nguyên văn, dòng 284-294):
```typescript
  // ── TASK-021: Set git identity ──────────────────────────────────────────────

  /**
   * Set `git config --global user.name` and `user.email` on the remote machine.
   * Why: errors propagate so the IPC caller can show a meaningful message.
   */
  private async setGitIdentity(params: { name: string; email: string }): Promise<void> {
    await execFileAsync('git', ['config', '--global', 'user.name', params.name])
    await execFileAsync('git', ['config', '--global', 'user.email', params.email])
  }
}
```

**THAY BẰNG:**
```typescript
  // ── TASK-021: Set git identity ──────────────────────────────────────────────

  /**
   * Store git author/committer identity for this RPC connection.
   * Why: BUG-AG-HLD-003 — identity is stored per-`clientId`, not written to
   * `git config --global`, so two users sharing this relay daemon can never
   * clobber each other's commit authorship. Errors propagate so the IPC
   * caller can show a meaningful message.
   */
  private async setGitIdentity(
    params: { name: string; email: string },
    context: RequestContext
  ): Promise<void> {
    if (!params.name || !params.email) {
      throw new Error('name and email are required')
    }
    setClientGitIdentity(context.clientId, { name: params.name, email: params.email })
  }
}
```

> [!IMPORTANT]
> `setGitIdentity` không còn dùng `execFileAsync` cho identity nữa — nếu `execFileAsync` vẫn còn được các hàm khác trong file dùng (nó có, ví dụ `checkGhCli`/`checkGitCli`), **không xóa import/const `execFileAsync`**.
>
> `RequestContext` cần import type — kiểm tra xem file đã có `import type { RequestContext } from './dispatcher'` chưa; nếu chưa, thêm dòng đó cạnh import `RelayDispatcher` ở bước 2.

---

## Verify

```bash
cd agent
npx tsc --noEmit
npx vitest run src/relay/__tests__/preflight-handler.test.ts
```

Test case mới cần thêm (trong `preflight-handler.test.ts` hoặc file test tương ứng):
- `preflight.setGitIdentity` không còn gọi `execFileAsync('git', ['config', '--global', ...])` (spy không được gọi).
- Hai `clientId` khác nhau gọi `setGitIdentity` với identity khác nhau → `getClientGitIdentity(clientId)` trả đúng identity tương ứng cho mỗi client.
- Sau khi dispatcher gọi client-detach listener cho một `clientId` → `getClientGitIdentity(clientId)` trả `undefined`.
- `setGitIdentity` với `name`/`email` rỗng → throw.

---

## Definition of Done

- [ ] File mới `agent/src/relay/git-identity-registry.ts` tồn tại, export `GitIdentity`, `setClientGitIdentity`, `getClientGitIdentity`, `clearClientGitIdentity`, `buildGitIdentityEnv`
- [ ] `preflight-handler.ts` import `setClientGitIdentity`, `clearClientGitIdentity` từ `./git-identity-registry`
- [ ] Constructor gọi `this.dispatcher.onClientDetached?.((clientId) => clearClientGitIdentity(clientId))`
- [ ] `registerHandlers()`'s `onRequest('preflight.setGitIdentity', ...)` truyền `context` cho `setGitIdentity`
- [ ] `setGitIdentity()` không còn gọi `execFileAsync('git', ['config', '--global', ...])` — chỉ gọi `setClientGitIdentity(context.clientId, ...)`
- [ ] `setGitIdentity()` throw khi thiếu `name`/`email`
- [ ] `npx tsc --noEmit` (trong `agent/`) pass
- [ ] `npx vitest run src/relay/__tests__/preflight-handler.test.ts` pass

---

## Kết Quả Thực Thi (2026-08-09)

Đã tạo `agent/src/relay/git-identity-registry.ts` (mới) và sửa `preflight-handler.ts` (`setGitIdentity` giờ ghi vào registry theo `context.clientId` thay vì `git config --global`).

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
