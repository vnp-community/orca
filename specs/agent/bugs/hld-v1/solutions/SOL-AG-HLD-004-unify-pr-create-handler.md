# SOL-AG-HLD-004 — Hợp nhất `git.pr.create`/`github.pr.create` về một implementation có idempotency-check

**Fixes:** [BUG-AG-HLD-004](../BUG-AG-HLD-004-duplicate-pr-create-implementations.md)
**TDD Ref:** TDD-AG-13 §3.3 (`handleGitHubPrCreate()` — idempotency check), §6 (RPC Method Registration pattern)
**File:** `agent/src/relay/agent-rpc-dispatch.ts`, `agent/src/relay/agent-git-handler.ts`
**Effort:** 1-2 giờ
**Status:** 🔴 TODO

---

## Phân Tích

Xác nhận bằng GitNexus (`impact({target: "handleGitPrCreate", direction: "upstream"})`): risk **LOW**, `impactedCount: 0` — `handleGitPrCreate` (`agent-git-handler.ts:277`) không có caller tĩnh nào trong call graph. Caller thực tế là `agent-rpc-dispatch.ts`'s `case 'git.pr.create'` (dòng 411-419), nhưng vì đó là `await import('./agent-git-handler')` (dynamic import) nên GitNexus không track được thành edge — xác nhận qua đọc trực tiếp file, không phải suy đoán. Kết luận: xoá `handleGitPrCreate` an toàn tuyệt đối về mặt call-graph — chỉ cần sửa đúng một chỗ gọi nó (`agent-rpc-dispatch.ts:411-419`).

Grep xác nhận thêm: không có caller nào khác trong repo gọi RPC method `'git.pr.create'` hay `'github.pr.create'` bằng tên chuỗi ngoài `agent/` (và bản sao `desktop/src/relay/*` — ngoài phạm vi package `agent/`). Nghĩa là hợp nhất response shape (xem bên dưới) hiện tại **không phá vỡ caller thực tế nào** trong codebase — an toàn để làm ngay, và chuẩn hoá shape cho người viết caller sau này.

**Khác biệt response shape cần lưu ý khi hợp nhất:**

| | `handleGitPrCreate` (agent-git-handler.ts, bị xoá) | `handleGitHubPrCreate` (external-api-connector.ts, giữ lại) |
|---|---|---|
| Idempotency check | ❌ không có | ✅ `checkExistingPr()` trước khi tạo |
| Success result | `{ url, stdout, stderr }` | `{ url, number, title, state, alreadyExisted? }` |
| gh args | `--title --body --base` (không `--json`) | `--title --body --base --json url,number,title,state` |
| env builder | `GH_CONFIG_DIR` inline trong hàm | `buildGhEnv(userId, config.toolEnv)` (tái dùng bởi mọi GitHub handler) |

Sau khi hợp nhất, `git.pr.create` trả về shape mới (`{url, number, title, state}`) — vì hiện không có caller thật nào trong repo phụ thuộc field `stdout`/`stderr` cũ, đây không phải breaking change thực tế, chỉ cần ghi chú lại cho phía backend/frontend khi họ wire tính năng "Create PR" vào RPC này.

## Thay Đổi Cần Thực Hiện

### 1. `agent/src/relay/agent-rpc-dispatch.ts` — trỏ `git.pr.create` sang `handleGitHubPrCreate`

```diff
     // ── v5.0: git.pr.create ──────────────────────────────────────────────────
     case 'git.pr.create': {
       try {
-        const { handleGitPrCreate } = await import('./agent-git-handler')
-        return (await handleGitPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+        // BUG-AG-HLD-004: 'git.pr.create' and 'github.pr.create' are the same
+        // action — route both to the one implementation with the idempotency
+        // check (handleGitHubPrCreate) so a retry/double-click never creates
+        // a duplicate PR regardless of which method name the caller used.
+        const { handleGitHubPrCreate } = await import('./external-api-connector')
+        return (await handleGitHubPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
       } catch (err: unknown) {
         const msg = err instanceof Error ? err.message : String(err)
         return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`)
       }
     }
```

> Giữ nguyên tên method `git.pr.create` trong `switch` (tương thích ngược cho bất kỳ caller cũ nào), chỉ đổi handler đích — đúng khuyến nghị của bug report ("định tuyến `git.pr.create` ... sang cùng implementation").

### 2. `agent/src/relay/agent-git-handler.ts` — xoá `handleGitPrCreate` và import không còn dùng

```diff
-// ─── git.pr.create (via gh CLI) ──────────────────────────────────────────────
-
-/**
- * Create a GitHub Pull Request via `gh pr create`.
- * Uses GH_CONFIG_DIR per userId for auth isolation.
- * Requires: `gh` binary in PATH + `gh auth login` configured for this user.
- */
-export async function handleGitPrCreate(
-  id:     string | number | null,
-  params: Record<string, unknown>,
-  config: AgentConfig,
-  log:    AgentLogger
-): Promise<object> {
-  const title  = typeof params.title  === 'string' ? params.title.trim()  : ''
-  const body   = typeof params.body   === 'string' ? params.body           : ''
-  const base   = typeof params.base   === 'string' ? params.base.trim()   : 'main'
-  const draft  = params.draft === true
-  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
-  const userId = typeof params.userId === 'string' ? params.userId          : ''
-
-  const span = gitTracer.start({ method: 'git.pr.create', title: title.slice(0, 40), base })
-
-  if (!title) {
-    span.fail('missing title', { method: 'git.pr.create' })
-    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
-  }
-  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
-    span.fail('unsafe characters in params', { method: 'git.pr.create' })
-    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
-  }
-
-  const ghArgs: string[] = ['pr', 'create', '--title', title, '--body', body, '--base', base]
-  if (draft) ghArgs.push('--draft')
-
-  const { homedir } = await import('node:os')
-  const env: NodeJS.ProcessEnv = {
-    ...config.toolEnv,
-    ...(userId ? { GH_CONFIG_DIR: `${homedir()}/.config/gh/${userId}/` } : {}),
-    GH_NO_UPDATE_NOTIFIER: '1',
-    GH_PROMPT_DISABLED:    '1',
-  }
-
-  span.step('ghExec', { base })
-  try {
-    const { stdout, stderr } = await execFileAsync('gh', ghArgs, { cwd, env, timeout: 30_000 })
-    const url = stdout.trim()
-    log.info(`git.pr.create: PR created → ${url}`)
-    span.ok({ url })
-    return { jsonrpc: '2.0', id, result: { url, stdout, stderr } }
-  } catch (err: unknown) {
-    const msg = err instanceof Error ? err.message : String(err)
-    log.error(`git.pr.create failed: ${msg}`)
-    span.fail(err, { method: 'git.pr.create' })
-    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
-  }
-}
-
 // ─── git.worktree.list ────────────────────────────────────────────────────────
```

`execFile`/`execFileAsync` top-level import chỉ được dùng bởi `handleGitPrCreate` đã xoá — `handleGitWorktreeList` tự `await import('node:child_process')` cục bộ, không phụ thuộc biến top-level này:

```diff
-import { spawn, execFile } from 'node:child_process'
-import { promisify } from 'node:util'
-
-const execFileAsync = promisify(execFile)
+import { spawn } from 'node:child_process'
 import type WebSocket from 'ws'
```

> Kiểm tra lại `spawn` vẫn được dùng (bởi `handleGitExec`/`handleGitExecStream`) — giữ nguyên import đó.

## Verification

```bash
cd agent
npx vitest run src/relay/__tests__/agent-git-handler.test.ts
npx vitest run src/relay/__tests__/external-api-connector.test.ts
npx vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
npx tsc --noEmit   # xác nhận không còn reference nào tới handleGitPrCreate/execFileAsync đã xoá
```

Sau khi sửa, chạy lại `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận thay đổi chỉ chạm `agent-rpc-dispatch.ts` (case `git.pr.create`) và `agent-git-handler.ts` (xoá hàm + import), không lan sang symbol nào khác ngoài dự kiến.

Test case mới cần thêm (trong `external-api-connector.test.ts` hoặc file dispatch test):
- Gọi RPC `git.pr.create` với branch đã có PR mở → trả `{ ...existing, alreadyExisted: true }`, **không** spawn `gh pr create` lần 2.
- Gọi `git.pr.create` hai lần liên tiếp (double-click simulation) cho cùng branch → lần 2 phải nhận `alreadyExisted: true`, không tạo PR trùng.

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-rpc-dispatch.ts` | `case 'git.pr.create'` đổi sang gọi `handleGitHubPrCreate` |
| `agent/src/relay/agent-git-handler.ts` | Xoá `handleGitPrCreate` + import `execFile`/`execFileAsync` không còn dùng |
| `agent/src/relay/external-api-connector.ts` | Implementation giữ lại — `handleGitHubPrCreate` (đã có `checkExistingPr`) |
| `agent/src/relay/__tests__/agent-git-handler.test.ts` | Xoá test case cho `handleGitPrCreate` nếu tồn tại, hoặc migrate sang test `external-api-connector` |
| Liên quan | BUG-AG-HLD-005 (cả hai đường PR/MR create đều bypass `runner.ts`'s circuit breaker — fix riêng, không nằm trong scope hợp nhất này) |
