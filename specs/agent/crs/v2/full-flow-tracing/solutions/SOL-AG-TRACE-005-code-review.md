# SOL-AG-TRACE-005: Code Review — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**TDD Ref:** TDD-AG-06 (Tool Handlers — agent-exec-handler.ts), TDD-AG-09 (AI Credential Relay, tham chiếu cho ranh giới credential)
**File(s):**
- `src/relay/ai-complete-handler.ts` [MODIFY]
- `src/relay/agent-git-handler.ts` [MODIFY]
- `src/relay/agent-spawner.ts` [MODIFY]
**Mức độ:** 🟡 Trung bình
**Thời gian ước tính:** 3h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

CR-TRACE-005 mô tả 5 sub-flow (BL-CR-01→05) với **hai code path song song** — LOCAL (Electron Main, `child_process.execFile`/`runtime.*`) và REMOTE (`relay.call()` → Dev Server Agent). Solution này **chỉ** bao phủ phần thực thi thật sự chạy trên Dev Server Agent (`src/relay/*.ts`, tiến trình `agent.js` build từ `agent-entry.ts`) — tức là các RPC handler được gọi khi `src/main/runtime/rpc/methods/git-remote.ts` (Main process, **ngoài phạm vi**) thực hiện `relay.call('git.exec' | 'git.pr.create' | 'ai.complete' | 'agent.sendInput', ...)`. Toàn bộ phần LOCAL path (PTY-based `orca-runtime-git.ts`, `commit-message-text-generation.ts`), routing (`ProjectServerRouter`), và GitHub REST client (`src/main/github/client.ts`) thuộc bộ solution phía backend/gateway riêng — không đụng tới ở đây.

Ánh xạ BL-CR-0X → agent-side RPC method thực tế (đã verify qua `grep`/`Read`, không dùng lại nguyên văn giả định của CR mà kiểm chứng lại):

| Sub-flow | RPC method (Agent WS JSON-RPC) | File agent-side | Trạng thái hiện tại |
|----------|--------------------------------|-------------------|----------------------|
| BL-CR-01 (Diff) | `git.exec` | `src/relay/agent-git-handler.ts:98` (`handleGitExec`) | **Đã có tracer** (`agent:git`) |
| BL-CR-04 (diff --staged) | `git.exec` | như trên | **Đã có tracer** |
| BL-CR-04 (AI generate) | `ai.complete` | `src/relay/ai-complete-handler.ts:38` (`handleAIComplete`) | **CHƯA có tracer nào** |
| BL-CR-05 (Tạo PR) | `git.pr.create` | `src/relay/agent-git-handler.ts:265` (`handleGitPrCreate`) | **CHƯA có tracer** |
| BL-CR-02/03 (remote feedback → PTY) | `agent.sendInput` | `src/relay/agent-spawner.ts:459` (`handleAgentSendInput`) | Chỉ có span generic ở tầng dispatch (`agent:rpc`), không có field `ptyId` |
| BL-CR-02 (Annotate — persist) | `annotation.create` | **không tồn tại** | N/A — xem mục 3.5 (forward-looking) |
| BL-CR-03 (feedback alternate route) | `review.sendFeedback` | **không tồn tại** | N/A — xem mục 3.5 (forward-looking) |

**Phát hiện lệch so với CR-TRACE-005 (cần cập nhật khi backend-side solution được viết):** CR-TRACE-005 §1/§4 (BL-CR-05) giả định nhánh remote gọi `relay.call('shell.exec', { cmd: 'gh', args: ghArgs })` (`git-remote.ts:416`). Kiểm tra `src/relay/agent-rpc-dispatch.ts` (toàn bộ danh sách `case` đã liệt kê) cho thấy **không có method `shell.exec`** trong dispatcher — chỉ có `shell.eval` (dùng nội bộ cho `devServer.browseDir`, không liên quan code review). Thay vào đó, có sẵn method `git.pr.create` (case tại `agent-rpc-dispatch.ts:319`, gọi `handleGitPrCreate` trong `agent-git-handler.ts`). Ngoài ra còn tồn tại **một implementation PR-creation thứ hai**, `github.pr.create` (case tại dispatch, gọi `handleGitHubPrCreate` trong `src/relay/external-api-connector.ts:130`) — bản này đã có tracer `agent:ext-api` sẵn (kèm idempotency check `checkExistingPr()`). Solution này instrument **cả hai** method để không phụ thuộc vào việc xác nhận backend thực sự gọi method nào; khuyến nghị điều tra riêng (ngoài phạm vi CR này) để loại bỏ 1 trong 2 handler trùng lặp.

## 2. Gap hiện tại

| # | File:function | Trạng thái | Hành động |
|---|----------------|-----------|-----------|
| 1 | `agent-git-handler.ts:98 handleGitExec` | Đã có `gitTracer` (`agent:git`) — `start({method, cmd, cwd})` / `ok({cmd, exitCode, outLen})` / `fail(...)` bao phủ đủ BL-CR-01 và bước `diffStaged` của BL-CR-04 | Không cần sửa — chỉ dẫn chiếu trong test plan |
| 2 | `ai-complete-handler.ts:38 handleAIComplete` (+ `callAnthropic`/`callOpenAI`/`callGoogle`) | Không có `createTracer`/span nào — chỉ `log.info`/`log.error` | **Thêm mới** tracer `agent:aiComplete` |
| 3 | `agent-git-handler.ts:265 handleGitPrCreate` | Không có tracer (khác với `handleGitHubPrCreate` trong `external-api-connector.ts` đã có `agent:ext-api`) | **Thêm** span dùng lại `gitTracer` đã có sẵn trong cùng file |
| 4 | `agent-spawner.ts:459 handleAgentSendInput` | Không có span riêng; tầng dispatch (`agent:rpc`) có bọc generic nhưng `extractTraceFields()` cho nhóm `agent.*` chỉ trích `session`/`cmd`, không khớp field thực tế của method này (`ptyId`/`data`) | **Thêm** span dùng lại `spawnerTracer` (`agent:spawn`) đã có sẵn trong cùng file |
| 5 | `annotation.create`, `review.sendFeedback` | RPC method không tồn tại trong `agent-rpc-dispatch.ts` (đã enumerate toàn bộ `case`, xác nhận không có) | Không implement — chỉ ghi tài liệu forward-looking (mục 3.5) |

## 3. Full Implementation

### 3.1 `ai-complete-handler.ts` — tracer `agent:aiComplete` (mới)

Đây là gap quan trọng nhất: đây chính là bước "AI generate" mà CR-TRACE-005 §1 (BL-CR-04) muốn tách latency ra khỏi bước git diff — hiện tại hoàn toàn không có timing nào ngoài `log.info` một dòng.

```typescript
// src/relay/ai-complete-handler.ts
// (thêm import + tracer ở đầu file, giữ nguyên toàn bộ phần còn lại)

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'

const aiCompleteTracer = createTracer('agent:aiComplete')

// ... (types AICompleteParams / AICompleteResult giữ nguyên) ...

export async function handleAIComplete(
  params: AICompleteParams,
  config:  AgentConfig,
  log:     AgentLogger,
): Promise<AICompleteResult> {
  const { prompt, format = 'text', taskId } = params

  const span = aiCompleteTracer.start({
    method: 'ai.complete', format, taskId, promptLength: prompt.length,
  })

  if (!prompt.trim()) {
    span.fail('empty prompt', { taskId })
    throw new Error('ai.complete: prompt must not be empty')
  }

  const model = params.model
    ?? (config as unknown as Record<string, unknown>)['defaultModel'] as string | undefined
    ?? process.env['ORCA_AI_MODEL_ID']
    ?? 'claude-opus-4-5'

  const apiKey = resolveApiKey(model)
  if (!apiKey) {
    const msg = `ai.complete: No API key found for model "${model}". ` +
      'Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY in the agent environment, ' +
      'or configure an AI provider in Orca settings.'
    span.fail('no API key for model', { model, taskId })
    throw new Error(msg)
  }

  log.info(`ai.complete: model=${model} format=${format} promptLen=${prompt.length}`)
  span.step('provider-call', { model, provider: providerNameFromModel(model) })

  try {
    const text = await dispatch(model, apiKey, prompt, format, log)
    span.ok({ model, contentLength: text.length })
    return { content: text, model }
  } catch (err: unknown) {
    span.fail(err, { model, taskId })
    throw err
  }
}

/** Trích tên provider chỉ để gắn field trace — KHÔNG chứa apiKey. */
function providerNameFromModel(model: string): 'anthropic' | 'openai' | 'google' | 'unknown' {
  if (model.startsWith('claude')) return 'anthropic'
  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) return 'openai'
  if (model.startsWith('gemini')) return 'google'
  return 'unknown'
}

// ... phần resolveApiKey / dispatch / callAnthropic / callOpenAI / callGoogle giữ nguyên,
//     KHÔNG được thêm apiKey vào bất kỳ TraceFields nào ...
```

**Ràng buộc bảo mật (bắt buộc, tương tự CR-TRACE-014 §4 BL-INT-01):** `apiKey` (giá trị đã resolve từ `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GOOGLE_API_KEY`) **không bao giờ** được đưa vào field của `agent:aiComplete` — chỉ `method`/`model`/`format`/`provider`/`promptLength`/`contentLength`/`taskId`. Nội dung `prompt`/response `content` cũng KHÔNG được đưa vào fields — chỉ độ dài.

### 3.2 `agent-git-handler.ts` — bổ sung span cho `handleGitPrCreate` (dùng lại `gitTracer`)

`gitTracer` đã tồn tại ở đầu file (`const gitTracer = createTracer('agent:git')`, dòng 27) — chỉ cần dùng lại, không tạo tracer mới (tránh vi phạm "1 tracer = 1 sub-flow" theo hướng ngược, tương tự lý luận CR-TRACE-013 §4 BL-AWS-03 về việc không nhân đôi tracer cho cùng 1 concern trong cùng 1 file).

```typescript
// src/relay/agent-git-handler.ts — handleGitPrCreate() (dòng 265)

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

### 3.3 `agent-spawner.ts` — bổ sung span cho `handleAgentSendInput` (dùng lại `spawnerTracer`)

```typescript
// src/relay/agent-spawner.ts — handleAgentSendInput() (dòng 459)

export async function handleAgentSendInput(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''

  const span = spawnerTracer.start({ method: 'agent.sendInput', ptyId: ptyId || '(empty)' })

  if (!ptyId) {
    span.fail('missing ptyId', { method: 'agent.sendInput' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    span.fail('pty-not-found', { ptyId })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.write(data)
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
    span.ok({ ptyId, bytes: data.length })
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.sendInput failed: ${msg}`)
    span.fail(err, { ptyId })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

Field `data` (nội dung feedback/keystroke gửi vào PTY) **không** được đưa vào trace fields — chỉ `bytes: data.length` — vì nội dung có thể chứa dữ liệu nhạy cảm do người dùng nhập (tương tự nguyên tắc không log token ở CR-TRACE-014).

### 3.4 Lan truyền `traceId` từ Agent WS JSON-RPC (theo CR-TRACE-000 §3.2/§3.3)

Theo hàng "Agent WS JSON-RPC 2.0" trong CR-TRACE-000 §3.3, `traceId` nằm trong `params._trace.id`. Hiện tại **không có handler nào trong 3 file trên đọc `params._trace`** — cũng như tầng dispatch (`agent-rpc-dispatch.ts`) chưa đọc field này (xác nhận qua Read: `rpcTracer.start({...})` không có tham số `resume`). Việc này phụ thuộc **CR-TRACE-000 mục 3** (`Tracer.start(fields?, resume?)`) chưa ship trong `src/shared/trace/index.ts` hiện tại (`start(fields: TraceFields = {})` — không có tham số `resume`). Khi core API đó ship, mẫu sửa cho 3 handler ở trên là:

```typescript
// Ví dụ cho handleAIComplete — CHỈ áp dụng sau khi Tracer.start(fields?, resume?) tồn tại
const traceId = (params as { _trace?: { id?: string } })._trace?.id
const span = aiCompleteTracer.start(
  { model, format, promptLength: prompt.length },
  traceId ? { id: traceId } : undefined
)
```

Vì core API resume chưa tồn tại, code ở mục 3.1–3.3 **không** đọc `_trace.id` — mỗi span agent-side vẫn tạo `id` độc lập với span backend (`codeReviewAiCommitFlow` v.v., thuộc bộ solution phía Main process). Khi TracePanel cần nối 2 span này, tạm thời phải dựa vào `elapsedMs`/thời điểm gần nhau + `method` field, không phải cùng `id` — đúng thực trạng CR-TRACE-013 §5 mục 2 đã ghi nhận cho `agent:rpc`.

### 3.5 Forward-looking — BL-CR-02 (Annotate) / BL-CR-03 (review.sendFeedback qua RPC riêng)

> **KHÔNG implement mục này.** `annotation.create` và `review.sendFeedback` không tồn tại như RPC method trên agent (đã enumerate toàn bộ `case` trong `agent-rpc-dispatch.ts`, xác nhận không có). CR-TRACE-005 §4 (BL-CR-02) ghi nhận `AgentManager.injectAnnotations()` chưa implement (BUG-AG-ORCH-005). Đoạn dưới đây chỉ mô tả **thiết kế dự kiến** nếu/khi 2 method này được thêm vào agent, để nhóm implement backend-side biết agent-side sẽ cần khai báo tracer gì — không được copy vào code cho tới khi RPC method thật sự tồn tại.

```typescript
// TƯƠNG LAI — chỉ tạo khi 'review.sendFeedback' thực sự được thêm vào agent-rpc-dispatch.ts
// Vị trí dự kiến: file mới hoặc agent-spawner.ts (cùng nhóm PTY input)
// Đặt tên theo local ad-hoc convention hiện có trong src/relay/ (prefix `agent:`),
// KHÔNG trùng với `codeReview:sendFeedback` (đó là tracer phía Main process/tracers.ts)
const reviewFeedbackTracer = createTracer('agent:reviewFeedback') // tên đề xuất, cần thống nhất khi implement

// export async function handleReviewSendFeedback(id, params, config, log) {
//   const span = reviewFeedbackTracer.start({ ptyId: params.ptyId })
//   ...
// }
```

## 4. Test Plan (Vitest)

### 4.1 `src/relay/__tests__/ai-complete-handler.test.ts` [MỚI — file chưa tồn tại]

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
    expect(fail?.fields.err).toContain('no API key for model')
    expect(JSON.stringify(events)).not.toContain('sk-')
  })

  it('rejects empty prompt with start+fail("empty prompt") on the span, never the prompt content', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await expect(handleAIComplete({ prompt: '   ' }, mockConfig, mockLog)).rejects.toThrow('prompt must not be empty')
    unregister()
    expect(events.some(e => e.flow === 'agent:aiComplete' && e.level === 'start')).toBe(true)
    const fail = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'fail')
    expect(fail?.fields.err).toBe('empty prompt')
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
    expect(step?.fields.provider).toBe('anthropic')
  })

  it('ok() includes contentLength, never the response content itself', async () => {
    vi.stubEnv('ANTHROPIC_API_KEY', 'test-key')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ content: [{ type: 'text', text: 'ok response body' }] }), { status: 200 })
    ))
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleAIComplete({ prompt: 'x', model: 'claude-haiku' }, mockConfig, mockLog)
    unregister()
    const ok = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'ok')
    expect(ok?.fields.contentLength).toBe('ok response body'.length)
    expect(JSON.stringify(ok?.fields)).not.toContain('ok response body')
  })
})
```

### 4.2 Mở rộng `src/relay/__tests__/agent-git-handler.test.ts` (đã tồn tại)

Thêm `describe('handleGitPrCreate — agent:git tracing')` cạnh `describe('handleGitPrCreate — validation')` hiện có (dòng 164):
- `it('span.fail("missing title") khi title rỗng, KHÔNG gọi execFileAsync')`
- `it('span.fail("unsafe characters...") khi title/base chứa metachar')`
- `it('span.ok({url}) khi gh pr create thành công — mock execFileAsync')`

### 4.3 Mở rộng `src/relay/__tests__/agent-spawner.test.ts` (đã tồn tại)

Thêm `describe('handleAgentSendInput — agent:spawn tracing')`:
- `it('span.fail("missing ptyId") khi ptyId rỗng')`
- `it('span.fail("pty-not-found") khi ptyId không có trong PTY_REGISTRY')`
- `it('span.ok({ptyId, bytes}) khi write thành công — KHÔNG chứa nội dung data trong fields')`

## 5. Acceptance Criteria

- [ ] `agent:aiComplete` bao phủ `handleAIComplete` với `step('provider-call')` tách biệt khỏi thời gian resolve API key/validate prompt, đúng mục tiêu CR-TRACE-005 (tách latency AI provider khỏi latency git diff — span diff nằm ở `agent:git`, span riêng biệt)
- [ ] Không field nào trong `agent:aiComplete` chứa giá trị `apiKey` (chỉ `method`/`model`/`format`/`provider`/`promptLength`/`contentLength`/`taskId`) — field naming này khớp 1:1 với SOL-AG-TRACE-018 §3.1 (cả hai solution cùng viết chung một implementation cho `ai-complete-handler.ts`, không còn drift)
- [ ] `handleGitPrCreate` (`agent-git-handler.ts`) và `handleGitHubPrCreate` (`external-api-connector.ts`, đã có sẵn `agent:ext-api`) đều có tracing đầy đủ — không phụ thuộc việc xác nhận backend gọi method nào
- [ ] `handleAgentSendInput` ghi được `ptyId` trong mọi event (`start`/`ok`/`fail`) — khắc phục gap của tầng dispatch generic không trích đúng field cho method này
- [ ] Không field nào trong `agent:spawn` (nhánh `agent.sendInput`) chứa nội dung `data` gửi vào PTY
- [ ] `agent:reviewFeedback` (mục 3.5) KHÔNG xuất hiện trong code thật cho tới khi `review.sendFeedback` RPC method tồn tại trong `agent-rpc-dispatch.ts`
- [ ] `ai-complete-handler.test.ts` mới tạo có ≥ 3 test case theo mục 4.1; `agent-git-handler.test.ts`/`agent-spawner.test.ts` có thêm ≥ 3 test case mỗi file theo mục 4.2/4.3
- [ ] `pnpm vitest run src/relay/__tests__/ai-complete-handler.test.ts src/relay/__tests__/agent-git-handler.test.ts src/relay/__tests__/agent-spawner.test.ts` pass
