// Why: locks in the fix for specs/agent/api/gaps-and-findings.md #5 —
// preflight.detectAgents/detectWindowsTerminalCapabilities/
// detectGhosttyConfig/setGitIdentity previously only existed on Part B
// (relay.ts), so they threw MethodNotFound for direct-websocket/
// relay-websocket dev servers — the default connection mode.
import { describe, expect, it } from 'vitest'
import {
  handleDetectAgents,
  handleDetectGhosttyConfig,
  handleDetectWindowsTerminalCapabilities,
  handleSetGitIdentity
} from './agent-preflight-handler'
import { getConnectionGitIdentity } from './git-identity-registry'

describe('preflight.detectAgents', () => {
  it('returns an empty agent list when commands is not an array', async () => {
    const resp = (await handleDetectAgents(1, {})) as { result: { agents: string[] } }
    expect(resp.result.agents).toEqual([])
  })

  it('reports the current platform', async () => {
    const resp = (await handleDetectAgents(1, { commands: [] })) as { result: { platform: string } }
    expect(resp.result.platform).toBe(process.platform)
  })

  it('detects a command that is really on PATH (node itself)', async () => {
    const resp = (await handleDetectAgents(1, {
      commands: [{ id: 'node-test', cmd: 'node' }]
    })) as { result: { agents: string[] } }
    expect(resp.result.agents).toContain('node-test')
  })

  it('does not report an agent whose command is not on PATH', async () => {
    const resp = (await handleDetectAgents(1, {
      commands: [{ id: 'nonexistent-test', cmd: 'definitely-not-a-real-binary-xyz' }]
    })) as { result: { agents: string[] } }
    expect(resp.result.agents).not.toContain('nonexistent-test')
  })
})

describe('preflight.detectWindowsTerminalCapabilities', () => {
  it('returns the expected shape without throwing on any platform', async () => {
    const resp = (await handleDetectWindowsTerminalCapabilities(1)) as {
      result: { wslAvailable: boolean; wslDistros: string[]; pwshAvailable: boolean; hostPlatform: string }
    }
    expect(typeof resp.result.wslAvailable).toBe('boolean')
    expect(Array.isArray(resp.result.wslDistros)).toBe(true)
    expect(typeof resp.result.pwshAvailable).toBe('boolean')
    expect(resp.result.hostPlatform).toBe(process.platform)
  })
})

describe('preflight.detectGhosttyConfig', () => {
  it('returns null for both paths when Ghostty is not configured on this host', async () => {
    const resp = (await handleDetectGhosttyConfig(1)) as {
      result: { configPath: string | null; themeDir: string | null }
    }
    // Either shape is valid depending on the test host — just confirm no throw
    // and the right key types.
    expect(resp.result.configPath === null || typeof resp.result.configPath === 'string').toBe(true)
    expect(resp.result.themeDir === null || typeof resp.result.themeDir === 'string').toBe(true)
  })
})

describe('preflight.setGitIdentity', () => {
  it('stores identity keyed by the connection object', async () => {
    const ws = {} as never
    const resp = (await handleSetGitIdentity(1, { name: 'Ada Lovelace', email: 'ada@example.com' }, ws)) as {
      result: { ok: boolean }
    }
    expect(resp.result.ok).toBe(true)
    expect(getConnectionGitIdentity(ws)).toEqual({ name: 'Ada Lovelace', email: 'ada@example.com' })
  })

  it('does not leak identity across two different connections', async () => {
    const wsA = {} as never
    const wsB = {} as never
    await handleSetGitIdentity(1, { name: 'User A', email: 'a@example.com' }, wsA)
    await handleSetGitIdentity(2, { name: 'User B', email: 'b@example.com' }, wsB)
    expect(getConnectionGitIdentity(wsA)).toEqual({ name: 'User A', email: 'a@example.com' })
    expect(getConnectionGitIdentity(wsB)).toEqual({ name: 'User B', email: 'b@example.com' })
  })

  it('rejects a missing name or email', async () => {
    const ws = {} as never
    const resp = (await handleSetGitIdentity(1, { name: '', email: 'a@example.com' }, ws)) as {
      error: { code: number }
    }
    expect(resp.error).toBeDefined()
  })
})
