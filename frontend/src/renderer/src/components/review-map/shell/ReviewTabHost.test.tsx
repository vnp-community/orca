// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { useAppStore } from '@/store'
import { ReviewTabHost } from './ReviewTabHost'

const WT = 'repo1::/path/wt1'

beforeEach(() => {
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never
  useAppStore.setState({
    repos: [{ id: 'repo1', projectId: 'proj' } as never],
    worktreesByRepo: {
      repo1: [
        { id: WT, repoId: 'repo1', path: '/path/wt1', branch: 'b', baseRef: 'origin/main' } as never
      ]
    },
    codeIntelSupportState: { state: 'enabled' }
  })
})
afterEach(cleanup)

describe('ReviewTabHost', () => {
  it('lazily mounts the workspace for the tab worktree (client not ready => inline error, no crash)', { timeout: 15000 }, async () => {
    render(<ReviewTabHost worktreeId={WT} tabId="tab-1" />)
    expect(screen.getByTestId('review-tab-host')).toBeTruthy()
    // Why: the lazy chunk now pulls in the dock, turn and report modules; a cold transform can pass 1 s.
    expect(await screen.findByTestId('review-workspace', {}, { timeout: 8000 })).toBeTruthy()
    expect(await screen.findByRole('alert', {}, { timeout: 8000 })).toBeTruthy()
  })
})
