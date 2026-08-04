# TASK-AG-014.1: Add agent:ext-api spans to auth-status and remaining handlers in external-api-connector.ts

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-014](../solutions/SOL-AG-TRACE-014-remote-integration.md)
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Precondition:** Phase 0 (`Tracer.start(fields?, resume?)`)
**Estimated time:** 1.5h
**Status:** ✅ Done (2026-08-03) — implemented as specified for all 7 handlers, reusing existing `apiTracer` (`agent:ext-api`). No drift from spec — real current source matched the task's code samples closely. `gitnexus_impact` on all 7 target handlers reported LOW risk (0 direct upstream callers — dispatched dynamically via `agent-rpc-dispatch.ts`'s method table, not visible to static call-graph). typecheck:node clean for `external-api-connector.ts` itself (pre-existing unused-import error remains in `external-api-connector.test.ts`, untouched by this task).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này sửa 7 handler cùng file, dùng lại `apiTracer` có sẵn — chạy `codegraph explore` cho các handler đại diện trước khi sửa (áp dụng cùng pattern cho `handleGitHubIssueList`/`handleGitHubIssueCreate`/`handleGitLabMrList`/`handleGitLabPipelineStatus`):

```bash
codegraph explore "handleGitHubAuthStatus"
codegraph explore "handleGitLabAuthStatus"
codegraph explore "handleGitLabMrCreate"
```

Cả 3 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "handleGitHubAuthStatus", direction: "upstream" })
gitnexus_impact({ target: "handleGitLabAuthStatus", direction: "upstream" })
gitnexus_impact({ target: "handleGitLabMrCreate", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục. Lặp lại `codegraph explore`/`gitnexus_impact` tương tự cho 4 handler còn lại trước khi sửa từng handler đó.

## Bối cảnh

`external-api-connector.ts` đã có tracer `apiTracer` (`agent:ext-api`) đầy đủ cho `handleGitHubPrCreate`/`handleGitHubPrMerge`. 7 handler còn lại trong CÙNG file (`handleGitHubAuthStatus`, `handleGitLabAuthStatus`, `handleGitHubIssueList`, `handleGitHubIssueCreate`, `handleGitLabMrCreate`, `handleGitLabMrList`, `handleGitLabPipelineStatus`) hoàn toàn KHÔNG có tracer. Task này dùng lại `apiTracer` có sẵn — KHÔNG tạo tracer mới.

## File: `src/relay/external-api-connector.ts` [MODIFY]

### `handleGitHubAuthStatus()`

```typescript
export async function handleGitHubAuthStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const env = buildGhEnv(userId, config.toolEnv)
  const span = apiTracer.start({ method: 'github.auth.status', cli: 'gh' })

  span.step('exec', { cli: 'gh' })
  try {
    const result = await execFileCaptured('gh', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`github.auth.status: userId=${userId} ok=${ok}`)
    if (ok) {
      span.ok({ cli: 'gh', authenticated: ok })
    } else {
      span.fail(result.stderr || 'gh auth status non-zero exit', { cli: 'gh', exitCode: result.exitCode, authenticated: false })
    }
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { cli: 'gh' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

### `handleGitLabAuthStatus()`

```typescript
export async function handleGitLabAuthStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const env = buildGlabEnv(userId, config.toolEnv)
  const span = apiTracer.start({ method: 'gitlab.auth.status', cli: 'glab' })

  span.step('exec', { cli: 'glab' })
  try {
    const result = await execFileCaptured('glab', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`gitlab.auth.status: userId=${userId} ok=${ok}`)
    if (ok) {
      span.ok({ cli: 'glab', authenticated: ok })
    } else {
      span.fail(result.stderr || 'glab auth status non-zero exit', { cli: 'glab', exitCode: result.exitCode, authenticated: false })
    }
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { cli: 'glab' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

**Ràng buộc bảo mật (bắt buộc):** không field nào trong `agent:ext-api` được chứa giá trị token/PAT — `userId` là ID nội bộ (không phải secret), `result.stdout`/`result.stderr` của `gh auth status`/`glab auth status` **không** được đưa vào trace fields (chỉ vào JSON-RPC `result` trả về client — khác kênh với trace). Chỉ field cho phép: `method`/`cli`/`exitCode`/`authenticated`.

### 5 handler còn lại — cùng pattern, dùng lại `apiTracer`

Áp dụng cùng pattern (`apiTracer.start({method, ...})` → `ok`/`fail`) cho `handleGitHubIssueList`, `handleGitHubIssueCreate`, `handleGitLabMrCreate`, `handleGitLabMrList`, `handleGitLabPipelineStatus`. Ví dụ đại diện (`handleGitLabMrCreate`, các handler còn lại theo đúng khuôn mẫu này):

```typescript
// src/relay/external-api-connector.ts — handleGitLabMrCreate()

export async function handleGitLabMrCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const title        = typeof params.title        === 'string' ? params.title.trim()        : ''
  const description  = typeof params.description  === 'string' ? params.description          : ''
  const targetBranch = typeof params.targetBranch === 'string' ? params.targetBranch.trim()  : 'main'
  const cwd          = typeof params.cwd          === 'string' && params.cwd ? params.cwd : config.workDir
  const userId       = typeof params.userId       === 'string' ? params.userId               : ''
  const span = apiTracer.start({ method: 'gitlab.mr.create', title: title.slice(0, 40), targetBranch })

  if (!title) {
    span.fail('missing title', { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(targetBranch)) {
    span.fail('unsafe characters in params', { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in MR params' } }
  }

  const glabArgs = ['mr', 'create', '--title', title, '--description', description, '--target-branch', targetBranch, '--yes']
  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'glab mr create failed', { method: 'gitlab.mr.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }
    const url = result.stdout.trim().split('\n').find((l: string) => l.startsWith('https://')) ?? result.stdout.trim()
    log.info(`gitlab.mr.create: MR → ${url}`)
    span.ok({ url })
    return { jsonrpc: '2.0', id, result: { url, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

Áp dụng tương tự cho `handleGitHubIssueList`/`handleGitHubIssueCreate` (`span = apiTracer.start({method:'github.issue.list'|'github.issue.create', ...})`) và `handleGitLabMrList`/`handleGitLabPipelineStatus` — mỗi handler: `span.ok({...kết quả không nhạy cảm...})` khi `exitCode === 0`, `span.fail(result.stderr, {exitCode})` khi khác 0, `span.fail(err)` khi catch exception.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "external-api-connector" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `handleGitHubAuthStatus`/`handleGitLabAuthStatus` phân biệt `cli: 'gh'` vs `cli: 'glab'` và `exitCode`/`authenticated` trong mọi event
- [ ] Không field nào trong `agent:ext-api` (bất kỳ handler nào) chứa giá trị token/PAT hoặc nội dung `stdout`/`stderr` thô của `gh`/`glab`
- [ ] 5 handler còn lại (`issue.list`, `issue.create`, `mr.create`, `mr.list`, `pipeline.status`) đều có tracing nhất quán với `handleGitHubPrCreate`/`handleGitHubPrMerge` đã có sẵn
- [ ] KHÔNG tạo tracer mới — chỉ dùng lại `apiTracer` (`agent:ext-api`) đã tồn tại đầu file
- [ ] `pnpm run typecheck:node` pass
