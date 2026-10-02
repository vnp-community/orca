import { describe, expect, it, vi } from 'vitest'
import {
  displayMcpClientName,
  fetchMcpTerminalOrigins,
  isMcpOrigin,
  normalizeTerminalListOrigins,
  ptyIdToOriginKey,
  stopMcpOriginTerminal
} from './mcp-terminal-origin'

const rpc = vi.hoisted(() => ({ callRuntimeRpc: vi.fn() }))
vi.mock('@/runtime/runtime-rpc-client', () => rpc)

const origin = { type: 'mcp', clientName: 'Claude', mcpSessionId: 's1', userId: 'u1' }

describe('normalizeTerminalListOrigins', () => {
  it('reads the bare array shape keyed by ptyId', () => {
    expect(
      normalizeTerminalListOrigins([{ ptyId: 'p1', origin }, { ptyId: 'p2' }, null, 'x'])
    ).toEqual({ p1: origin })
  })

  it('reads the { terminals } shape keyed by handle', () => {
    expect(
      normalizeTerminalListOrigins({ terminals: [{ handle: 'h1', ptyId: 'p1', origin }] })
    ).toEqual({ h1: origin })
  })

  it('skips invalid origins and tolerates garbage input', () => {
    expect(
      normalizeTerminalListOrigins([
        { ptyId: 'a', origin: { ...origin, type: 'ui' } },
        { ptyId: 'b', origin: { ...origin, userId: 4 } },
        { ptyId: 'c', origin: 'mcp' }
      ])
    ).toEqual({})
    expect(normalizeTerminalListOrigins(undefined)).toEqual({})
    expect(normalizeTerminalListOrigins({ terminals: 'nope' })).toEqual({})
  })

  it('returns an empty map when no entry has an origin (UI-created terminals only)', () => {
    expect(normalizeTerminalListOrigins({ terminals: [{ handle: 'h', ptyId: 'p' }] })).toEqual({})
  })
})

describe('origin helpers', () => {
  it('validates origin shape', () => {
    expect(isMcpOrigin(origin)).toBe(true)
    expect(isMcpOrigin({ ...origin, clientName: undefined })).toBe(false)
    expect(isMcpOrigin(null)).toBe(false)
  })

  it('maps remote pty ids to the backend handle', () => {
    expect(ptyIdToOriginKey('remote:env-1@@abc')).toBe('abc')
    expect(ptyIdToOriginKey('remote:abc')).toBe('abc')
    expect(ptyIdToOriginKey('local-pty-1')).toBe('local-pty-1')
  })

  it('strips control and bidi characters and truncates long names', () => {
    expect(displayMcpClientName('Evil‮gnp\u0007.exe')).toBe('Evilgnp.exe')
    expect(displayMcpClientName('<script>alert(1)</script>')).toBe('<script>alert(1)</script>')
    expect(displayMcpClientName('x'.repeat(100))).toHaveLength(40)
    expect(displayMcpClientName('  ‮ ')).toBe('MCP client')
  })
})

describe('rpc wrappers', () => {
  it('lists via terminal.list and normalizes', async () => {
    rpc.callRuntimeRpc.mockResolvedValueOnce([{ ptyId: 'p1', origin }])
    expect(await fetchMcpTerminalOrigins({ kind: 'local' })).toEqual({ p1: origin })
    expect(rpc.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'local' },
      'terminal.list',
      {},
      { timeoutMs: 8000 }
    )
  })

  it('stops with terminal.stop and force-closes with terminal.close', async () => {
    rpc.callRuntimeRpc.mockResolvedValue({})
    await stopMcpOriginTerminal({ kind: 'local' }, 'h1')
    await stopMcpOriginTerminal({ kind: 'local' }, 'h1', true)
    expect(rpc.callRuntimeRpc).toHaveBeenCalledWith({ kind: 'local' }, 'terminal.stop', {
      terminal: 'h1'
    })
    expect(rpc.callRuntimeRpc).toHaveBeenCalledWith({ kind: 'local' }, 'terminal.close', {
      terminal: 'h1'
    })
  })
})
