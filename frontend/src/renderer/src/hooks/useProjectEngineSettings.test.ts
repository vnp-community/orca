// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useProjectEngineSettings } from './useProjectEngineSettings'

const stored = {
  projectId: 'p1',
  engine: 'openspec',
  openspecMinVersion: '1.2.3',
  solutionAgent: true,
  executionMode: 'step_by_step',
  outcomeReviewEnabled: false,
  version: 4
}

const fail = (kind: string, message: string, backendCode?: string) => ({
  ok: false,
  error: { kind, code: 'runtime_error', message, ...(backendCode ? { backendCode } : {}) }
})

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)

describe('useProjectEngineSettings', () => {
  it('reads the settings of the project', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: stored })
    const { result } = renderHook(() => useProjectEngineSettings('p1'))
    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(callRequestRpc).toHaveBeenCalledWith('request.engineSettings', { projectId: 'p1' })
    expect(result.current.settings).toEqual(stored)
  })

  it.each([
    ['unsupported', fail('unsupported', 'm'), 'unsupported'],
    ['request management off', fail('feature_off', 'm'), 'unsupported'],
    ['forbidden', fail('forbidden', 'REQUEST_FORBIDDEN: no'), 'forbidden'],
    ['a network error', fail('network', 'down'), 'error'],
    ['an unreadable answer', { ok: true, value: { nope: 1 } }, 'error']
  ])('reports %s', async (_name, response, expected) => {
    callRequestRpc.mockResolvedValue(response)
    const { result } = renderHook(() => useProjectEngineSettings('p1'))
    await waitFor(() => expect(result.current.status).toBe(expected))
    expect(result.current.settings).toBeNull()
  })

  it('saves the new mode together with everything it read, and the expected version', async () => {
    callRequestRpc.mockResolvedValueOnce({ ok: true, value: stored }).mockResolvedValueOnce({
      ok: true,
      value: { ...stored, executionMode: 'continuous', version: 5 }
    })
    const { result } = renderHook(() => useProjectEngineSettings('p1'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    let outcome: string | undefined
    await act(async () => {
      outcome = await result.current.saveExecutionMode('continuous')
    })
    expect(outcome).toBe('saved')
    expect(callRequestRpc).toHaveBeenLastCalledWith('request.engineSettingsSet', {
      projectId: 'p1',
      engine: 'openspec',
      openspecMinVersion: '1.2.3',
      solutionAgent: true,
      executionMode: 'continuous',
      expectedVersion: 4
    })
    expect(result.current.settings).toMatchObject({ executionMode: 'continuous', version: 5 })
  })

  it('loads the latest values again after a version conflict', async () => {
    callRequestRpc
      .mockResolvedValueOnce({ ok: true, value: stored })
      .mockResolvedValueOnce(
        fail(
          'invalid_state',
          'REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT: stale',
          'REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT'
        )
      )
      .mockResolvedValueOnce({ ok: true, value: { ...stored, engine: 'native', version: 6 } })
    const { result } = renderHook(() => useProjectEngineSettings('p1'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    let outcome: string | undefined
    await act(async () => {
      outcome = await result.current.saveExecutionMode('continuous')
    })
    expect(outcome).toBe('conflict')
    await waitFor(() => expect(result.current.settings?.version).toBe(6))
    expect(callRequestRpc).toHaveBeenCalledTimes(3)
  })

  it('reports a refused save and other failures without touching what is shown', async () => {
    callRequestRpc
      .mockResolvedValueOnce({ ok: true, value: stored })
      .mockResolvedValueOnce(fail('forbidden', 'REQUEST_FORBIDDEN: admin only'))
      .mockResolvedValueOnce(fail('network', 'down'))
    const { result } = renderHook(() => useProjectEngineSettings('p1'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    let first: string | undefined
    let second: string | undefined
    await act(async () => {
      first = await result.current.saveExecutionMode('continuous')
    })
    await act(async () => {
      second = await result.current.saveExecutionMode('continuous')
    })
    expect(first).toBe('forbidden')
    expect(second).toBe('error')
    expect(result.current.settings).toEqual(stored)
  })

  it('sends one save for a double click', async () => {
    let release: (v: unknown) => void = () => {}
    callRequestRpc
      .mockResolvedValueOnce({ ok: true, value: stored })
      .mockReturnValueOnce(new Promise((resolve) => (release = resolve)))
    const { result } = renderHook(() => useProjectEngineSettings('p1'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    let first: Promise<string> | undefined
    let second: string | undefined
    await act(async () => {
      first = result.current.saveExecutionMode('continuous')
      second = await result.current.saveExecutionMode('continuous')
    })
    expect(second).toBe('error')
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
    await act(async () => {
      release({ ok: true, value: { ...stored, executionMode: 'continuous', version: 5 } })
      await first
    })
  })

  it('re-reads when the project changes and ignores a late answer for the old one', async () => {
    let resolveOld: (v: unknown) => void = () => {}
    callRequestRpc.mockImplementation((_m: string, args: { projectId: string }) =>
      args.projectId === 'p1'
        ? new Promise((resolve) => (resolveOld = resolve))
        : Promise.resolve({ ok: true, value: { ...stored, projectId: 'p2', engine: 'native' } })
    )
    const { result, rerender } = renderHook(({ id }) => useProjectEngineSettings(id), {
      initialProps: { id: 'p1' }
    })
    rerender({ id: 'p2' })
    await waitFor(() => expect(result.current.settings?.projectId).toBe('p2'))
    await act(async () => resolveOld({ ok: true, value: stored }))
    expect(result.current.settings?.projectId).toBe('p2')
  })
})
