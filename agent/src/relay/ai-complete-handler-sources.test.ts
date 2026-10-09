import type * as CliSessionModule from './ai-complete-cli-session'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const completeWithCliSession = vi.fn()
const cliSessionSupported = vi.fn()
vi.mock('./ai-complete-cli-session', async importOriginal => {
  const real = await importOriginal<typeof CliSessionModule>()
  return { ...real, completeWithCliSession, cliSessionSupported }
})

const log = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() } as never
const config = {} as never

async function run(params: Record<string, unknown>) {
  const { handleAIComplete } = await import('./ai-complete-handler')
  return handleAIComplete({ prompt: 'classify', ...params } as never, config, log)
}

describe('ai.complete source order', () => {
  const saved = { ...process.env }
  beforeEach(() => {
    for (const k of ['ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GOOGLE_API_KEY', 'ORCA_AI_MODEL_ID', 'ORCA_AI_CLI_SESSION']) {
      delete process.env[k]
    }
    completeWithCliSession.mockReset()
    cliSessionSupported.mockReset().mockResolvedValue(true)
    completeWithCliSession.mockResolvedValue({ content: 'from-cli', usage: { totalTokens: 5 }, model: 'claude-x' })
  })
  afterEach(() => {
    process.env = { ...saved }
  })

  it('uses the CLI login when no key exists anywhere, without naming a model', async () => {
    const result = await run({ format: 'json' })
    expect(result).toMatchObject({ content: 'from-cli', provider: 'claude_cli', model: 'claude-x' })
    expect(completeWithCliSession).toHaveBeenCalledWith({ prompt: 'classify', format: 'json', model: undefined })
  })

  it('forwards an explicitly requested Claude model to the CLI', async () => {
    await run({ model: 'claude-sonnet-4' })
    expect(completeWithCliSession.mock.calls[0][0].model).toBe('claude-sonnet-4')
  })

  it('does not call the CLI when the kill switch is off, and says how to fix it', async () => {
    process.env['ORCA_AI_CLI_SESSION'] = 'off'
    await expect(run({})).rejects.toMatchObject({
      errorData: { reason: 'NO_API_KEY' },
      message: expect.stringContaining('claude auth login')
    })
    expect(completeWithCliSession).not.toHaveBeenCalled()
  })

  it('does not call the CLI when this claude build cannot disable tools', async () => {
    cliSessionSupported.mockResolvedValue(false)
    await expect(run({})).rejects.toMatchObject({ errorData: { reason: 'NO_API_KEY' } })
    expect(completeWithCliSession).not.toHaveBeenCalled()
  })

  it('never routes a non-Claude model to the Claude CLI', async () => {
    await expect(run({ model: 'gpt-4o' })).rejects.toMatchObject({ errorData: { reason: 'NO_API_KEY', provider: 'openai' } })
    expect(completeWithCliSession).not.toHaveBeenCalled()
  })

  it('propagates CLI failures with their structured reason', async () => {
    const { AICompleteProviderError } = await import('./ai-complete-errors')
    completeWithCliSession.mockRejectedValue(
      new AICompleteProviderError('Not logged in', { provider: 'claude_cli', httpStatus: null, retryable: false, reason: 'CLI_NOT_LOGGED_IN' })
    )
    await expect(run({})).rejects.toMatchObject({ errorData: { reason: 'CLI_NOT_LOGGED_IN' } })
  })
})
