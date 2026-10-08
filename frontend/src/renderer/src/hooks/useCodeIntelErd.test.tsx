// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => ({
    state: 'ready',
    worktreeId: 'wt',
    projectId: 'p',
    environmentId: null
  })
}))

import { useAppStore } from '@/store'
import { useCodeIntelErd, resolveErdSelection } from './useCodeIntelErd'
import type { CodeIntelCallFn, CodeIntelCallOutcome } from './useCodeIntelQuery'
import { sampleErdModel } from '../components/review-map/erd/erd-model.fixture'

const SERVICES = {
  services: [
    { name: 'empty', dialects: ['postgres'], tableCount: 0 },
    { name: 'infra-fleet', dialects: ['postgres', 'mysql'], tableCount: 3 },
    { name: 'auth', dialects: ['postgres'], tableCount: 2 }
  ]
}
const ok = (result: unknown): CodeIntelCallOutcome => ({ ok: true, result, meta: null })

async function flush() {
  await act(async () => {
    for (let i = 0; i < 6; i++) {
      await Promise.resolve()
    }
  })
}

beforeEach(() => {
  useAppStore.setState({
    codeIntelSupportState: { state: 'enabled', effective: { codeIntelEnabled: true } } as never,
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0
  })
})
afterEach(cleanup)

const scope = { kind: 'branch', baseRef: 'main', includeUncommitted: true } as const
const baseArgs = {
  worktreeId: 'wt',
  environmentId: null,
  scope,
  service: null,
  dialect: null
} as const

describe('resolveErdSelection', () => {
  it('defaults to the first service with tables and its first dialect', () => {
    expect(resolveErdSelection(SERVICES.services as never, null, null)).toEqual({
      service: 'infra-fleet',
      dialect: 'postgres'
    })
    expect(resolveErdSelection(SERVICES.services as never, 'infra-fleet', 'mysql').dialect).toBe(
      'mysql'
    )
    expect(resolveErdSelection(SERVICES.services as never, 'auth', 'mysql').dialect).toBe(
      'postgres'
    )
    expect(resolveErdSelection([], null, null)).toEqual({ service: null, dialect: null })
  })
})

describe('useCodeIntelErd', () => {
  it('runs two phases: services first, then the model for the resolved service', async () => {
    const call = vi.fn<CodeIntelCallFn>(async (_w, _m, params) =>
      'service' in params ? ok(sampleErdModel()) : ok(SERVICES)
    )
    const { result } = renderHook(() => useCodeIntelErd({ ...baseArgs, callFn: call }))
    await flush()
    expect(call.mock.calls[0][2]).toEqual({})
    const modelCall = call.mock.calls.find((c) => 'service' in c[2])!
    expect(modelCall[2]).toMatchObject({
      service: 'infra-fleet',
      dialect: 'postgres',
      base: 'main',
      includeAccess: true,
      includeInferred: false
    })
    expect(result.current.model.status).toBe('success')
    expect(result.current.model.data?.service).toBe('infra-fleet')
    expect(result.current.resolved.service).toBe('infra-fleet')
  })

  it('hides the previous model while another service loads and ignores the stale response', async () => {
    let releaseFirst: (o: CodeIntelCallOutcome) => void = () => undefined
    const call = vi.fn<CodeIntelCallFn>((_w, _m, params) => {
      if (!('service' in params)) {
        return Promise.resolve(ok(SERVICES))
      }
      if (params.service === 'infra-fleet') {
        return new Promise((r) => (releaseFirst = r))
      }
      return Promise.resolve(ok(sampleErdModel({ service: 'auth' })))
    })
    const { result, rerender } = renderHook(
      (p: { service: string | null }) =>
        useCodeIntelErd({ ...baseArgs, service: p.service, callFn: call }),
      {
        initialProps: { service: null as string | null }
      }
    )
    await flush()
    rerender({ service: 'auth' })
    await flush()
    expect(result.current.model.data?.service).toBe('auth')
    await act(async () => releaseFirst(ok(sampleErdModel({ service: 'infra-fleet' }))))
    await flush()
    expect(result.current.model.data?.service).toBe('auth')
  })

  it('does not call anything when disabled', async () => {
    const call = vi.fn<CodeIntelCallFn>()
    const { result } = renderHook(() =>
      useCodeIntelErd({ ...baseArgs, enabled: false, callFn: call })
    )
    await flush()
    expect(call).not.toHaveBeenCalled()
    expect(result.current.model.status).toBe('idle')
  })

  it('does not call anything when code intel is switched off', async () => {
    useAppStore.setState({
      codeIntelSupportState: { state: 'disabled', effective: { codeIntelEnabled: false } } as never
    })
    const call = vi.fn<CodeIntelCallFn>()
    renderHook(() => useCodeIntelErd({ ...baseArgs, callFn: call }))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })

  it('does not reload on a changed push; only raises isStale', async () => {
    const call = vi.fn<CodeIntelCallFn>(async (_w, _m, params) =>
      'service' in params ? ok(sampleErdModel()) : ok(SERVICES)
    )
    const { result } = renderHook(() => useCodeIntelErd({ ...baseArgs, callFn: call }))
    await flush()
    const calls = call.mock.calls.length
    act(() =>
      useAppStore.setState((s) => ({
        codeIntelWorktreeState: {
          ...s.codeIntelWorktreeState,
          wt: { ...(s.codeIntelWorktreeState.wt as object), stale: true } as never
        }
      }))
    )
    await flush()
    expect(call.mock.calls.length).toBe(calls)
    expect(result.current.isStale).toBe(true)
  })

  it('surfaces a classified error from the model phase', async () => {
    const call = vi.fn<CodeIntelCallFn>(async (_w, _m, params) =>
      'service' in params
        ? { ok: false, error: { kind: 'too-large', code: null, message: 'x', retryable: false } }
        : ok(SERVICES)
    )
    const { result } = renderHook(() => useCodeIntelErd({ ...baseArgs, callFn: call }))
    await flush()
    expect(result.current.model.status).toBe('error')
    expect(result.current.model.error?.kind).toBe('too-large')
    expect(result.current.services.status).toBe('success')
  })
})
