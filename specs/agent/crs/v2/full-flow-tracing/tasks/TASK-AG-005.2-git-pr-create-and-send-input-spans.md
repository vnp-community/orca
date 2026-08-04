# TASK-AG-005.2: Add spans to handleGitPrCreate and handleAgentSendInput (reuse existing tracers)

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-005](../solutions/SOL-AG-TRACE-005-code-review.md)
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Precondition:** Phase 0 + [TASK-AG-001.1](./TASK-AG-001.1-agent-git-handler-worktree-tracing.md) (đảm bảo `gitTracer` const đã tồn tại trong `agent-git-handler.ts`) + [TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md) (đảm bảo `spawnerTracer` đã có trong `agent-spawner.ts`, và không đè code `orchSpan` đã thêm)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — implemented exactly as specced, layered on top of TASK-AG-002.3's `orchSpan` without conflict (two independent spans, both fire). 143/143 tests pass across `agent-git-handler.test.ts` + `agent-spawner.test.ts` + `sub-agent-spawner.test.ts`, `pnpm run typecheck:node` clean for both production files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handleGitPrCreate"
codegraph explore "handleAgentSendInput"
```

Cả 2 đều là symbol MODIFY (đã tồn tại — `handleAgentSendInput` vừa được TASK-AG-002.3 thêm `orchSpan`, task này thêm THÊM một span khác, không thay thế) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "handleGitPrCreate", direction: "upstream" })
gitnexus_impact({ target: "handleAgentSendInput", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

CR-TRACE-005 mô tả BL-CR-05 (tạo PR) giả định `relay.call('shell.exec', ...)` — dispatcher thật KHÔNG có method `shell.exec`. Thay vào đó có `git.pr.create` (`handleGitPrCreate` trong `agent-git-handler.ts`, chưa có tracer) và `github.pr.create` (`handleGitHubPrCreate` trong `external-api-connector.ts`, ĐÃ có `agent:ext-api`). Task này instrument `handleGitPrCreate` — không phụ thuộc việc xác nhận backend gọi method nào.

Ngoài ra, `agent-spawner.ts::handleAgentSendInput()` (dùng cho BL-CR-02/03, remote feedback → PTY) chưa ghi được `ptyId` trong mọi event vì tầng dispatch generic không trích đúng field. Thêm span local dùng lại `spawnerTracer` (`agent:spawn`) đã có sẵn trong file — **không tạo tracer mới**.

## File: `src/relay/agent-git-handler.ts` [MODIFY]

`gitTracer` đã tồn tại ở đầu file (`const gitTracer = createTracer('agent:git')`) — dùng lại, KHÔNG tạo tracer mới.

```typescript
// src/relay/agent-git-handler.ts — handleGitPrCreate()

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
```

## File: `src/relay/agent-spawner.ts` [MODIFY]

**Lưu ý:** [TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md) đã thêm `orchSpan` (Ctrl+C only) vào `handleAgentSendInput()`. Task này thêm THÊM một span `spawnerTracer` riêng (`agent:spawn`, method `agent.sendInput`) bao phủ MỌI lần gọi (không chỉ Ctrl+C) — 2 span khác granularity/mục đích, cùng tồn tại song song, không thay thế nhau. Merge cẩn thận với code đã có từ TASK-AG-002.3 thay vì ghi đè.

```typescript
// src/relay/agent-spawner.ts — handleAgentSendInput()
// Base signature/logic từ TASK-AG-002.3 giữ nguyên — chỉ thêm `span` (spawnerTracer) bên cạnh `orchSpan` đã có.

export async function handleAgentSendInput(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''

  const span = spawnerTracer.start({ method: 'agent.sendInput', ptyId: ptyId || '(empty)' })  // [NEW — CR-TRACE-005]

  if (!ptyId) {
    span.fail('missing ptyId', { method: 'agent.sendInput' })                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    span.fail('pty-not-found', { ptyId })                                           // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.write(data)
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
    span.ok({ ptyId, bytes: data.length })                                          // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.sendInput failed: ${msg}`)
    span.fail(err, { ptyId })                                                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

Field `data` (nội dung feedback/keystroke gửi vào PTY) **không** được đưa vào trace fields — chỉ `bytes: data.length`.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep -E "agent-git-handler|agent-spawner" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `handleGitPrCreate` dùng lại `gitTracer` (KHÔNG tạo tracer mới) — `span.fail()` khi missing title / unsafe chars, `span.ok({url})` khi thành công
- [ ] `handleAgentSendInput` ghi được `ptyId` trong MỌI event (`start`/`ok`/`fail`) qua span `spawnerTracer` mới thêm — không chỉ nhánh Ctrl+C của `orchSpan` (từ TASK-AG-002.3)
- [ ] Không field nào trong `agent:spawn` (nhánh `agent.sendInput`) chứa nội dung `data`
- [ ] `orchSpan` (Ctrl+C, từ TASK-AG-002.3) vẫn hoạt động song song, không bị xoá/ghi đè
- [ ] `pnpm run typecheck:node` pass
