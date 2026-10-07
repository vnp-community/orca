import { describe, it, expect, beforeEach } from 'vitest'
import {
  detectClaudeFlags,
  readonlyUnsupportedReason,
  buildReadonlyArgs,
  readonlyUnsupportedError,
  resetClaudeFlagCacheForTests,
  READONLY_DEFAULT_TOOLS
} from './agent-readonly-tool-policy'

describe('agent-readonly-tool-policy', () => {
  beforeEach(() => {
    resetClaudeFlagCacheForTests()
  })

  describe('detectClaudeFlags', () => {
    it('returns flag support when help includes flags', async () => {
      const mockHelp = `
Usage: claude [options] [command]
Options:
  --tools <tools>              Comma-separated list of tools
  --permission-mode <mode>     Permission mode (default, plan, etc.)
  --disallowedTools <tools>    Disallowed tools
`
      const flags = await detectClaudeFlags({ PATH: '/usr/bin' }, async () => mockHelp)
      expect(flags).toEqual({
        tools: true,
        permissionMode: true,
        disallowedTools: true
      })
    })

    it('returns flag support when help lacks flags', async () => {
      const mockHelp = 'Usage: claude [options]\nOptions:\n  --version  Show version'
      const flags = await detectClaudeFlags({ PATH: '/usr/bin' }, async () => mockHelp)
      expect(flags).toEqual({
        tools: false,
        permissionMode: false,
        disallowedTools: false
      })
    })

    it('caches successful probes for 10 minutes by PATH', async () => {
      let callCount = 0
      const probe = async () => {
        callCount++
        return '--tools --permission-mode'
      }

      let fakeTime = 1000
      const now = () => fakeTime

      const flags1 = await detectClaudeFlags({ PATH: '/usr/bin' }, probe, now)
      expect(callCount).toBe(1)
      expect(flags1?.tools).toBe(true)

      // Same PATH within TTL: should use cache
      fakeTime += 5 * 60 * 1000 // +5 mins
      const flags2 = await detectClaudeFlags({ PATH: '/usr/bin' }, probe, now)
      expect(callCount).toBe(1)
      expect(flags2).toEqual(flags1)

      // Different PATH: should probe
      const flags3 = await detectClaudeFlags({ PATH: '/usr/local/bin' }, probe, now)
      expect(callCount).toBe(2)
      expect(flags3?.tools).toBe(true)

      // Same PATH after TTL (10m): should probe again
      fakeTime += 6 * 60 * 1000 // +6 mins (total 11m)
      await detectClaudeFlags({ PATH: '/usr/bin' }, probe, now)
      expect(callCount).toBe(3)
    })

    it('does not cache failed probes', async () => {
      let callCount = 0
      const failingProbe = async () => {
        callCount++
        throw new Error('command not found')
      }

      const res1 = await detectClaudeFlags({ PATH: '/usr/bin' }, failingProbe)
      expect(res1).toBeNull()
      expect(callCount).toBe(1)

      // Second call should retry because failures are not cached
      const res2 = await detectClaudeFlags({ PATH: '/usr/bin' }, failingProbe)
      expect(res2).toBeNull()
      expect(callCount).toBe(2)
    })

    it('single-flights concurrent calls for the same PATH', async () => {
      let callCount = 0
      const slowProbe = async () => {
        callCount++
        await new Promise((resolve) => setTimeout(resolve, 10))
        return '--tools --permission-mode'
      }

      const [res1, res2] = await Promise.all([
        detectClaudeFlags({ PATH: '/bin' }, slowProbe),
        detectClaudeFlags({ PATH: '/bin' }, slowProbe)
      ])

      expect(callCount).toBe(1)
      expect(res1).toEqual(res2)
    })
  })

  describe('readonlyUnsupportedReason', () => {
    it('returns CLAUDE_HELP_UNAVAILABLE when flags is null', () => {
      expect(readonlyUnsupportedReason(null)).toBe('CLAUDE_HELP_UNAVAILABLE')
    })

    it('returns FLAG_TOOLS_MISSING when tools flag is missing', () => {
      expect(
        readonlyUnsupportedReason({ tools: false, permissionMode: true, disallowedTools: false })
      ).toBe('FLAG_TOOLS_MISSING')
    })

    it('returns FLAG_PERMISSION_MODE_MISSING when permissionMode flag is missing', () => {
      expect(
        readonlyUnsupportedReason({ tools: true, permissionMode: false, disallowedTools: false })
      ).toBe('FLAG_PERMISSION_MODE_MISSING')
    })

    it('returns null when all required readonly flags are present', () => {
      expect(
        readonlyUnsupportedReason({ tools: true, permissionMode: true, disallowedTools: false })
      ).toBeNull()
    })
  })

  describe('buildReadonlyArgs', () => {
    it('returns exactly expected readonly flags and confirms no YOLO flags', () => {
      const args = buildReadonlyArgs()
      expect(args).toEqual(['--permission-mode', 'plan', '--tools', 'Read,Glob,Grep'])
      expect(READONLY_DEFAULT_TOOLS).toEqual(['Read', 'Glob', 'Grep'])

      // Asserts no YOLO flags
      expect(args).not.toContain('--dangerously-skip-permissions')
      expect(args).not.toContain('-y')
      expect(args).not.toContain('--yes')
    })
  })

  describe('readonlyUnsupportedError', () => {
    it('formats JSON-RPC error response with reason and detail', () => {
      const err = readonlyUnsupportedError('req-1', 'agent.execPrompt', 'FLAG_TOOLS_MISSING') as any
      expect(err.jsonrpc).toBe('2.0')
      expect(err.id).toBe('req-1')
      expect(err.error.code).toBe(-32602)
      expect(err.error.message).toBe('agent.execPrompt: READONLY_MODE_UNSUPPORTED (FLAG_TOOLS_MISSING)')
      expect(err.error.data).toEqual({
        reason: 'READONLY_MODE_UNSUPPORTED',
        detail: 'FLAG_TOOLS_MISSING'
      })
    })
  })
})
