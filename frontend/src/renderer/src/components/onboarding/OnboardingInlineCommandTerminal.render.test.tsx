// @vitest-environment happy-dom

// Why: regression guard for a live bug introduced (and fixed) in this same
// pass — devServerId can start null and resolve to a real value shortly
// after mount (the connected-dev-server list loads asynchronously). The
// create-tab effect must NOT treat devServerId as a dependency, or that
// first resolution tears down and recreates the tab, destroying a PTY that
// may have just started streaming (the exact create→destroy churn
// BUG-FE-PTY-001 was fought over — see that memory). Found live 2026-08-30
// on the onboarding CLI-install terminal.
import { describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

const createTab = vi.fn(
  (
    worktreeId: string,
    _groupId?: string,
    _shellOverride?: string,
    _options?: Record<string, unknown>
  ) => ({
    id: `tab-for-${worktreeId}`
  })
)
const closeTab = vi.fn()
const setActiveTabForWorktree = vi.fn()
const setTabCustomTitle = vi.fn()

vi.mock('@/store', () => {
  const state = { createTab, closeTab, setActiveTabForWorktree, setTabCustomTitle }
  const useAppStore = Object.assign((selector: (s: typeof state) => unknown) => selector(state), {
    getState: () => state
  })
  return { useAppStore }
})

vi.mock('@/components/terminal-pane/TerminalPane', () => ({
  default: function TerminalPane() {
    return null
  }
}))

vi.stubGlobal('window', {
  ...globalThis.window,
  matchMedia: () => ({ matches: true }),
  api: { app: { getFloatingTerminalCwd: () => Promise.resolve('') } },
  dispatchEvent: () => true,
  requestAnimationFrame: (cb: FrameRequestCallback) => setTimeout(() => cb(0), 0),
  cancelAnimationFrame: (id: number) => clearTimeout(id)
})

async function renderWithDevServerId(devServerId: string | null): Promise<{
  root: Root
  container: HTMLElement
  rerender: (nextDevServerId: string | null) => Promise<void>
}> {
  const { OnboardingInlineCommandTerminal } = await import('./OnboardingInlineCommandTerminal')
  const container = document.createElement('div')
  const root = createRoot(container)
  const renderOnce = (id: string | null): void => {
    root.render(
      <OnboardingInlineCommandTerminal
        command="npx orca-cli install"
        title="Skill setup"
        ariaLabel="Skill setup terminal"
        worktreeId="feature-tip-cli-skills-terminal"
        devServerId={id}
      />
    )
  }
  await act(async () => {
    renderOnce(devServerId)
    await Promise.resolve()
  })
  return {
    root,
    container,
    rerender: async (nextDevServerId) => {
      await act(async () => {
        renderOnce(nextDevServerId)
        await Promise.resolve()
      })
    }
  }
}

describe('OnboardingInlineCommandTerminal (render)', () => {
  it('does not recreate the tab when devServerId resolves from null to a real value after mount', async () => {
    createTab.mockClear()
    closeTab.mockClear()
    const { root, rerender } = await renderWithDevServerId(null)
    expect(createTab).toHaveBeenCalledTimes(1)
    expect(createTab.mock.calls[0][3]).not.toHaveProperty('connectionId')

    await rerender('dev-01')

    expect(createTab).toHaveBeenCalledTimes(1)
    expect(closeTab).not.toHaveBeenCalled()
    await act(() => root.unmount())
  })

  it('passes the devServerId already available at mount as connectionId', async () => {
    createTab.mockClear()
    const { root } = await renderWithDevServerId('dev-01')
    expect(createTab).toHaveBeenCalledTimes(1)
    expect(createTab.mock.calls[0][3]).toMatchObject({ connectionId: 'dev-01' })
    await act(() => root.unmount())
  })
})
