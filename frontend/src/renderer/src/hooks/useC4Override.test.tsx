// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const call = vi.fn()
vi.mock('../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ call }) }))

import { classifyC4SaveError, useC4Override } from './useC4Override'

const err = (kind: string, code: string | null = null, data: Record<string, unknown> | null = null) => ({
  ok: false, error: { kind, code, message: 'm', data, retryable: false }
})
beforeEach(() => call.mockReset())
const mount = async () => {
  const h = renderHook(() => useC4Override({ worktreeId: 'w', environmentId: null, container: 'c' }))
  await act(async () => {})
  return h
}

describe('useC4Override', () => {
  it('treats not-found as an empty version-0 record', async () => {
    call.mockResolvedValueOnce(err('not-found'))
    const { result } = await mount()
    expect(result.current.status).toBe('ready')
    expect(result.current.record).toMatchObject({ document: '', version: 0 })
  })

  it('saves with the loaded version and returns warnings', async () => {
    call.mockResolvedValueOnce({ ok: true, result: { document: 'a: 1', version: 2 } })
    const { result } = await mount()
    call.mockResolvedValueOnce({ ok: true, result: { version: 3, warnings: [{ code: 'x', message: 'y' }] } })
    let out
    await act(async () => { out = await result.current.save('a: 2') })
    expect(call.mock.calls[1][2]).toEqual({ container: 'c', document: 'a: 2', expectedVersion: 2 })
    expect(out).toEqual({ ok: true, version: 3, warnings: [{ code: 'x', message: 'y' }] })
  })

  it('does not send two saves for a double click', async () => {
    call.mockResolvedValueOnce({ ok: true, result: { document: '', version: 1 } })
    const { result } = await mount()
    let release: (v: unknown) => void = () => {}
    call.mockReturnValueOnce(new Promise((r) => { release = r }))
    await act(async () => {
      void result.current.save('a: 1')
      void result.current.save('a: 1')
      release({ ok: true, result: { version: 2, warnings: [] } })
    })
    expect(call).toHaveBeenCalledTimes(2)
  })

  it('overwrite re-reads the latest version first', async () => {
    call.mockResolvedValueOnce({ ok: true, result: { document: '', version: 1 } })
    const { result } = await mount()
    call.mockResolvedValueOnce({ ok: true, result: { document: 'n', version: 7 } })
    call.mockResolvedValueOnce({ ok: true, result: { version: 8, warnings: [] } })
    await act(async () => { await result.current.overwrite('mine') })
    expect(call.mock.calls[2][2]).toMatchObject({ expectedVersion: 7 })
  })

  it('goes read-only on forbidden', async () => {
    call.mockResolvedValueOnce({ ok: true, result: { document: '', version: 1 } })
    const { result } = await mount()
    call.mockResolvedValueOnce(err('forbidden'))
    await act(async () => { await result.current.save('a: 1') })
    expect(result.current.readOnly).toBe(true)
  })
})

describe('classifyC4SaveError', () => {
  const base = { message: 'm', retryable: false }
  it('maps the contract codes', () => {
    expect(classifyC4SaveError({ ...base, kind: 'conflict', code: null, data: { currentVersion: 4 } })).toEqual({ kind: 'conflict', currentVersion: 4 })
    expect(classifyC4SaveError({ ...base, kind: 'validation', code: 'CODEINTEL_INVALID_PARAMS', data: { field: 'document', reason: 'bad' } } as never)).toEqual({ kind: 'invalid', field: 'document', reason: 'bad' })
    expect(classifyC4SaveError({ ...base, kind: 'validation', code: 'CODEINTEL_PAYLOAD_TOO_LARGE', data: { limit: 65536 } } as never)).toEqual({ kind: 'too-large', limit: 65536 })
    expect(classifyC4SaveError({ ...base, kind: 'timeout', code: null, data: null })).toEqual({ kind: 'offline' })
  })
})
