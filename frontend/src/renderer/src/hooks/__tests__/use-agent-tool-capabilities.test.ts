// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { PIS_TOOL_NAMES } from '../use-agent-tool-capabilities'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
}))

vi.mock('../../runtime/runtime-compatibility-cache', () => ({
  ensureRuntimeEnvironmentCompatible: vi.fn().mockResolvedValue(undefined),
}))

const mockCallRpc = vi.mocked(callRuntimeRpc)

// eslint-disable-next-line @typescript-eslint/no-require-imports
const { useAgentToolCapabilities } = await import('../use-agent-tool-capabilities')

describe('useAgentToolCapabilities', () => {
  beforeEach(() => vi.clearAllMocks())

  it('hasPisTools=true when catalog contains pis_query_crs', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [{ name: 'pis_query_crs', description: 'query crs' }] })
    const { result } = renderHook(() => useAgentToolCapabilities('env-1'))
    await act(() => result.current.fetchCapabilities())
    expect(result.current.hasPisTools).toBe(true)
  })

  it('hasPisTools=false when catalog has no pis_* tools', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [{ name: 'orca.create_task', description: '...' }] })
    const { result } = renderHook(() => useAgentToolCapabilities('env-1'))
    await act(() => result.current.fetchCapabilities())
    expect(result.current.hasPisTools).toBe(false)
  })

  it('hasPisTools=false when catalog is empty (ProductsAgentID not set)', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [] })
    const { result } = renderHook(() => useAgentToolCapabilities('env-1'))
    await act(() => result.current.fetchCapabilities())
    expect(result.current.hasPisTools).toBe(false)
  })

  it('PIS_TOOL_NAMES has exactly 5 tools', () => {
    expect(PIS_TOOL_NAMES).toHaveLength(5)
    expect(PIS_TOOL_NAMES).toContain('pis_query_crs')
    expect(PIS_TOOL_NAMES).toContain('pis_kg_search')
    expect(PIS_TOOL_NAMES).toContain('pis_query_feature_context')
    expect(PIS_TOOL_NAMES).toContain('pis_query_hld')
    expect(PIS_TOOL_NAMES).toContain('pis_query_adr')
  })

  it('loading=false after fetchCapabilities completes', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [] })
    const { result } = renderHook(() => useAgentToolCapabilities())
    await act(() => result.current.fetchCapabilities())
    expect(result.current.loading).toBe(false)
  })
})
