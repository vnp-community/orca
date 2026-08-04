# SOL-AG-TRACE-018: Task Graph — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**TDD Ref:** TDD-AG-12 (ProfileAware Agent Spawner — AI Agent CLI Host)
**File(s):** `src/relay/ai-complete-handler.ts` [MODIFY], `src/relay/agent-rpc-dispatch.ts` [MODIFY]
**Mức độ:** 🟡 trung bình
**Thời gian ước tính:** 2.5h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

CR-TRACE-018 có 4 sub-flow (BL-TG-01→04). Đọc trực tiếp các file backend liên quan (`TaskDAGValidator.ts`, `TaskGrantService.ts`, `TaskAIPlanner.ts`, `TaskAgentExecutor.ts`) để xác định chính xác cái nào chạm tới Dev Server Agent:

| Sub-flow | Component backend | Băng qua boundary tới agent? |
|---|---|---|
| BL-TG-01 (CRUD + cycle detection) | `TaskDAGValidator` — BFS thuần in-process trên `orca_task_edges` | **KHÔNG** — không có `relay.call()` nào trong `TaskDAGValidator.ts`. Không có agent-side counterpart. |
| BL-TG-02 (AI decompose) | `TaskAIPlanner.decompose()` → `relay.call('ai.complete', { prompt, format: 'json', taskId, model })` (`TaskAIPlanner.ts:54`) | **CÓ** — nhận bởi `case 'ai.complete'` (`agent-rpc-dispatch.ts:563-586`) → `handleAIComplete()` (`ai-complete-handler.ts`) |
| BL-TG-03 (grant resolution) | `TaskGrantService.resolvePermission()` — chuỗi SELECT tuần tự in-process | **KHÔNG** — không có `relay.call()` nào trong `TaskGrantService.ts`. Không có agent-side counterpart. |
| BL-TG-04 (agent execution) | `TaskAgentExecutor.executeTask()` → `ProfileAwareAgentSpawner.spawn()` → `relay.call('agent.exec', {...})` | **CÓ** — cùng handler `case 'agent.exec'` (`agent-rpc-dispatch.ts:502-557`) mà **SOL-AG-TRACE-015** (Profile) và **SOL-AG-TRACE-017** (Workflow) đã phân tích/instrument |

**Kết luận phạm vi:** BL-TG-01 và BL-TG-03 **không có gì để làm phía agent** — ghi nhận rõ và bỏ qua thay vì fabricate tracer không cần thiết. BL-TG-04 **tái sử dụng nguyên vẹn** đường đi `agent.exec` — solution này **không lặp lại** phần base (`binary`/`argsCount`/`exitCode`/`timedOut`, xem SOL-AG-TRACE-015 §3) hay phần `stepId`/`parentTraceId` (xem SOL-AG-TRACE-017 §3.1, không áp dụng trực tiếp cho Task Graph vì `TaskAgentExecutor` không đi qua `StepExecutors`), chỉ bổ sung phần dành riêng cho Task Graph: field `taskId`. **BL-TG-02 là phần việc chính, thực chất của solution này** — `handleAIComplete()` hiện **hoàn toàn không có tracer nào**.

## 2. Gap hiện tại

**Gap 1 (chính) — `ai-complete-handler.ts` không có bất kỳ tracer nào:**

```
$ grep -n "createTracer\|Tracer" src/relay/ai-complete-handler.ts
(không có kết quả)
```

Đọc toàn bộ 210 dòng file — xác nhận không có `import { createTracer }`, không có `span` nào. Đây là network hop thật sự ra ngoài (Anthropic/OpenAI/Google API, timeout 120s cố định qua `AbortSignal.timeout(120_000)` tại mỗi hàm `callAnthropic`/`callOpenAI`/`callGoogle`), đúng loại thao tác CR-TRACE-000 §5 rule 2 xác định là "đáng trace" (có khả năng chậm/fail độc lập) — nhưng hiện tại khi `ai.complete` chậm hoặc lỗi, không có span nào phản ánh model nào được chọn, provider nào bị gọi, hay lỗi đến từ đâu (thiếu key? HTTP lỗi? timeout?) ngoài dòng log thô `log.error(...)`.

Ngay cả tracer chung `agent:rpc` (`rpcTracer`, bọc mọi RPC dispatch) cũng không giúp được: `extractTraceFields()` trong `agent-rpc-dispatch.ts` (dòng 58-118) có bucket cho `fs.`/`git.`/`github.`/`gitlab.`/`ai.provider.`/`tools/call`/`agent.` nhưng **không có bucket nào cho `ai.complete`** — method này rơi vào `return {}` cuối hàm (dòng 117), span chỉ có `{ method: 'ai.complete', id }`, không `model`, không `promptLength`.

**Gap 2 — `agent.exec` thiếu `taskId` (cùng root cause với SOL-AG-TRACE-015/017, khác nguyên nhân cụ thể):** `TaskAgentExecutor.executeTask()` gọi `ProfileAwareAgentSpawner.spawn({ projectId, userId, command, workdir })` (`TaskAgentExecutor.ts` — theo bảng CR-TRACE-018 §4) — nhưng `ProfileAwareAgentSpawner.spawn()` (đã đọc trực tiếp `src/main/project/ProfileAwareAgentSpawner.ts:115-121`) chỉ gửi `{ binary, args, cwd, env, timeoutMs }` tới `relay.call('agent.exec', ...)`. `taskId` **chỉ tồn tại bên trong `env.ORCA_TASK_ID`** (một biến môi trường sẽ được set trong PTY con, `ProfileAwareAgentSpawner.ts` không set field này trực tiếp — nó đến từ `TaskAgentExecutor`/profile compose ở bước khác), **không phải một top-level param** của request `agent.exec`. Vì vậy phía agent **không thể** đọc `taskId` từ `params.taskId` hôm nay dù đã mở bucket `agent.exec` theo SOL-AG-TRACE-015 — đây là **prerequisite backend** (nằm trong chính acceptance criteria của CR-TRACE-018: "`AgentSpawnOptions` có thêm optional field `traceId` và `spawn()` forward nó vào relay call" — cùng logic áp dụng cho `taskId` nếu muốn hiển thị trong trace). Solution này chuẩn bị sẵn phía agent để đọc field này ngay khi backend gửi, tương tự cách tiếp cận forward-compatible của SOL-AG-TRACE-017 với `parentTraceId`.

## 3. Full Implementation

### 3.1. Thêm tracer `agent:aiComplete` cho `ai-complete-handler.ts`

```typescript
// src/relay/ai-complete-handler.ts

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'

const aiCompleteTracer = createTracer('agent:aiComplete')

// ── Types ─────────────────────────────────────────────────────────────────────

export interface AICompleteParams {
  prompt:  string
  format?: 'json' | 'text'
  taskId?: string
  model?:  string
}

export interface AICompleteResult {
  content: string
  model?:  string
}

// ── Main handler ──────────────────────────────────────────────────────────────

export async function handleAIComplete(
  params: AICompleteParams,
  config:  AgentConfig,
  log:     AgentLogger,
): Promise<AICompleteResult> {
  const { prompt, format = 'text', taskId } = params

  // CR-TRACE-018 BL-TG-02: KHÔNG BAO GIỜ đưa nội dung prompt/response vào
  // TraceFields — prompt thường chứa code/nội dung nghiệp vụ nội bộ (task
  // description, file content được inject vào planning prompt). Chỉ trace độ
  // dài (promptLength) — cùng nguyên tắc bảo mật đã áp dụng cho AI credential
  // (CR-TRACE-016 §1: "không đưa dữ liệu nhạy cảm vào TraceFields").
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
    // BL-TG-02: đây chính xác là "AI call chậm/fail" mà CR muốn tách biệt khỏi
    // "parse JSON lỗi" (parse JSON diễn ra ở TaskAIPlanner, phía backend — xem
    // §4 CR-TRACE-018) — fail() ở đây đại diện đúng cho nửa "AI call" phía agent.
    span.fail('no API key for model', { model, taskId })
    throw new Error(
      `ai.complete: No API key found for model "${model}". ` +
      'Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY in the agent environment, ' +
      'or configure an AI provider in Orca settings.'
    )
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

// CR-TRACE-018: tiện ích nhỏ để field `provider` dễ đọc trong TracePanel thay
// vì phải suy ra từ tiền tố `model`. Union type (thay vì `string` chung chung)
// khớp với chữ ký gốc trong SOL-AG-TRACE-005 §3.1 — cùng một hàm cho cùng một file.
function providerNameFromModel(model: string): 'anthropic' | 'openai' | 'google' | 'unknown' {
  if (model.startsWith('claude')) return 'anthropic'
  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) return 'openai'
  if (model.startsWith('gemini')) return 'google'
  return 'unknown'
}

// ── Key resolution / Provider dispatch / callAnthropic / callOpenAI / callGoogle ──
// ...unchanged...
```

### 3.2. Bucket `ai.complete` trong `extractTraceFields()` (cho span `agent:rpc` bên ngoài)

```typescript
// src/relay/agent-rpc-dispatch.ts

function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
  // ...existing helpers unchanged...

  // ...existing fs./git./github.-gitlab./ai.provider. buckets unchanged...

  if (method === 'ai.complete') {
    // CR-TRACE-018 BL-TG-02: trước đây method này rơi vào `return {}` cuối hàm
    // — span agent:rpc ngoài cùng không có field nào. Bucket riêng ở đây là lớp
    // wrapper thô (id dispatch-level); breakdown chi tiết (provider-call step,
    // contentLength, fail reason) nằm ở tracer agent:aiComplete riêng (§3.1) —
    // hai tracer bổ sung cho nhau, không trùng lặp: agent:rpc đo tổng thời gian
    // dispatch (bao gồm cả import động `./ai-complete-handler`), agent:aiComplete
    // đo riêng phần gọi provider.
    return {
      model:        str(p['model']),
      taskId:       str(p['taskId']),
      promptLength: typeof p['prompt'] === 'string' ? (p['prompt'] as string).length : undefined,
    }
  }

  if (method === 'agent.exec') {
    return {
      binary:         str(p['binary']),
      argsCount:      Array.isArray(p['args']) ? (p['args'] as unknown[]).length : undefined,
      hasEnvOverride: p['env'] !== undefined && p['env'] !== null,
      timeoutMs:      num(p['timeoutMs']),
      stepId:         str(p['stepId']),         // (SOL-AG-TRACE-017)
      parentTraceId:  str(p['parentTraceId']),  // (SOL-AG-TRACE-017)
      // CR-TRACE-018 BL-TG-04: chỉ có giá trị SAU KHI backend
      // (ProfileAwareAgentSpawner.spawn() / TaskAgentExecutor) được cập nhật để
      // gửi `taskId` như một top-level param thay vì chỉ nhét vào `env.ORCA_TASK_ID`
      // — xem Gap 2 ở mục 2. Cho tới lúc đó field này luôn undefined, không lỗi.
      taskId: str(p['taskId']),
    }
  }

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId']),
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}
```

> Khi triển khai cùng lúc SOL-AG-TRACE-015/017/018, gộp 3 khối field của bucket `method === 'agent.exec'` thành một object literal duy nhất (đã minh hoạ đầy đủ ở trên) — tránh 3 `if (method === 'agent.exec')` chồng nhau trong cùng hàm.

## 4. Test Plan (Vitest)

**File mới:** `src/relay/__tests__/ai-complete-handler.test.ts` (chưa tồn tại — xác nhận qua `ls src/relay/__tests__/` không có file này).

```typescript
import { describe, it, expect, vi, afterEach } from 'vitest'
import { handleAIComplete } from '../ai-complete-handler'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }
const mockConfig = {} as AgentConfig

afterEach(() => { vi.unstubAllEnvs(); vi.restoreAllMocks() })

describe('handleAIComplete — agent:aiComplete tracer (CR-TRACE-018 BL-TG-02)', () => {
  it('start event includes promptLength but never the prompt content', async () => {
    vi.stubEnv('ANTHROPIC_API_KEY', 'test-key')
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ content: [{ type: 'text', text: 'ok' }] }), { status: 200 })
    )
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await handleAIComplete({ prompt: 'a very long secret prompt body', model: 'claude-opus-4-5', taskId: 'task-1' }, mockConfig, mockLog)
    unregister()
    const start = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'start')!
    expect(start.fields.promptLength).toBe('a very long secret prompt body'.length)
    expect(JSON.stringify(start.fields)).not.toContain('secret prompt body')
  })

  it('fail()s with reason when no API key is found for the model', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await expect(handleAIComplete({ prompt: 'x', model: 'gemini-2.0' }, mockConfig, mockLog)).rejects.toThrow()
    unregister()
    const fail = events.find(e => e.flow === 'agent:aiComplete' && e.level === 'fail')!
    expect(fail.fields.err).toContain('no API key for model')
  })

  it('ok() includes contentLength, not the response content itself', async () => { /* mock fetch success, assert ok.fields.contentLength, no `content` key */ })

  it('step("provider-call") records the resolved provider name from the model prefix', async () => { /* assert step event fields.provider === 'anthropic' for claude-* */ })
})
```

Mở rộng `src/relay/__tests__/agent-rpc-dispatch.test.ts`:

```typescript
describe('ai.complete — extractTraceFields (CR-TRACE-018)', () => {
  it('surfaces model/taskId/promptLength on the agent:rpc dispatch span', async () => { /* dispatch ai.complete, assert agent:rpc start fields */ })
})

describe('agent.exec — taskId (CR-TRACE-018, forward-compat)', () => {
  it('surfaces taskId when present in params', async () => { /* params.taskId = 'task-99' */ })
  it('omits taskId cleanly when absent (current ProfileAwareAgentSpawner behavior)', async () => { /* matches today's real backend payload shape */ })
})
```

## 5. Acceptance Criteria

- [ ] `handleAIComplete()` có tracer `agent:aiComplete` bao phủ: `start` (model/format/taskId/promptLength) → `step('provider-call')` → `ok`(contentLength)/`fail`(reason) — field naming này khớp 1:1 với SOL-AG-TRACE-005 §3.1 (cả hai solution cùng viết chung một implementation cho `ai-complete-handler.ts`, không còn drift)
- [ ] Không có trace event nào trong `ai-complete-handler.ts` chứa nội dung `prompt` hay response `content` — chỉ độ dài
- [ ] `extractTraceFields()` có bucket riêng cho `ai.complete` (trước đây rơi vào `return {}`)
- [ ] Bucket `agent.exec` đọc `taskId` từ params, sẵn sàng nhận giá trị ngay khi backend (`ProfileAwareAgentSpawner.spawn()`) gửi field này — không cần sửa lại code phía agent lần 2
- [ ] Xác nhận rõ trong tài liệu: BL-TG-01 và BL-TG-03 không có và không cần agent-side counterpart (đã verify qua đọc `TaskDAGValidator.ts`/`TaskGrantService.ts` — không có `relay.call()` nào)
- [ ] Không lặp lại phần base instrumentation của `agent.exec` đã thuộc SOL-AG-TRACE-015, không lặp lại `stepId`/`parentTraceId` đã thuộc SOL-AG-TRACE-017
- [ ] `src/relay/__tests__/ai-complete-handler.test.ts` (file mới) đạt ít nhất 4 test case như mục 4
- [ ] `providerNameFromModel()` không throw với model prefix lạ — trả `'unknown'` thay vì lỗi, giữ handler resilient
