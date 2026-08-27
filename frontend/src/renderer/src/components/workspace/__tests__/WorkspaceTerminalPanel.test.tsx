// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

let mockTabsByWorktree: Record<string, { id: string }[]>

vi.mock('../../../store', () => ({
  useAppStore: (selector: (state: unknown) => unknown) =>
    selector({ tabsByWorktree: mockTabsByWorktree })
}))

const createNewTerminalTab = vi.fn()
const closeTerminalTab = vi.fn()
vi.mock('../../terminal/terminal-tab-actions', () => ({
  createNewTerminalTab: (worktreeId: string) => createNewTerminalTab(worktreeId),
  closeTerminalTab: (tabId: string) => closeTerminalTab(tabId)
}))

vi.mock('../../terminal-pane/TerminalPane', () => ({
  default: ({
    tabId,
    worktreeId,
    isActive,
    onPtyExit,
    onCloseTab
  }: {
    tabId: string
    worktreeId: string
    isActive: boolean
    onPtyExit: () => void
    onCloseTab: () => void
  }) => (
    <div data-testid="terminal-pane" data-tab-id={tabId} data-worktree-id={worktreeId} data-active={String(isActive)}>
      <button data-testid="pty-exit" onClick={onPtyExit} />
      <button data-testid="close-tab" onClick={onCloseTab} />
    </div>
  )
}))

import { WorkspaceTerminalPanel } from '../WorkspaceTerminalPanel'

afterEach(() => cleanup())

describe('WorkspaceTerminalPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockTabsByWorktree = {}
  })

  it('creates a terminal tab (reusing createNewTerminalTab) when the worktree has none yet', () => {
    render(<WorkspaceTerminalPanel worktreeId="wt-1" />)
    expect(screen.getByText(/Starting terminal/)).toBeInTheDocument()
    expect(createNewTerminalTab).toHaveBeenCalledWith('wt-1')
    expect(screen.queryByTestId('terminal-pane')).not.toBeInTheDocument()
  })

  it('renders the real TerminalPane for the worktree\'s first tab once one exists', () => {
    mockTabsByWorktree = { 'wt-1': [{ id: 'tab-1' }, { id: 'tab-2' }] }
    render(<WorkspaceTerminalPanel worktreeId="wt-1" />)

    const pane = screen.getByTestId('terminal-pane')
    expect(pane).toHaveAttribute('data-tab-id', 'tab-1')
    expect(pane).toHaveAttribute('data-worktree-id', 'wt-1')
    expect(pane).toHaveAttribute('data-active', 'true')
    expect(createNewTerminalTab).not.toHaveBeenCalled()
  })

  it('reuses closeTerminalTab for both onPtyExit and onCloseTab', () => {
    mockTabsByWorktree = { 'wt-1': [{ id: 'tab-1' }] }
    render(<WorkspaceTerminalPanel worktreeId="wt-1" />)

    screen.getByTestId('pty-exit').click()
    expect(closeTerminalTab).toHaveBeenCalledWith('tab-1')

    closeTerminalTab.mockClear()
    screen.getByTestId('close-tab').click()
    expect(closeTerminalTab).toHaveBeenCalledWith('tab-1')
  })
})
