# SOL-AG-HLD-009 — Xoá `handleAgentExec()` Dead Code, Gộp Ngữ Cảnh Vào Case `agent.exec` Thật

**Fixes:** [BUG-AG-HLD-009](../BUG-AG-HLD-009-agent-exec-dead-duplicate-handler.md)
**TDD Ref:** `specs/agent/tdd/v5/07-jsonrpc-dispatch.md` §6 (v5.0 — Additional Methods); `specs/agent/tdd/v5/12-agent-spawner.md` (không đề cập `agent.exec`/TG-001 — xác nhận không phải nguồn thiết kế cho `handleAgentExec`)
**File:** `agent/src/relay/agent-exec-handler.ts`, `agent/src/relay/agent-rpc-dispatch.ts`
**Effort:** 30-45 phút
**Status:** 🔴 Open

---

## Phân Tích

### Xác nhận qua GitNexus (bắt buộc trước khi sửa symbol)

`impact({ target: "handleAgentExec", direction: "upstream", repo: "orca" })` →

```json
{
  "status": "ambiguous",
  "candidates": [
    { "uid": "Function:agent/src/relay/agent-exec-handler.ts:handleAgentExec", "impactedCount": 0, "risk": "LOW" },
    { "uid": "Function:desktop/src/relay/agent-exec-handler.ts:handleAgentExec", "impactedCount": 0, "risk": "LOW" }
  ]
}
```

`handleAgentExec` trong `agent/src/relay/agent-exec-handler.ts` có **0 upstream callers** (risk LOW) — xác nhận độc lập bằng grep: không xuất hiện ở đâu ngoài file định nghĩa nó. Đây thực sự là dead code. **Blast radius = 0, an toàn để xoá.**

Ngược lại, `impact({ target: "AgentExecHandler", direction: "upstream", file_path: "agent/src/relay/agent-exec-handler.ts" })` cho thấy **class `AgentExecHandler`** (cùng file, nhưng khác symbol — method `agent.execNonInteractive` / `agent.cancelExec`, dùng cho AI-commit-message generator) có 4 upstream refs, bao gồm `relay.ts:main` (`new AgentExecHandler(dispatcher)` tại `relay.ts:488`) và `agent-exec-handler-test-harness.ts`. **Class này đang chạy thật và có test — không được đụng vào.**

→ Kết luận quan trọng: file `agent-exec-handler.ts` **không phải toàn bộ là dead code**. Nó gộp chung 2 thứ không liên quan:

1. `class AgentExecHandler` (dòng 130-305) — RPC method `agent.execNonInteractive`/`agent.cancelExec`, **live**, wired ở `relay.ts:488`, test bởi `agent-exec-handler.test.ts` + `agent-exec-handler-windows.test.ts`.
2. `handleAgentExec()` + `parseAgentExecRequest()` + `AgentExecRequest` + docblock TG-001 (dòng 307-451) — RPC method `agent.exec`, **dead**, không dispatch ở đâu.

Đề xuất "xoá `handleAgentExec()` và toàn bộ `agent-exec-handler.ts`" trong bug report là **quá tay** — sẽ xoá luôn class `AgentExecHandler` đang chạy thật và làm vỡ `agent.execNonInteractive`/`agent.cancelExec` cũng như 2 file test. Solution này thu hẹp phạm vi: chỉ xoá phần dead code (mục 2), giữ nguyên class `AgentExecHandler`.

### So sánh 2 implementation của `agent.exec`

| Tiêu chí | Inline `case 'agent.exec'` (đang chạy, `agent-rpc-dispatch.ts:594-663`) | `handleAgentExec()` (dead, `agent-exec-handler.ts:355-451`) |
|---|---|---|
| Params contract | `binary`, `args`, `cwd`, `stdin`, `env`, `timeoutMs`, `taskId`, `stepId`, `parentTraceId` | `prompt`, `worktreePath`, `trustPreset`, `model`, `accountId`, `taskId`, `stepId`, `timeoutMs` — **khác hẳn contract** |
| Xử lý `env`/`extraEnv` | `spawnEnv = { ...process.env, ...extraEnv }` (dòng 623) — merge đúng, giữ `PATH` hệ thống + override từ caller | Tự build `toolEnv` cứng chỉ gồm `HOME/PATH/TERM/NO_COLOR/ORCA_TASK_ID/ORCA_WORKTREE_PATH`, **không đọc `params.env` — mất hoàn toàn khả năng nhận extraEnv từ caller** |
| Tracing | Có `Tracers.agentOrchSpawn` (start/step/ok/fail) — instrumented đầy đủ | Chỉ `log.info` 2 dòng, không tracer |
| Error handling | `try/catch` bọc toàn bộ, `makeError(rpc.id, AgentErrorCode.InvalidParams/ServerError, ...)` theo đúng JSON-RPC error contract dùng chung trong dispatcher | Tự trả `{ jsonrpc: '2.0', id, error: {...} }` thủ công, không dùng `AgentErrorCode`/`makeError` — không nhất quán |
| Test coverage | `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts` — nhiều test cho đúng method `'agent.exec'` với params `binary/args/cwd/env/timeoutMs/stepId/parentTraceId` (khớp 100% với inline case), gồm test riêng cho `hasEnvOverride` | 0 test — `agent-exec-handler.test.ts`/`agent-exec-handler-windows.test.ts` chỉ test class `AgentExecHandler` (`agent.execNonInteractive`), không test `handleAgentExec` |
| Trùng khớp docblock đã có sẵn tại call site | Case `'agent.exec'` đã có docblock ngắn (dòng 590-593, xem bên dưới) nhưng thiếu phần "Called by" | Docblock TG-001 (dòng 307-318) có phần "Called by: StepExecutors.executeAgent(), ProfileAwareAgentSpawner" — thông tin hữu ích, đáng giữ lại |

**Kết luận:** Bản inline đang chạy thật (`case 'agent.exec'`) tốt hơn ở MỌI tiêu chí quan trọng — xử lý `env` đúng, có tracing, có error contract nhất quán, và được test kỹ. Bản `handleAgentExec()` không những dead mà còn có params contract khác hẳn (thiết kế cho một API `agent.exec` "prompt-based" chưa từng tồn tại thật) — wire nó vào sẽ **làm vỡ toàn bộ test suite hiện có** của `agent-rpc-dispatch.test.ts` vì test gửi `binary/args/env`, không phải `prompt/worktreePath`.

→ Chọn **hướng (a)**: xoá `handleAgentExec()`/`parseAgentExecRequest()`/`AgentExecRequest`/docblock TG-001, đồng thời bổ sung phần "Called by" từ docblock cũ vào docblock đã có sẵn tại `case 'agent.exec'` trong dispatcher để không mất ngữ cảnh. Giữ nguyên class `AgentExecHandler` và toàn bộ phần còn lại của file.

---

## Thay Đổi Cần Thực Hiện

### File 1: `agent/src/relay/agent-exec-handler.ts`

Xoá toàn bộ khối từ dòng 307 đến hết file (dòng 452), tức là dead code section + import phụ trợ không còn dùng nếu có (`AgentConfig`, `AgentLogger` types — cần kiểm tra `class AgentExecHandler` phía trên có dùng chúng không; theo code đã đọc, class `AgentExecHandler` KHÔNG dùng `AgentConfig`/`AgentLogger`, chỉ dead code dùng — nên xoá luôn 2 import này).

```diff
--- a/agent/src/relay/agent-exec-handler.ts
+++ b/agent/src/relay/agent-exec-handler.ts
@@ -302,130 +302,3 @@ export class AgentExecHandler {
     })
   }
 }
-
-// ─── TG-001: handleAgentExec — Non-interactive AI agent execution ─────────────
-//
-// Called by:
-//   - StepExecutors.executeAgent() via relay.call('agent.exec', {...})
-//   - ProfileAwareAgentSpawner via relay.call('agent.exec', {...})
-//
-// Difference from agent.spawn (interactive PTY):
-//   - No terminal allocation (runs as subprocess with piped stdio)
-//   - Returns captured stdout/stderr in JSON-RPC response (not streamed)
-//   - Has a fixed timeout (default 5min)
-//   - Structured result includes stepId for workflow tracking
-// ─────────────────────────────────────────────────────────────────────────────
-
-import type { AgentConfig } from './agent-config'
-import type { AgentLogger } from './agent-logger'
-
-export interface AgentExecRequest {
-  prompt:       string
-  worktreePath: string
-  trustPreset?: 'standard' | 'full' | 'none'
-  model?:       string
-  accountId?:   string
-  taskId?:      string
-  stepId?:      string
-  timeoutMs?:   number
-}
-
-function parseAgentExecRequest(params: Record<string, unknown>): AgentExecRequest | null {
-  if (typeof params.prompt       !== 'string' || !params.prompt)       return null
-  if (typeof params.worktreePath !== 'string' || !params.worktreePath) return null
-  return {
-    prompt:       params.prompt,
-    worktreePath: params.worktreePath,
-    trustPreset:  typeof params.trustPreset === 'string' ? params.trustPreset as AgentExecRequest['trustPreset'] : 'standard',
-    model:        typeof params.model       === 'string' ? params.model       : undefined,
-    accountId:    typeof params.accountId   === 'string' ? params.accountId   : undefined,
-    taskId:       typeof params.taskId      === 'string' ? params.taskId      : undefined,
-    stepId:       typeof params.stepId      === 'string' ? params.stepId      : undefined,
-    timeoutMs:    typeof params.timeoutMs   === 'number' ? params.timeoutMs   : undefined,
-  }
-}
-
-/**
- * handleAgentExec — Run an AI agent CLI non-interactively and capture output.
- *
- * Supports Claude (--print mode), Codex, Gemini, and opencode.
- * Returns { stdout, stderr, exitCode, latencyMs, timedOut, stepId }.
- */
-export async function handleAgentExec(
-  id:     string | number | null,
-  params: Record<string, unknown>,
-  config: AgentConfig,
-  log:    AgentLogger,
-): Promise<object> {
-  const req = parseAgentExecRequest(params)
-  if (!req) {
-    return {
-      jsonrpc: '2.0', id,
-      error: { code: -32602, message: 'agent.exec: prompt and worktreePath are required' },
-    }
-  }
-
-  // Resolve binary based on model
-  const { resolveAgentSpec } = await import('./agent-spawner')
-  const spec = resolveAgentSpec(req.model ?? 'claude')
-  if (!spec) {
-    return {
-      jsonrpc: '2.0', id,
-      error: { code: -32602, message: `agent.exec: unknown model "${req.model ?? 'claude'}"` },
-    }
-  }
-
-  const { homedir } = await import('node:os')
-  const toolEnv: NodeJS.ProcessEnv = {
-    HOME: homedir(),
-    PATH: config.toolPath ?? process.env.PATH ?? '/usr/local/bin:/usr/bin:/bin',
-    TERM: 'dumb',
-    // Non-interactive mode — no color output
-    NO_COLOR: '1',
-    ...(req.taskId ? { ORCA_TASK_ID: req.taskId } : {}),
-    ...(req.worktreePath ? { ORCA_WORKTREE_PATH: req.worktreePath } : {}),
-  }
-
-  // Build CLI args for non-interactive (print) mode
-  // Claude uses: --print <prompt> --output-format text
-  const args: string[] = []
-  if (req.model)        args.push('--model', req.model)
-  args.push('--print', req.prompt, '--output-format', 'text')
-  if (req.trustPreset && req.trustPreset !== 'standard') {
-    args.push('--allowedTools', req.trustPreset === 'full' ? 'all' : 'none')
-  }
-
-  const timeoutMs = Math.min(req.timeoutMs ?? 300_000, 600_000)
-  log.info(`agent.exec: model=${req.model ?? 'claude'} cwd=${req.worktreePath} stepId=${req.stepId ?? '-'}`)
-
-  const start = Date.now()
-  const { spawn: nodeSpawn } = await import('node:child_process')
-
-  const result = await new Promise<{
-    stdout: string; stderr: string; exitCode: number | null; timedOut: boolean
-  }>((resolve) => {
-    let stdout = '', stderr = '', timedOut = false, settled = false
-
-    const finish = (r: typeof result): void => {
-      if (settled) return
-      settled = true
-      clearTimeout(timer)
-      resolve(r)
-    }
-
-    const child = nodeSpawn(spec.binary, args, {
-      cwd:   req.worktreePath,
-      env:   toolEnv,
-      stdio: ['pipe', 'pipe', 'pipe'],
-    })
-
-    const timer = setTimeout(() => {
-      timedOut = true
-      try { child.kill('SIGKILL') } catch { /* best effort */ }
-      finish({ stdout, stderr, exitCode: null, timedOut: true })
-    }, timeoutMs)
-
-    child.stdout?.on('data', (d: Buffer) => { stdout += d.toString('utf8') })
-    child.stderr?.on('data', (d: Buffer) => { stderr += d.toString('utf8') })
-    child.on('error',  (err) => { finish({ stdout, stderr: err.message, exitCode: null, timedOut }) })
-    child.on('close',  (code) => { finish({ stdout, stderr, exitCode: code, timedOut }) })
-
-    child.stdin?.end()
-  })
-
-  const latencyMs = Date.now() - start
-  log.info(`agent.exec: done exitCode=${result.exitCode} latency=${latencyMs}ms timedOut=${result.timedOut}`)
-
-  return {
-    jsonrpc: '2.0', id,
-    result: {
-      stdout:    result.stdout,
-      stderr:    result.stderr,
-      exitCode:  result.exitCode,
-      latencyMs,
-      timedOut:  result.timedOut,
-      stepId:    req.stepId,
-    },
-  }
-}
```

File còn lại sau khi xoá chỉ còn phần `class AgentExecHandler` (dòng 1-305 hiện tại), không đổi hành vi của `agent.execNonInteractive`/`agent.cancelExec`.

> **Lưu ý:** `desktop/src/relay/agent-exec-handler.ts` (GitNexus liệt kê là candidate thứ 2 của `handleAgentExec`, cùng nội dung dòng 354) nằm ngoài phạm vi bug này (module `desktop/`, không phải `agent/`). Nếu đó là bản sync/copy của cùng file, cân nhắc mở bug riêng — không xử lý trong SOL này để giữ đúng phạm vi `agent/`.

### File 2: `agent/src/relay/agent-rpc-dispatch.ts`

Bổ sung phần "Called by" từ docblock TG-001 cũ vào docblock đã có sẵn ở `case 'agent.exec'`, để không mất ngữ cảnh caller khi xoá file kia.

```diff
--- a/agent/src/relay/agent-rpc-dispatch.ts
+++ b/agent/src/relay/agent-rpc-dispatch.ts
@@ -591,6 +591,9 @@
     // ── v5.0: agent.exec ─────────────────────────────────────────────────────
     // TG-001: Non-interactive subprocess execution (for task graph steps).
     // Returns captured stdout/stderr/exitCode instead of streaming.
     // Distinct from agent.spawn (interactive PTY) — no terminal allocation.
+    // Called by:
+    //   - StepExecutors.executeAgent() via relay.call('agent.exec', {...})
+    //   - ProfileAwareAgentSpawner via relay.call('agent.exec', {...})
     case 'agent.exec': {
```

(Vị trí dòng chính xác phụ thuộc số dòng comment hiện có trước `case 'agent.exec': {` ở dòng 594 — chèn ngay trước dòng `case 'agent.exec': {`.)

---

## Verification

Script test/typecheck lấy đúng từ `agent/package.json`:

```bash
cd /opt/repos/orca

# 1. Typecheck toàn bộ agent/ (agent/ dùng chung tsconfig node của repo root — xác nhận agent/package.json không có script "test"/"typecheck" riêng, dùng script root)
pnpm run typecheck:node

# 2. Chạy full test suite (bao gồm agent-rpc-dispatch.test.ts, agent-exec-handler.test.ts, agent-exec-handler-windows.test.ts)
pnpm test -- agent/src/relay/agent-exec-handler.test.ts agent/src/relay/agent-exec-handler-windows.test.ts agent/src/relay/__tests__/agent-rpc-dispatch.test.ts

# 3. Xác nhận không còn reference nào tới symbol đã xoá
grep -rn "handleAgentExec\|parseAgentExecRequest\|AgentExecRequest" agent/src --include="*.ts"
# → phải trả về rỗng

# 4. Build agent bundle để chắc chắn không có import treo lơ lửng
pnpm run build:agent
```

Kỳ vọng:
- `agent-exec-handler.test.ts` và `agent-exec-handler-windows.test.ts` **PASS không đổi** — chúng chỉ test class `AgentExecHandler` (`agent.execNonInteractive`/`agent.cancelExec`), không đụng tới phần đã xoá.
- `agent-rpc-dispatch.test.ts` **PASS không đổi** — case `'agent.exec'` không bị sửa logic, chỉ thêm comment.
- `pnpm run typecheck:node` không báo lỗi unused import (`AgentConfig`/`AgentLogger` đã bị xoá cùng dead code).

Sau khi sửa, theo AGENTS.md/CLAUDE.md của repo:

```bash
node .gitnexus/run.cjs analyze   # cập nhật index nếu cần
# rồi trong phiên GitNexus:
detect_changes({ scope: "compare", base_ref: "main" })
```

để xác nhận thay đổi chỉ ảnh hưởng `handleAgentExec`, `parseAgentExecRequest`, `AgentExecRequest`, và comment tại `case 'agent.exec'` — không lan ra symbol nào khác (khớp với `impactedCount: 0` đã xác nhận ở bước phân tích).

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-exec-handler.ts` | Xoá `handleAgentExec()`/`parseAgentExecRequest()`/`AgentExecRequest`/docblock TG-001 (dòng 307-451) + 2 import không còn dùng; giữ nguyên `class AgentExecHandler` (dòng 1-305, live) |
| `agent/src/relay/agent-rpc-dispatch.ts` | Bổ sung "Called by" vào docblock trước `case 'agent.exec': {` (dòng ~594) |
| `agent/src/relay/agent-exec-handler-test-harness.ts` | Không đổi — chỉ dựng harness cho `class AgentExecHandler`, không tham chiếu `handleAgentExec` |
| `agent/src/relay/agent-exec-handler.test.ts` | Không đổi — test `AgentExecHandler`/`agent.execNonInteractive`, PASS nguyên trạng |
| `agent/src/relay/agent-exec-handler-windows.test.ts` | Không đổi — test Windows batch spawn cho `AgentExecHandler`, PASS nguyên trạng |
| `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts` | Không đổi logic — test hiện có cho `case 'agent.exec'` (params `binary/args/env/timeoutMs/stepId/parentTraceId`) tiếp tục là nguồn sự thật cho contract của `agent.exec` |
| `desktop/src/relay/agent-exec-handler.ts` | Ngoài phạm vi — candidate thứ 2 GitNexus tìm thấy cho `handleAgentExec`, thuộc module `desktop/`; cân nhắc bug riêng nếu cần dọn |
