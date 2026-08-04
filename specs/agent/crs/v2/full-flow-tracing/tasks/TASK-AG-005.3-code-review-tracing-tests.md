# TASK-AG-005.3: Add code-review tracing tests (ai-complete-handler, agent-git-handler, agent-spawner)

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-005](../solutions/SOL-AG-TRACE-005-code-review.md)
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Precondition:** Phase 0 + [TASK-AG-005.1](./TASK-AG-005.1-ai-complete-handler-tracer.md) + [TASK-AG-005.2](./TASK-AG-005.2-git-pr-create-and-send-input-spans.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — created `ai-complete-handler.test.ts` (4 tests, fixed a mock shape bug: Anthropic response needs `type: 'text'` on content blocks). Added 3 tests to `agent-git-handler.test.ts` for `handleGitPrCreate` (kept this file's "real gh exec, don't mock" convention rather than mocking execFileAsync — the 3rd test asserts a `ghExec` step fires then either ok/fail follows, since gh isn't installed in the test env). Added 3 tests to `agent-spawner.test.ts` for `handleAgentSendInput`'s `agent:spawn` span (distinct from `agentOrch:stop`). Also discovered and fixed a real cross-domain bug during this task: the frontend background agent had renamed the shared `worktreeCreate`/`worktreeDelete` tracers.ts entries to add a `ui:` prefix, breaking these very tests — reverted + added separate `uiWorktreeCreateFlow`/`uiWorktreeDeleteFlow` entries for frontend use. 127/127 tests pass across all 3 files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này viết test cho các symbol đã tồn tại (vừa được TASK-AG-005.1/005.2 thêm tracer) — chạy `codegraph explore` để hiểu implementation thật trước khi viết assertion:

```bash
codegraph explore "handleAIComplete"
codegraph explore "handleGitPrCreate"
codegraph explore "handleAgentSendInput"
```

Đây đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "handleAIComplete", direction: "upstream" })
gitnexus_impact({ target: "handleGitPrCreate", direction: "upstream" })
gitnexus_impact({ target: "handleAgentSendInput", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/ai-complete-handler.test.ts` [NEW]

```typescript
import { describe, it, expect, vi, afterEach } from 'vitest'
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'
import { handleAIComplete } from '../ai-complete-handler'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }
const mockConfig = {} as AgentConfig

describe('handleAIComplete — agent:aiComplete tracing', () => {
  afterEach(() => { vi.unstubAllEnvs(); vi.restoreAllMocks() })

  it('emits start/fail with reason="no API key for model", never includes an API key value', async () => {
    vi.stubEnv('ANTHROPIC_API_KEY', '')
    vi.stubEnv('OPENAI_API_KEY', '')
    vi.stubEnv('GOOGLE_API_KEY', '')
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))

    await expect(
      handleAIComplete({ prompt: 'diff summary', model: 'claude-haiku' }, mockConfig, mockLog)
    ).rejects.toThrow('No API key found')

    unregister()
    expect(events.some(e => e.flow === 'agent:aiComplete' && e.level === 'start')).toBe(true)
    const fail = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'fail')
    expect(fail?.fields.err).toBe('no API key for model')
    expect(JSON.stringify(events)).not.toContain('sk-')
  })

  it('emits start THEN fail(reason="empty prompt") for an empty/whitespace-only prompt — span already exists before the validation check', async () => {
    // TASK-AG-005.1's implementation calls aiCompleteTracer.start() BEFORE the
    // `!prompt.trim()` check (so promptLength/taskId/format are always captured,
    // even on the empty-prompt failure path) — unlike an earlier draft that
    // validated first. A start event IS emitted here, followed by a fail.
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await expect(handleAIComplete({ prompt: '   ', taskId: 't-1' }, mockConfig, mockLog)).rejects.toThrow('prompt must not be empty')
    unregister()
    expect(events.filter(e => e.flow === 'agent:aiComplete' && e.level === 'start')).toHaveLength(1)
    const fail = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'fail')
    expect(fail?.fields.err).toBe('empty prompt')
    expect(fail?.fields.taskId).toBe('t-1')
  })

  it('providerNameFromModel classification reaches step("provider-call") with provider=anthropic for claude models', async () => {
    vi.stubEnv('ANTHROPIC_API_KEY', 'test-key')
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')))

    await expect(
      handleAIComplete({ prompt: 'x', model: 'claude-haiku' }, mockConfig, mockLog)
    ).rejects.toThrow()

    unregister()
    const step = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'step')
    expect(step?.label).toBe('provider-call')
    expect(step?.fields.provider).toBe('anthropic')
  })

  it('ok() includes contentLength and promptLength, never prompt or response content', async () => {
    vi.stubEnv('ANTHROPIC_API_KEY', 'test-key')
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ content: [{ text: 'ok response body' }] }),
    }))

    await handleAIComplete({ prompt: 'a very long secret diff body', model: 'claude-haiku' }, mockConfig, mockLog)

    unregister()
    const start = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'start')
    const ok = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'ok')
    expect(start?.fields.promptLength).toBe('a very long secret diff body'.length)
    expect(ok?.fields.contentLength).toBe('ok response body'.length)
    expect(JSON.stringify(events)).not.toContain('a very long secret diff body')
    expect(JSON.stringify(events)).not.toContain('ok response body')
  })
})
```

## File: `src/relay/__tests__/agent-git-handler.test.ts` [MODIFY]

Thêm `describe('handleGitPrCreate — agent:git tracing')` cạnh `describe('handleGitPrCreate — validation')` hiện có:
- `it('span.fail("missing title") khi title rỗng, KHÔNG gọi execFileAsync')`
- `it('span.fail("unsafe characters...") khi title/base chứa metachar')`
- `it('span.ok({url}) khi gh pr create thành công — mock execFileAsync')`

## File: `src/relay/__tests__/agent-spawner.test.ts` [MODIFY]

Thêm `describe('handleAgentSendInput — agent:spawn tracing')`:
- `it('span.fail("missing ptyId") khi ptyId rỗng')`
- `it('span.fail("pty-not-found") khi ptyId không có trong PTY_REGISTRY')`
- `it('span.ok({ptyId, bytes}) khi write thành công — KHÔNG chứa nội dung data trong fields')`

## Verification

```bash
pnpm vitest run src/relay/__tests__/ai-complete-handler.test.ts src/relay/__tests__/agent-git-handler.test.ts src/relay/__tests__/agent-spawner.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `ai-complete-handler.test.ts` (file mới) có 4 test case theo trên, pass — field/label naming khớp đúng chuẩn đã thống nhất (`promptLength`/`contentLength`/`'provider-call'`/`providerNameFromModel`, không phải `promptChars`/`contentChars`/`'providerCall'`/`providerNameOf`)
- [ ] `agent-git-handler.test.ts` có thêm 3 test case cho `handleGitPrCreate` tracing
- [ ] `agent-spawner.test.ts` có thêm 3 test case cho `handleAgentSendInput` tracing (span `agent:spawn`, không phải `agentOrch:stop` từ TASK-AG-002.4)
- [ ] Test xác nhận không có API key thật (`sk-...`) hoặc nội dung `data` xuất hiện trong bất kỳ `TraceEvent`
- [ ] `pnpm vitest run src/relay/__tests__/ai-complete-handler.test.ts src/relay/__tests__/agent-git-handler.test.ts src/relay/__tests__/agent-spawner.test.ts` pass
