/**
 * ai-complete-handler — AI text completion for task planning and commit messages (TDD-18)
 *
 * IMPORTANT: This file runs on the Dev Server (relay binary), NOT on Orca Server.
 * Do NOT import anything from src/main/.
 *
 * Called by relay dispatch case 'ai.complete':
 *   - TaskAIPlanner.decompose() → relay.call('ai.complete', { prompt, format: 'json' })
 *   - git generateCommitMessage → relay.call('ai.complete', { prompt, format: 'text' })
 *
 * Credential resolution priority:
 *   1. Environment variables injected by agent spawn (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY)
 *   2. params.resolvedApiKey — same mechanism as agent.spawn's resolvedApiKey (agent-spawner.ts
 *      buildAgentEnv): Orca Server holds the Layer-1 session key and forwards a plaintext key
 *      per-call. This process (relay/agent.js) never decrypts the credential store directly —
 *      see BUG-AG-HLD-002 for why reading agent-credential-store.ts here would inject ciphertext.
 *   3. Neither present but params.accountId has a stored credential → throw a clear
 *      "resolvedApiKey missing" error (server wiring gap) instead of silently failing auth.
 *
 * @module relay/ai-complete-handler
 */

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'

const aiCompleteTracer = createTracer('agent:aiComplete')

// ── Types ─────────────────────────────────────────────────────────────────────

export type AICompleteParams = {
  prompt:  string
  format?: 'json' | 'text'
  taskId?: string
  model?:  string
  accountId?:      string  // credential-store account, mirrors AgentSpawnRequest.accountId
  resolvedApiKey?: string  // plaintext key forwarded by Orca Server, mirrors agent.spawn's resolvedApiKey
}

export type AICompleteResult = {
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

  // Resolve model: params > config > env > default
  const model = params.model
    ?? (config as unknown as Record<string, unknown>)['defaultModel'] as string | undefined
    ?? process.env['ORCA_AI_MODEL_ID']
    ?? 'claude-opus-4-5'

  // Resolve API key
  let apiKey: string | null
  try {
    apiKey = await resolveApiKey(model, params.accountId, params.resolvedApiKey, config, log)
  } catch (err: unknown) {
    span.fail(err, { model, taskId })
    throw err
  }
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
  if (model.startsWith('claude')) {return 'anthropic'}
  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) {return 'openai'}
  if (model.startsWith('gemini')) {return 'google'}
  return 'unknown'
}

// ── Key resolution ────────────────────────────────────────────────────────────

/** Tier 1: env vars injected by buildAgentEnv() at agent.spawn time. */
function resolveApiKeyFromEnv(model: string): string | null {
  if (model.startsWith('claude')) {
    return process.env['ANTHROPIC_API_KEY'] ?? null
  }
  if (model.startsWith('gpt') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4')) {
    return process.env['OPENAI_API_KEY'] ?? null
  }
  if (model.startsWith('gemini')) {
    return process.env['GOOGLE_API_KEY'] ?? null
  }
  // Unknown provider — try all
  return process.env['ANTHROPIC_API_KEY']
      ?? process.env['OPENAI_API_KEY']
      ?? process.env['GOOGLE_API_KEY']
      ?? null
}

/**
 * Tier 1: env var (see resolveApiKeyFromEnv).
 * Tier 2: params.resolvedApiKey — plaintext key Orca Server forwards per-call,
 *   the same pattern buildAgentEnv() uses for agent.spawn (see BUG-AG-HLD-002 /
 *   agent-spawner.ts). We never call agent-credential-store.ts to obtain a key
 *   value here — it only holds Layer-1 (still browser-encrypted) ciphertext,
 *   which this process cannot decrypt.
 * Tier 3: no plaintext key anywhere, but a credential file exists for
 *   accountId → throw a specific "resolvedApiKey missing" error so operators
 *   don't mistake a server wiring gap for "user never configured a key".
 */
async function resolveApiKey(
  model:          string,
  accountId:      string | undefined,
  resolvedApiKey: string | undefined,
  config:         AgentConfig,
  log:            AgentLogger,
): Promise<string | null> {
  const envKey = resolveApiKeyFromEnv(model)
  if (envKey) {return envKey}

  if (resolvedApiKey) {return resolvedApiKey}

  if (accountId) {
    const { readDecryptedKey } = await import('./agent-credential-store')
    const blob = await readDecryptedKey(accountId, config, log)
    if (blob) {
      throw new Error(
        `ai.complete: a credential exists for accountId="${accountId}" but no plaintext ` +
        'resolvedApiKey was provided. The Dev Server agent cannot decrypt the Layer-1 ' +
        '(browser-encrypted) credential blob itself — Orca Server must resolve it and pass ' +
        '"resolvedApiKey" in the ai.complete RPC params (same mechanism as agent.spawn).'
      )
    }
  }

  return null
}

// ── Provider dispatch ─────────────────────────────────────────────────────────

async function dispatch(
  model:  string,
  apiKey: string,
  prompt: string,
  format: string,
  log:    AgentLogger,
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
  throw new Error(`ai.complete: Unknown model provider for model "${model}". Supported prefixes: claude, gpt, o1, o3, o4, gemini.`)
}

// ── Anthropic Claude ──────────────────────────────────────────────────────────

async function callAnthropic(
  model:  string,
  apiKey: string,
  prompt: string,
  format: string,
  log:    AgentLogger,
): Promise<string> {
  const body: Record<string, unknown> = {
    model,
    max_tokens: 4096,
    messages: [{ role: 'user', content: prompt }],
  }
  if (format === 'json') {
    body['system'] = 'Respond with valid JSON only. No markdown code fences, no explanation outside the JSON object.'
  }

  const res = await fetch('https://api.anthropic.com/v1/messages', {
    method:  'POST',
    headers: {
      'Content-Type':      'application/json',
      'x-api-key':         apiKey,
      'anthropic-version': '2023-06-01',
    },
    body:   JSON.stringify(body),
    signal: AbortSignal.timeout(120_000),
  })

  if (!res.ok) {
    const errBody = await res.text().catch(() => res.statusText)
    log.error(`ai.complete Anthropic ${res.status}: ${errBody}`)
    throw new Error(`Anthropic API error ${res.status}: ${errBody}`)
  }

  const data = await res.json() as { content: { type: string; text?: string }[] }
  return data.content.find(c => c.type === 'text')?.text ?? ''
}

// ── OpenAI GPT / o-series ─────────────────────────────────────────────────────

async function callOpenAI(
  model:  string,
  apiKey: string,
  prompt: string,
  log:    AgentLogger,
): Promise<string> {
  const res = await fetch('https://api.openai.com/v1/chat/completions', {
    method:  'POST',
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
    const errBody = await res.text().catch(() => res.statusText)
    log.error(`ai.complete OpenAI ${res.status}: ${errBody}`)
    throw new Error(`OpenAI API error ${res.status}: ${errBody}`)
  }

  const data = await res.json() as { choices: { message: { content: string } }[] }
  return data.choices[0]?.message.content ?? ''
}

// ── Google Gemini ─────────────────────────────────────────────────────────────

async function callGoogle(
  model:  string,
  apiKey: string,
  prompt: string,
  log:    AgentLogger,
): Promise<string> {
  const url = `https://generativelanguage.googleapis.com/v1beta/models/${model}:generateContent?key=${apiKey}`
  const res = await fetch(url, {
    method:  'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      contents: [{ parts: [{ text: prompt }] }],
    }),
    signal: AbortSignal.timeout(120_000),
  })

  if (!res.ok) {
    const errBody = await res.text().catch(() => res.statusText)
    log.error(`ai.complete Google ${res.status}: ${errBody}`)
    throw new Error(`Google AI API error ${res.status}: ${errBody}`)
  }

  const data = await res.json() as {
    candidates: { content: { parts: Array<{ text?: string }> } }[]
  }
  return data.candidates[0]?.content.parts[0]?.text ?? ''
}
