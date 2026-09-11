// @vitest-environment happy-dom

import { act, createElement } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '@/store'
import type { DiffComment } from '../../../shared/types'

const callRuntimeRpc = vi.fn()
vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (...args: unknown[]) => callRuntimeRpc(...args),
  getActiveRuntimeTarget: (settings: { activeRuntimeEnvironmentId?: string | null } | undefined) =>
    settings?.activeRuntimeEnvironmentId
      ? { kind: 'environment', environmentId: settings.activeRuntimeEnvironmentId }
      : { kind: 'local' }
}))

// eslint-disable-next-line import/first -- Why: must import after vi.mock so the mocked module is what use-composed-all-notes-prompt resolves.
import { useComposedAllNotesPrompt } from './use-composed-all-notes-prompt'

const initialAppState = useAppStore.getInitialState()

let latestResult: { prompt: string; annotationIds: string[] } | undefined
const roots: Root[] = []

function HookProbe({ notes }: { notes: DiffComment[] }): null {
  latestResult = useComposedAllNotesPrompt('wt-1', 'my-worktree', notes)
  return null
}

async function renderHookProbe(notes: DiffComment[]): Promise<Root> {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => {
    root.render(createElement(HookProbe, { notes }))
  })
  return root
}

function note(overrides: Partial<DiffComment> = {}): DiffComment {
  return {
    id: 'c1',
    worktreeId: 'wt-1',
    filePath: 'src/foo.ts',
    lineNumber: 10,
    body: 'a comment',
    createdAt: 1000,
    side: 'modified',
    ...overrides
  }
}

describe('useComposedAllNotesPrompt', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    callRuntimeRpc.mockReset()
    useAppStore.setState(initialAppState, true)
  })

  afterEach(() => {
    for (const root of roots.splice(0)) {
      root.unmount()
    }
    vi.useRealTimers()
  })

  it('returns the client-side format immediately, before the debounced RPC resolves', async () => {
    useAppStore.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })
    callRuntimeRpc.mockResolvedValue({ prompt: 'composed', annotationIds: ['server-1'] })

    await renderHookProbe([note()])

    expect(latestResult?.prompt).toContain('src/foo.ts')
    expect(latestResult?.prompt).not.toBe('composed')
  })

  it('swaps to the composed prompt once annotation.composeReviewPrompt resolves', async () => {
    useAppStore.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })
    callRuntimeRpc.mockResolvedValue({
      prompt: 'composed with context',
      annotationIds: ['server-1']
    })

    await renderHookProbe([note()])
    await act(async () => {
      await vi.runAllTimersAsync()
    })

    expect(callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'annotation.composeReviewPrompt',
      { worktreeId: 'wt-1', worktreeName: 'my-worktree' },
      { timeoutMs: 8000 }
    )
    expect(latestResult?.prompt).toBe('composed with context')
    expect(latestResult?.annotationIds).toEqual(['server-1'])
  })

  it('keeps the client-side fallback when the RPC fails', async () => {
    useAppStore.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })
    callRuntimeRpc.mockRejectedValue(new Error('annotation-service unreachable'))

    await renderHookProbe([note()])
    await act(async () => {
      await vi.runAllTimersAsync()
    })

    expect(latestResult?.prompt).toContain('src/foo.ts')
    expect(latestResult?.annotationIds).toEqual(['c1'])
  })

  it('does not call the RPC for the desktop/local target', async () => {
    useAppStore.setState({ settings: { activeRuntimeEnvironmentId: null } as never })

    await renderHookProbe([note()])
    await act(async () => {
      await vi.runAllTimersAsync()
    })

    expect(callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('does not call the RPC when there are no unsent notes', async () => {
    useAppStore.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await renderHookProbe([])
    await act(async () => {
      await vi.runAllTimersAsync()
    })

    expect(callRuntimeRpc).not.toHaveBeenCalled()
    expect(latestResult).toEqual({ prompt: '', annotationIds: [] })
  })
})
