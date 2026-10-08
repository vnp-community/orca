// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const callEnvelope = vi.fn()
vi.mock('../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ callEnvelope }) }))

import { parseC4Architecture, useC4Architecture } from './useC4Architecture'
import { invalidateCodeIntelViewCache, resetCodeIntelViewCache } from './useCodeIntelViewLoad'

const env = (data: unknown) => ({ ok: true, envelope: { data, truncated: false, totalCount: 0, stale: false } })
const containers = [{ id: 'a', name: 'a', path: 'a', kind: 'service' }]
const flush = () => act(async () => { await Promise.resolve() })

beforeEach(() => { callEnvelope.mockReset(); resetCodeIntelViewCache() })

describe('parseC4Architecture', () => {
  it('tolerates missing arrays and a null view', () => {
    expect(parseC4Architecture({})).toEqual({ containers: [], view: null })
    const r = parseC4Architecture({ containers, view: { container: containers[0] } })
    expect(r.view).toMatchObject({ components: [], relations: [], externals: [], warnings: [], hasOverrides: false })
  })
})

describe('useC4Architecture', () => {
  it('first call has no container; later calls carry container and includeHidden', async () => {
    callEnvelope.mockResolvedValue(env({ containers, view: null }))
    const { result, rerender } = renderHook((p) => useC4Architecture('w', null, p), { initialProps: { container: null as string | null, includeHidden: false } })
    await flush()
    expect(callEnvelope.mock.calls[0][2]).toEqual({})
    expect(result.current.data?.view).toBeNull()
    rerender({ container: 'a', includeHidden: true })
    await flush()
    expect(callEnvelope.mock.calls[1][2]).toEqual({ container: 'a', includeHidden: true })
  })

  it('caches by (container, includeHidden) and refetch bypasses the cache', async () => {
    callEnvelope.mockResolvedValue(env({ containers, view: null }))
    const { rerender, result } = renderHook((p) => useC4Architecture('w', null, p), { initialProps: { container: 'a', includeHidden: false } })
    await flush()
    rerender({ container: 'b', includeHidden: false })
    await flush()
    rerender({ container: 'a', includeHidden: false })
    await flush()
    expect(callEnvelope).toHaveBeenCalledTimes(2)
    act(() => result.current.refetch())
    await flush()
    expect(callEnvelope).toHaveBeenCalledTimes(3)
    invalidateCodeIntelViewCache('w', 'codeIntel.architecture')
    rerender({ container: 'b', includeHidden: false })
    await flush()
    expect(callEnvelope).toHaveBeenCalledTimes(4)
  })

  it('does not call when disabled and reports errors', async () => {
    const { result } = renderHook(() => useC4Architecture('w', null, { container: null, includeHidden: false, enabled: false }))
    await flush()
    expect(callEnvelope).not.toHaveBeenCalled()
    expect(result.current.status).toBe('idle')
    callEnvelope.mockResolvedValue({ ok: false, error: { kind: 'timeout', code: null, message: 'slow', data: null, retryable: true } })
    const r2 = renderHook(() => useC4Architecture('w2', null, { container: null, includeHidden: false }))
    await flush()
    expect(r2.result.current.status).toBe('error')
  })
})
