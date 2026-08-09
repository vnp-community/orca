# TASK-AG-HLD-005 — `GitHandler.commit()` Đọc Registry Theo `clientId`, Truyền `GIT_AUTHOR_*`/`GIT_COMMITTER_*` Qua Env

**Solution:** [SOL-AG-HLD-003](../solutions/SOL-AG-HLD-003-per-client-git-identity-env.md)
**Bug:** [BUG-AG-HLD-003](../BUG-AG-HLD-003-git-author-identity-global-mutable.md)
**File:** `agent/src/relay/git-handler.ts`
**Phụ thuộc:** TASK-AG-HLD-004 (registry `git-identity-registry.ts` phải tồn tại trước)
**Estimated:** 45 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Sửa `GitHandler.commit()` để đọc git identity từ registry theo `context.clientId` (đã tạo ở TASK-AG-HLD-004) và truyền `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL`/`GIT_COMMITTER_NAME`/`GIT_COMMITTER_EMAIL` qua env cho riêng lần `git commit` đó — không đụng đến global git config.

---

## Context

Đọc trước:
- `agent/src/relay/git-identity-registry.ts` (từ TASK-AG-HLD-004) — `getClientGitIdentity`, `buildGitIdentityEnv`
- `agent/src/relay/git-handler.ts` — class `GitHandler`, `registerHandlers()`, private `git()` helper, private `commit()`
- `agent/src/relay/dispatcher.ts` — `RequestContext` (đã import sẵn trong `git-handler.ts`)

---

## Thay Đổi Cần Thực Hiện

### 1. Thêm import

**TÌM** (nguyên văn, cuối khối import của file, ngay trước khai báo `execFileAsync`):
```typescript
import { GitResponseStreamRegistry } from './git-response-stream'
import { GIT_RESPONSE_STREAM_THRESHOLD } from './protocol'
import { endSubprocessStdin } from '../shared/subprocess-stdin-write'

const execFileAsync = promisify(execFile)
```

**THAY BẰNG:**
```typescript
import { GitResponseStreamRegistry } from './git-response-stream'
import { GIT_RESPONSE_STREAM_THRESHOLD } from './protocol'
import { endSubprocessStdin } from '../shared/subprocess-stdin-write'
import { getClientGitIdentity, buildGitIdentityEnv } from './git-identity-registry'

const execFileAsync = promisify(execFile)
```

### 2. `registerHandlers()` — truyền `context` cho `commit`

**TÌM** (nguyên văn, trong `registerHandlers()`):
```typescript
    this.dispatcher.onRequest('git.commit', (p) => this.commit(p))
```

**THAY BẰNG:**
```typescript
    this.dispatcher.onRequest('git.commit', (p, context) => this.commit(p, context))
```

### 3. `git()` helper — nhận thêm `extraEnv` để merge sau `buildRelayGitEnv()`

**TÌM** (nguyên văn, dòng 393-428):
```typescript
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
    }
  ): Promise<{ stdout: string; stderr: string }> {
    const env = buildRelayGitEnv()
    if (opts?.disableOptionalLocks) {
      env.GIT_OPTIONAL_LOCKS = '0'
    }
    if (opts?.nonInteractive) {
      env.GIT_TERMINAL_PROMPT = '0'
      env.GIT_ASKPASS = ''
      env.SSH_ASKPASS = ''
      env.GIT_SSH_COMMAND ??= 'ssh -o BatchMode=yes'
    }
    const execOptions = {
      cwd: expandTilde(cwd),
      env,
      encoding: 'utf-8',
      maxBuffer: opts?.maxBuffer ?? MAX_GIT_BUFFER,
      timeout: opts?.timeout,
      signal: opts?.signal
    } satisfies ExecFileOptions
    if (opts?.stdin !== undefined) {
      return execFileWithStdin('git', args, execOptions, opts.stdin)
    }
    const { stdout, stderr } = await execFileAsync('git', args, execOptions)
    return { stdout: String(stdout), stderr: String(stderr) }
  }
```

**THAY BẰNG:**
```typescript
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
      extraEnv?: NodeJS.ProcessEnv
    }
  ): Promise<{ stdout: string; stderr: string }> {
    const env = buildRelayGitEnv()
    if (opts?.extraEnv) {
      Object.assign(env, opts.extraEnv)
    }
    if (opts?.disableOptionalLocks) {
      env.GIT_OPTIONAL_LOCKS = '0'
    }
    if (opts?.nonInteractive) {
      env.GIT_TERMINAL_PROMPT = '0'
      env.GIT_ASKPASS = ''
      env.SSH_ASKPASS = ''
      env.GIT_SSH_COMMAND ??= 'ssh -o BatchMode=yes'
    }
    const execOptions = {
      cwd: expandTilde(cwd),
      env,
      encoding: 'utf-8',
      maxBuffer: opts?.maxBuffer ?? MAX_GIT_BUFFER,
      timeout: opts?.timeout,
      signal: opts?.signal
    } satisfies ExecFileOptions
    if (opts?.stdin !== undefined) {
      return execFileWithStdin('git', args, execOptions, opts.stdin)
    }
    const { stdout, stderr } = await execFileAsync('git', args, execOptions)
    return { stdout: String(stdout), stderr: String(stderr) }
  }
```

### 4. `commit()` — build `identityEnv` theo `context.clientId`, bọc `this.git` để truyền `extraEnv`

**TÌM** (nguyên văn, dòng 596-607):
```typescript
  private async commit(
    params: Record<string, unknown>
  ): Promise<{ success: boolean; error?: string }> {
    this.clearGitMutationReadCaches()
    const worktreePath = params.worktreePath as string
    const message = params.message as string
    try {
      return await commitChangesRelay(this.git.bind(this), worktreePath, message)
    } finally {
      this.clearGitMutationReadCaches()
    }
  }
```

**THAY BẰNG:**
```typescript
  private async commit(
    params: Record<string, unknown>,
    context: RequestContext
  ): Promise<{ success: boolean; error?: string }> {
    this.clearGitMutationReadCaches()
    const worktreePath = params.worktreePath as string
    const message = params.message as string
    // Why: BUG-AG-HLD-003 — author/committer come from this connection's
    // preflight.setGitIdentity call (if any), never from global git config.
    const identityEnv = buildGitIdentityEnv(getClientGitIdentity(context.clientId))
    try {
      return await commitChangesRelay(
        (args, cwd) => this.git(args, cwd, { extraEnv: identityEnv }),
        worktreePath,
        message
      )
    } finally {
      this.clearGitMutationReadCaches()
    }
  }
```

> [!IMPORTANT]
> Nếu client chưa từng gọi `preflight.setGitIdentity`, `getClientGitIdentity()` trả `undefined` → `buildGitIdentityEnv(undefined)` trả `{}` → hành vi fallback y hệt trước khi sửa (dùng `user.name`/`user.email` theo config hiện có trên host). Không breaking change cho setup single-user chưa gọi `setGitIdentity`.
>
> `RequestContext` đã được import sẵn ở đầu file (`import type { RelayDispatcher, RequestContext } from './dispatcher'`) — không cần thêm import mới cho type này.

---

## Verify

```bash
cd agent
npx tsc --noEmit
npx vitest run src/relay/__tests__/git-handler.test.ts
```

Test case mới cần thêm (trong `git-handler.test.ts` hoặc file test commit tương ứng):
- Hai `clientId` khác nhau gọi `preflight.setGitIdentity` (thông qua `setClientGitIdentity` trực tiếp trong test) với identity khác nhau → `git.commit` của mỗi client truyền đúng `GIT_AUTHOR_EMAIL`/`GIT_COMMITTER_EMAIL` tương ứng (assert trên `env` truyền vào `execFile`/`execFileAsync` mock).
- Client chưa gọi `setGitIdentity` → `git.commit` không set `GIT_AUTHOR_*`/`GIT_COMMITTER_*` (hành vi fallback không đổi).

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận thay đổi chỉ chạm `git-handler.ts` (`git()`, `commit()`, `registerHandlers()`), không lan ra symbol nào khác ngoài dự kiến.

---

## Definition of Done

- [ ] `git-handler.ts` import `getClientGitIdentity`, `buildGitIdentityEnv` từ `./git-identity-registry`
- [ ] `registerHandlers()`'s `onRequest('git.commit', ...)` truyền `context` cho `commit`
- [ ] `git()` opts nhận thêm `extraEnv?: NodeJS.ProcessEnv`, merge vào `env` bằng `Object.assign` ngay sau `buildRelayGitEnv()`
- [ ] `commit()` nhận thêm tham số `context: RequestContext`, build `identityEnv` từ `getClientGitIdentity(context.clientId)`
- [ ] `commit()` gọi `commitChangesRelay` với wrapper `(args, cwd) => this.git(args, cwd, { extraEnv: identityEnv })` thay vì `this.git.bind(this)`
- [ ] Không có breaking change cho client chưa gọi `setGitIdentity` (fallback `{}` env override)
- [ ] `npx tsc --noEmit` (trong `agent/`) pass
- [ ] `npx vitest run src/relay/__tests__/git-handler.test.ts` pass
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ show thay đổi trong `git-handler.ts` (và test file), không lan ra module khác

---

## Kết Quả Thực Thi (2026-08-09)

Đã sửa `git-handler.ts`: `commit()` nhận thêm `context`, đọc identity qua `getClientGitIdentity(context.clientId)`, truyền qua `extraEnv` cho `git()` helper. Client chưa gọi `setGitIdentity` → hành vi fallback không đổi (env rỗng).

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
