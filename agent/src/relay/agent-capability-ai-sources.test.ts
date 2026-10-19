import { describe, expect, it } from 'vitest'
import { deriveAiSources, type ClaudeEntry } from './agent-capability-report'

const loggedIn: ClaudeEntry = { installed: true, auth: 'logged_in', flags: { tools: true, permissionMode: true, disallowedTools: true } }

describe('deriveAiSources', () => {
  it('reports the CLI login when claude is logged in and can disable tools', () => {
    expect(deriveAiSources(loggedIn, {})).toEqual(['cli_session'])
  })

  it('reports env keys by presence only', () => {
    expect(deriveAiSources({ installed: false, auth: 'unknown' }, { OPENAI_API_KEY: 'x' })).toEqual(['env_key'])
    expect(deriveAiSources({ installed: false, auth: 'unknown' }, { OPENAI_API_KEY: '' })).toEqual([])
  })

  it('lists both when both exist, keys first (the order ai.complete tries them)', () => {
    expect(deriveAiSources(loggedIn, { ANTHROPIC_API_KEY: 'k' })).toEqual(['env_key', 'cli_session'])
  })

  it.each([
    ['logged out', { installed: true, auth: 'logged_out', flags: { tools: true, permissionMode: true, disallowedTools: true } } as ClaudeEntry],
    ['login not probed', { installed: true, auth: 'unknown', flags: { tools: true, permissionMode: true, disallowedTools: true } } as ClaudeEntry],
    ['tools cannot be disabled', { installed: true, auth: 'logged_in', flags: { tools: false, permissionMode: true, disallowedTools: true } } as ClaudeEntry],
    ['flags unknown', { installed: true, auth: 'logged_in' } as ClaudeEntry],
    ['not installed', { installed: false, auth: 'logged_in', flags: { tools: true, permissionMode: true, disallowedTools: true } } as ClaudeEntry]
  ])('does not claim the CLI login when %s', (_name, claude) => {
    expect(deriveAiSources(claude, {})).toEqual([])
  })

  it('honours the ORCA_AI_CLI_SESSION kill switch', () => {
    expect(deriveAiSources(loggedIn, { ORCA_AI_CLI_SESSION: 'off' })).toEqual([])
  })
})
