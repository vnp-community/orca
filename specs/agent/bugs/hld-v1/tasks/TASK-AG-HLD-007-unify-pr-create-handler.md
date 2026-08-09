# TASK-AG-HLD-007 — Hợp Nhất `git.pr.create` Về `handleGitHubPrCreate`, Xoá `handleGitPrCreate` Chết

**Solution:** [SOL-AG-HLD-004](../solutions/SOL-AG-HLD-004-unify-pr-create-handler.md)
**Bug:** [BUG-AG-HLD-004](../BUG-AG-HLD-004-duplicate-pr-create-implementations.md)
**File:** `agent/src/relay/agent-rpc-dispatch.ts`, `agent/src/relay/agent-git-handler.ts`
**Phụ thuộc:** —
**Estimated:** 60 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

`git.pr.create` và `github.pr.create` hiện là hai implementation riêng biệt tạo PR qua `gh pr create` — implementation trong `agent-git-handler.ts` (`handleGitPrCreate`) **không có idempotency check**, còn implementation trong `external-api-connector.ts` (`handleGitHubPrCreate`) **có** (`checkExistingPr` trước khi tạo). Trỏ `git.pr.create` sang `handleGitHubPrCreate` để cả hai method name dùng chung một implementation có bảo vệ chống tạo PR trùng, rồi xoá `handleGitPrCreate` chết.

---

## Context

Đọc trước:
- `agent/src/relay/agent-rpc-dispatch.ts` — hàm `route()`, `case 'git.pr.create'` (dòng 410-419)
- `agent/src/relay/agent-git-handler.ts` — `handleGitPrCreate()` (dòng 270-325), import `execFile`/`execFileAsync` ở đầu file
- `agent/src/relay/external-api-connector.ts` — `handleGitHubPrCreate()` (implementation giữ lại, không cần sửa)

---

## Thay Đổi Cần Thực Hiện

### 1. `agent/src/relay/agent-rpc-dispatch.ts` — trỏ `git.pr.create` sang `handleGitHubPrCreate`

**TÌM** (nguyên văn, dòng 410-419):
```typescript
    // ── v5.0: git.pr.create ──────────────────────────────────────────────────
    case 'git.pr.create': {
      try {
        const { handleGitPrCreate } = await import('./agent-git-handler')
        return (await handleGitPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`)
      }
    }
```

**THAY BẰNG:**
```typescript
    // ── v5.0: git.pr.create ──────────────────────────────────────────────────
    case 'git.pr.create': {
      try {
        // BUG-AG-HLD-004: 'git.pr.create' and 'github.pr.create' are the same
        // action — route both to the one implementation with the idempotency
        // check (handleGitHubPrCreate) so a retry/double-click never creates
        // a duplicate PR regardless of which method name the caller used.
        const { handleGitHubPrCreate } = await import('./external-api-connector')
        return (await handleGitHubPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`)
      }
    }
```

> Giữ nguyên tên method `git.pr.create` trong `switch` (tương thích ngược cho bất kỳ caller cũ nào), chỉ đổi handler đích.
>
> **Lưu ý shape thay đổi:** sau khi hợp nhất, `git.pr.create` trả về `{ url, number, title, state, alreadyExisted? }` (shape của `handleGitHubPrCreate`) thay vì `{ url, stdout, stderr }` (shape cũ của `handleGitPrCreate`). Hiện không có caller thật nào trong repo phụ thuộc `stdout`/`stderr` — không phải breaking change thực tế.

### 2. `agent/src/relay/agent-git-handler.ts` — xoá `handleGitPrCreate` và section header của nó

**TÌM** (nguyên văn, dòng 260-328 — từ `sendFrame` helper đến trước `// ─── git.worktree.list`):
```typescript
function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}

// ─── git.pr.create (via gh CLI) ──────────────────────────────────────────────

/**
 * Create a GitHub Pull Request via `gh pr create`.
 * Uses GH_CONFIG_DIR per userId for auth isolation.
 * Requires: `gh` binary in PATH + `gh auth login` configured for this user.
 */
export async function handleGitPrCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const title  = typeof params.title  === 'string' ? params.title.trim()  : ''
  const body   = typeof params.body   === 'string' ? params.body           : ''
  const base   = typeof params.base   === 'string' ? params.base.trim()   : 'main'
  const draft  = params.draft === true
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId          : ''

  const span = gitTracer.start({ method: 'git.pr.create', title: title.slice(0, 40), base })

  if (!title) {
    span.fail('missing title', { method: 'git.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    span.fail('unsafe characters in params', { method: 'git.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
  }

  const ghArgs: string[] = ['pr', 'create', '--title', title, '--body', body, '--base', base]
  if (draft) ghArgs.push('--draft')

  const { homedir } = await import('node:os')
  const env: NodeJS.ProcessEnv = {
    ...config.toolEnv,
    ...(userId ? { GH_CONFIG_DIR: `${homedir()}/.config/gh/${userId}/` } : {}),
    GH_NO_UPDATE_NOTIFIER: '1',
    GH_PROMPT_DISABLED:    '1',
  }

  span.step('ghExec', { base })
  try {
    const { stdout, stderr } = await execFileAsync('gh', ghArgs, { cwd, env, timeout: 30_000 })
    const url = stdout.trim()
    log.info(`git.pr.create: PR created → ${url}`)
    span.ok({ url })
    return { jsonrpc: '2.0', id, result: { url, stdout, stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`git.pr.create failed: ${msg}`)
    span.fail(err, { method: 'git.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── git.worktree.list ────────────────────────────────────────────────────────
```

**THAY BẰNG:**
```typescript
function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}

// ─── git.worktree.list ────────────────────────────────────────────────────────
```

### 3. `agent/src/relay/agent-git-handler.ts` — xoá import `execFile`/`execFileAsync` không còn dùng

**TÌM** (nguyên văn, dòng 15-19):
```typescript
import { spawn, execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
import type WebSocket from 'ws'
```

**THAY BẰNG:**
```typescript
import { spawn } from 'node:child_process'
import type WebSocket from 'ws'
```

> [!IMPORTANT]
> Kiểm tra lại bằng grep sau khi xoá rằng không còn tham chiếu nào tới `execFileAsync` hoặc `execFile` trong `agent-git-handler.ts` — `handleGitExec`/`handleGitExecStream`/`handleGitWorktreeList`/`handleGitWorktreeAdd`/`handleGitWorktreeRemove` chỉ dùng `spawn` (top-level import) hoặc tự `await import('node:child_process')` cục bộ, không phụ thuộc `execFileAsync` đã xoá.
> ```bash
> grep -n "execFileAsync\|\bexecFile\b" agent/src/relay/agent-git-handler.ts
> ```
> Nếu grep trả về kết quả nào ngoài các dòng vừa xoá, KHÔNG xoá import — giữ nguyên và báo cáo lại.

---

## Verify

```bash
cd agent
npx tsc --noEmit
npx vitest run src/relay/__tests__/agent-git-handler.test.ts
npx vitest run src/relay/__tests__/external-api-connector.test.ts
npx vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
```

Test case mới cần thêm (trong `agent-rpc-dispatch.test.ts` hoặc `external-api-connector.test.ts`):
- Gọi RPC `git.pr.create` với branch đã có PR mở → trả `{ ...existing, alreadyExisted: true }`, **không** spawn `gh pr create` lần 2.
- Gọi `git.pr.create` hai lần liên tiếp (double-click simulation) cho cùng branch → lần 2 phải nhận `alreadyExisted: true`, không tạo PR trùng.
- Xoá/migrate test case cũ trong `agent-git-handler.test.ts` đang assert hành vi của `handleGitPrCreate` đã xoá:
  ```bash
  grep -n "handleGitPrCreate" agent/src/relay/__tests__/agent-git-handler.test.ts
  ```

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận thay đổi chỉ chạm `agent-rpc-dispatch.ts` (case `git.pr.create`) và `agent-git-handler.ts` (xoá hàm + import), không lan sang symbol nào khác ngoài dự kiến.

---

## Definition of Done

- [ ] `agent-rpc-dispatch.ts`'s `case 'git.pr.create'` gọi `handleGitHubPrCreate` từ `./external-api-connector` (thay vì `handleGitPrCreate` từ `./agent-git-handler`)
- [ ] `agent-git-handler.ts` không còn export `handleGitPrCreate`
- [ ] `agent-git-handler.ts` không còn import `execFile`/`execFileAsync` chết (grep xác nhận sạch)
- [ ] `agent-git-handler.ts` vẫn giữ nguyên import `spawn` (dùng bởi `handleGitExec`/`handleGitExecStream`)
- [ ] Test case cũ cho `handleGitPrCreate` (nếu có) đã xoá/migrate
- [ ] Test mới: idempotency check hoạt động qua RPC `git.pr.create` (không tạo PR trùng)
- [ ] `npx tsc --noEmit` (trong `agent/`) pass
- [ ] 3 lệnh `npx vitest run` ở trên pass
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ show thay đổi trong `agent-rpc-dispatch.ts` và `agent-git-handler.ts` (+ test files)

---

## Kết Quả Thực Thi (2026-08-09)

Đã sửa `agent-rpc-dispatch.ts` (`case 'git.pr.create'` giờ trỏ tới `handleGitHubPrCreate`) và xoá `handleGitPrCreate`/import `execFile`/`execFileAsync` chết trong `agent-git-handler.ts`. Đã xoá luôn 2 describe-block test cũ trong `agent-git-handler.test.ts` assert hành vi của hàm đã xoá (test coverage cho `git.pr.create` giờ nằm ở `external-api-connector.test.ts` qua `handleGitHubPrCreate`).

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
