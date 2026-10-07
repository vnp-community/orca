import { describe, it, expect, vi } from 'vitest'
import { dispatchAiRpc } from './agent-rpc-dispatch-ai'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

const dummyConfig: AgentConfig = {
  port: 8080,
  token: 'tok',
  workDir: '/tmp',
  toolPath: '/bin'
}

const dummyLogger: AgentLogger = {
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
  debug: vi.fn()
}

describe('dispatchAiRpc', () => {
  it('returns InvalidParams when prompt is empty or missing', async () => {
    const resEmpty = await dispatchAiRpc(
      { jsonrpc: '2.0', id: 1, method: 'ai.complete', params: { prompt: '   ' } },
      dummyConfig,
      dummyLogger
    )
    expect(resEmpty).toMatchObject({
      jsonrpc: '2.0',
      id: 1,
      error: { code: -32602, message: expect.stringContaining('prompt is required') }
    })
  })

  it('forwards params to handleAIComplete and returns result', async () => {
    vi.doMock('./ai-complete-handler', () => ({
      handleAIComplete: async (params: any) => ({
        content: 'completed text',
        model: params.model ?? 'default-model',
        provider: 'anthropic',
        latencyMs: 120,
        usage: { promptTokens: 10, completionTokens: 5, totalTokens: 15 }
      })
    }))

    const res = await dispatchAiRpc(
      {
        jsonrpc: '2.0',
        id: 2,
        method: 'ai.complete',
        params: { prompt: 'Hello', maxTokens: 1000 }
      },
      dummyConfig,
      dummyLogger
    )

    expect(res).toEqual({
      jsonrpc: '2.0',
      id: 2,
      result: {
        content: 'completed text',
        model: 'default-model',
        provider: 'anthropic',
        latencyMs: 120,
        usage: { promptTokens: 10, completionTokens: 5, totalTokens: 15 }
      }
    })
    vi.doUnmock('./ai-complete-handler')
  })

  it('surfaces error.data when handleAIComplete throws AICompleteProviderError', async () => {
    const { AICompleteProviderError } = await import('./ai-complete-handler')
    vi.doMock('./ai-complete-handler', () => ({
      AICompleteProviderError,
      handleAIComplete: async () => {
        throw new AICompleteProviderError('Anthropic API rate limit exceeded', {
          provider: 'anthropic',
          httpStatus: 429,
          retryable: true,
          reason: 'PROVIDER_HTTP_ERROR'
        })
      }
    }))

    const res = (await dispatchAiRpc(
      { jsonrpc: '2.0', id: 3, method: 'ai.complete', params: { prompt: 'Test' } },
      dummyConfig,
      dummyLogger
    )) as any

    expect(res.jsonrpc).toBe('2.0')
    expect(res.id).toBe(3)
    expect(res.error.code).toBe(-32000)
    expect(res.error.message).toContain('ai.complete failed:')
    expect(res.error.data).toEqual({
      provider: 'anthropic',
      httpStatus: 429,
      retryable: true,
      reason: 'PROVIDER_HTTP_ERROR'
    })
    vi.doUnmock('./ai-complete-handler')
  })

  it('omits error.data for generic errors', async () => {
    const { AICompleteProviderError } = await import('./ai-complete-handler')
    vi.doMock('./ai-complete-handler', () => ({
      AICompleteProviderError,
      handleAIComplete: async () => {
        throw new Error('generic network crash')
      }
    }))

    const res = (await dispatchAiRpc(
      { jsonrpc: '2.0', id: 4, method: 'ai.complete', params: { prompt: 'Test' } },
      dummyConfig,
      dummyLogger
    )) as any

    expect(res.jsonrpc).toBe('2.0')
    expect(res.id).toBe(4)
    expect(res.error.code).toBe(-32000)
    expect(res.error.message).toContain('generic network crash')
    expect(res.error.data).toBeUndefined()
    vi.doUnmock('./ai-complete-handler')
  })
})
