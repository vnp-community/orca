# TASK-TG-002: Tạo ai-complete-handler.ts và thêm case vào agent-rpc-dispatch

**Priority:** 🔴 HIGH — AI task planning và git commit message không hoạt động  
**Effort:** ~45 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TG-002  
**Solution ref:** [SOLUTION-task-graph-exact.md](../solutions/SOLUTION-task-graph-exact.md)

---

## Mục tiêu

1. Tạo `src/relay/ai-complete-handler.ts` — handler gọi AI provider API (Anthropic/OpenAI/Google)
2. Thêm `case 'ai.complete'` vào `agent-rpc-dispatch.ts`

---

## Bước 1 — Tạo file mới: `src/relay/ai-complete-handler.ts`

```typescript
// src/relay/ai-complete-handler.ts
/**
 * ai.complete handler — AI completion for task planning and git commit messages (TDD-18)
 *
 * Called by:
 * - TaskAIPlanner.decompose() → relay.call('ai.complete', { prompt, format: 'json' })
 * - git-remote.ts → relay.call('ai.complete', { prompt, format: 'text' })
 *
 * Reads AI provider credentials from credential store.
 * Supports: Anthropic Claude, OpenAI GPT/o-series, Google Gemini
 */

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

export interface AICompleteParams {
  prompt:   string
  format?:  'json' | 'text'
  taskId?:  string
  model?:   string
}

export interface AICompleteResult {
  content: string
  model?:  string
}

export async function handleAIComplete(
  params:  AICompleteParams,
  config:  AgentConfig,
  log:     AgentLogger
): Promise<AICompleteResult> {
  const { prompt, format = 'text' } = params

  // 1. Read provider credential from agent config / credential store
  // Priority: params.model > config.defaultModel > env ORCA_AI_MODEL_ID
  const model = params.model
    ?? config.defaultModel
    ?? process.env['ORCA_AI_MODEL_ID']
    ?? 'claude-opus-4-5'

  const apiKey = await resolveApiKey(model, config, log)
  if (!apiKey) {
    throw new Error(`No API key configured for model: ${model}. Set up an AI provider in Orca settings.`)
  }

  log.info(`ai.complete: model=${model} format=${format} promptLen=${prompt.length}`)

  const text = await dispatch(model, apiKey, prompt, format, log)
  return { content: text, model }
}

async function resolveApiKey(
  model:  string,
  config: AgentConfig,
  log:    AgentLogger
): Promise<string | null> {
  // Try env vars first (injected by ProfileAwareAgentSpawner):
  if (model.startsWith('claude') && process.env['ANTHROPIC_API_KEY']) {
    return process.env['ANTHROPIC_API_KEY']
  }
  if ((model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3'))
       && process.env['OPENAI_API_KEY']) {
    return process.env['OPENAI_API_KEY']
  }
  if (model.startsWith('gemini') && process.env['GOOGLE_API_KEY']) {
    return process.env['GOOGLE_API_KEY']
  }

  // Try ai-provider-handler credential store:
  try {
    const { readCredential } = await import('./ai-provider-handler')
    const accountId = process.env['ORCA_ACCOUNT_ID'] ?? config.defaultAIAccountId
    if (accountId) {
      const cred = await readCredential(accountId)
      return cred?.apiKey ?? null
    }
  } catch (err) {
    log.warn(`ai.complete: credential store unavailable: ${String(err)}`)
  }

  return null
}

async function dispatch(
  model:   string,
  apiKey:  string,
  prompt:  string,
  format:  string,
  log:     AgentLogger
): Promise<string> {
  if (model.startsWith('claude')) {
    return callAnthropic(model, apiKey, prompt, format, log)
  }
  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) {
    return callOpenAI(model, apiKey, prompt, log)
  }
  if (model.startsWith('gemini')) {
    return callGoogle(model, apiKey, prompt, log)
  }
  throw new Error(`Unknown model provider for: ${model}`)
}

async function callAnthropic(
  model: string, apiKey: string, prompt: string, format: string, log: AgentLogger
): Promise<string> {
  const systemPrompt = format === 'json'
    ? 'Respond with valid JSON only. No markdown, no explanation.'
    : undefined

  const body: Record<string, unknown> = {
    model,
    max_tokens: 4096,
    messages: [{ role: 'user', content: prompt }],
  }
  if (systemPrompt) body['system'] = systemPrompt

  const res = await fetch('https://api.anthropic.com/v1/messages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-api-key': apiKey,
      'anthropic-version': '2023-06-01',
    },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(120_000),
  })

  if (!res.ok) {
    const err = await res.text().catch(() => res.statusText)
    throw new Error(`Anthropic ${res.status}: ${err}`)
  }

  const data = await res.json() as { content: Array<{ type: string; text?: string }> }
  return data.content.find(c => c.type === 'text')?.text ?? ''
}

async function callOpenAI(
  model: string, apiKey: string, prompt: string, log: AgentLogger
): Promise<string> {
  const res = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type':  'application/json',
      'Authorization': `Bearer ${apiKey}`,
    },
    body: JSON.stringify({
      model,
      messages:   [{ role: 'user', content: prompt }],
      max_tokens: 4096,
    }),
    signal: AbortSignal.timeout(120_000),
  })

  if (!res.ok) {
    const err = await res.text().catch(() => res.statusText)
    throw new Error(`OpenAI ${res.status}: ${err}`)
  }

  const data = await res.json() as { choices: Array<{ message: { content: string } }> }
  return data.choices[0]?.message.content ?? ''
}

async function callGoogle(
  model: string, apiKey: string, prompt: string, log: AgentLogger
): Promise<string> {
  const url = `https://generativelanguage.googleapis.com/v1beta/models/${model}:generateContent?key=${apiKey}`
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      contents: [{ parts: [{ text: prompt }] }],
    }),
    signal: AbortSignal.timeout(120_000),
  })

  if (!res.ok) {
    const err = await res.text().catch(() => res.statusText)
    throw new Error(`Google AI ${res.status}: ${err}`)
  }

  const data = await res.json() as {
    candidates: Array<{ content: { parts: Array<{ text?: string }> } }>
  }
  return data.candidates[0]?.content.parts[0]?.text ?? ''
}
```

---

## Bước 2 — Thêm case vào `src/relay/agent-rpc-dispatch.ts`

Thêm sau `case 'agent.exec'` block (khoảng sau line 557):

```typescript
    // ── v5.0: ai.complete ─────────────────────────────────────────────────────
    // TG-002: Non-interactive AI completion for task planning and commit messages.
    // Called by: TaskAIPlanner.decompose(), git-remote generateCommitMessage
    case 'ai.complete': {
      try {
        const p = rpc.params ?? {}
        const prompt = typeof p['prompt'] === 'string' ? p['prompt'] : ''
        if (!prompt) {
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'ai.complete: prompt is required')
        }
        const { handleAIComplete } = await import('./ai-complete-handler')
        const result = await handleAIComplete(
          {
            prompt,
            format:  typeof p['format'] === 'string' ? p['format'] as 'json' | 'text' : 'text',
            taskId:  typeof p['taskId'] === 'string' ? p['taskId']  : undefined,
            model:   typeof p['model']  === 'string' ? p['model']   : undefined,
          },
          config,
          log
        )
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`)
      }
    }
```

---

## Verification

```bash
pnpm tsc --noEmit

# Verify case exists:
grep -n "ai.complete" src/relay/agent-rpc-dispatch.ts
# Expected: 1 result với case 'ai.complete':

# Verify file created:
ls -la src/relay/ai-complete-handler.ts
```
