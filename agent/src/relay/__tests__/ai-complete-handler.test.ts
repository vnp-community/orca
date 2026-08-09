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
    // handleAIComplete calls aiCompleteTracer.start() BEFORE the `!prompt.trim()`
    // check (so promptLength/taskId/format are always captured, even on the
    // empty-prompt failure path) — a start event IS emitted here, followed by a fail.
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
      json: async () => ({ content: [{ type: 'text', text: 'ok response body' }] }),
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
