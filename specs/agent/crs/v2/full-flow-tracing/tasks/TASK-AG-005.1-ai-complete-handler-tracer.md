# TASK-AG-005.1: Add agent:aiComplete tracer to ai-complete-handler.ts (Code Review AI generate step)

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-005](../solutions/SOL-AG-TRACE-005-code-review.md)
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Precondition:** Phase 0 (`Tracer.start(fields?, resume?)`)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — implemented exactly as specced, no concurrent drift found in `ai-complete-handler.ts`. `pnpm run typecheck:node` clean for this file. Test file (`ai-complete-handler.test.ts`) doesn't exist yet — created in TASK-AG-005.3.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handleAIComplete"
```

`handleAIComplete` là symbol MODIFY (đã tồn tại — task này chỉ thêm tracer, không đổi logic nghiệp vụ) — chạy thêm

```
gitnexus_impact({ target: "handleAIComplete", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp — bao gồm cả BL-TG-02 "AI decompose task" dùng chung handler này, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

`ai-complete-handler.ts::handleAIComplete()` (RPC `ai.complete`, dùng bởi BL-CR-04 "AI generate commit message/review summary" VÀ BL-TG-02 "AI decompose task" — cùng một handler, 2 caller khác domain) hiện KHÔNG có tracer nào — chỉ `log.info`/`log.error`. Đây là gap quan trọng nhất của CR-TRACE-005: khi `ai.complete` chậm/lỗi, không có span nào tách biệt bước "resolve API key" khỏi bước "gọi provider" khỏi bước git diff (đã có `agent:git`).

**✅ Resolved 2026-08-02:** SOL-AG-TRACE-005 (CR-TRACE-005) và SOL-AG-TRACE-018 (CR-TRACE-018) từng độc lập đề xuất field naming khác nhau cho cùng tracer `agent:aiComplete` — đã đồng bộ lại, cả 2 solution giờ dùng chung 1 field shape (dưới đây). Không còn task reconcile riêng — task này là điểm TẠO DUY NHẤT của `agent:aiComplete`; SOL-AG-TRACE-018/TASK-AG-018.2 chỉ bổ sung bucket cho `agent:rpc` (file khác, `agent-rpc-dispatch.ts`), không đụng lại file này.

## File: `src/relay/ai-complete-handler.ts` [MODIFY]

Thêm import + tracer ở đầu file, giữ nguyên toàn bộ phần còn lại:

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

  // KHÔNG BAO GIỜ đưa nội dung prompt/response vào TraceFields — prompt thường
  // chứa code/nội dung nghiệp vụ nội bộ (commit diff, task description, file
  // content). Chỉ trace độ dài (promptLength) — cùng nguyên tắc bảo mật đã áp
  // dụng cho AI credential (CR-TRACE-016 §1).
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

/** Trích tên provider chỉ để gắn field trace — KHÔNG chứa apiKey. */
function providerNameFromModel(model: string): string {
  if (model.startsWith('claude')) return 'anthropic'
  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) return 'openai'
  if (model.startsWith('gemini')) return 'google'
  return 'unknown'
}

// ... phần resolveApiKey / dispatch / callAnthropic / callOpenAI / callGoogle giữ nguyên,
//     KHÔNG được thêm apiKey vào bất kỳ TraceFields nào ...
```

**Ràng buộc bảo mật (bắt buộc):** `apiKey` (giá trị đã resolve từ `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GOOGLE_API_KEY`) **không bao giờ** được đưa vào field của `agent:aiComplete` — chỉ `method`/`model`/`format`/`provider`/`promptLength`/`contentLength`/`taskId`. Nội dung `prompt`/response `content` cũng KHÔNG được đưa vào fields — chỉ độ dài.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "ai-complete-handler" || echo "No errors"
pnpm test --run src/relay/__tests__/ai-complete-handler.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent:aiComplete` bao phủ `handleAIComplete` với `step('provider-call')` tách biệt khỏi thời gian resolve API key/validate prompt
- [ ] Field naming đúng chuẩn đã thống nhất: `promptLength` (không phải `promptChars`), `contentLength` (không phải `contentChars`), step label `'provider-call'` (không phải `'providerCall'`), helper tên `providerNameFromModel` (không phải `providerNameOf`)
- [ ] `start` fields gồm `method`/`format`/`taskId`/`promptLength`
- [ ] Không field nào trong `agent:aiComplete` chứa giá trị `apiKey`
- [ ] `span.fail('empty prompt', {taskId})` khi prompt rỗng, TRƯỚC khi throw
- [ ] `span.fail('no API key for model', {model, taskId})` khi không tìm thấy API key
- [ ] Không field nào chứa nội dung `prompt` hay response text — chỉ `promptLength`/`contentLength`
- [ ] `providerNameFromModel()` không throw với model prefix lạ — trả `'unknown'`
- [ ] `pnpm run typecheck:node` pass
