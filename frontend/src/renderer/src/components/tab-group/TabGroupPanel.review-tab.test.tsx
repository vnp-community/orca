// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'

// FE-CV-TASK-050-17: a Review tab renders ReviewTabHost (lazy) in the group body, never EditorPanel.
const h = vi.hoisted(() => ({
  model: null as unknown,
  tabBarProps: null as null | Record<string, unknown>,
  commands: {
    activateTerminal: vi.fn(),
    activateEditor: vi.fn(),
    closeItem: vi.fn(),
    focusGroup: vi.fn()
  } as Record<string, unknown>
}))

vi.mock('../../store', () => ({
  useAppStore: (sel: (s: Record<string, unknown>) => unknown) =>
    sel({ rightSidebarOpen: true, sidebarOpen: true })
}))
vi.mock('@dnd-kit/core', () => ({ useDroppable: () => ({ setNodeRef: () => undefined }) }))
vi.mock('./useTabGroupWorkspaceModel', () => ({ useTabGroupWorkspaceModel: () => h.model }))
vi.mock('../tab-bar/TabBar', () => ({
  default: (props: Record<string, unknown>) => {
    h.tabBarProps = props
    return <div data-testid="tab-bar" />
  }
}))
vi.mock('../tab-bar/TabBarQuickCommandsButton', () => ({ TabBarQuickCommandsButton: () => null }))
vi.mock('../review-map/shell/ReviewTabHost', () => ({
  ReviewTabHost: (p: { worktreeId: string; tabId: string }) => (
    <div data-testid="review-host" data-worktree={p.worktreeId} data-tab={p.tabId} />
  )
}))
vi.mock('../editor/EditorPanel', () => ({ default: () => <div data-testid="editor-panel" /> }))

import TabGroupPanel from './TabGroupPanel'

function tab(id: string, contentType: string) {
  return { id, entityId: `${id}-entity`, contentType, groupId: 'g1', worktreeId: 'wt-1', label: id }
}

function setModel(
  activeTab: ReturnType<typeof tab> | null,
  groupTabs = activeTab ? [activeTab] : []
) {
  h.model = {
    activeTab,
    browserItems: [],
    editorItems: [],
    terminalTabs: [],
    tabBarOrder: groupTabs.map((t) => t.id),
    groupTabs,
    expandedPaneByTabId: {},
    commands: new Proxy(h.commands, { get: (target, key: string) => target[key] ?? vi.fn() })
  }
}

const panel = (
  <TabGroupPanel
    groupId="g1"
    worktreeId="wt-1"
    isFocused={false}
    hasSplitGroups={false}
    touchesRightEdge
    touchesLeftEdge
    reserveClosedExplorerToggleSpace={false}
    reserveCollapsedSidebarHeaderSpace={false}
  />
)

beforeEach(() => {
  h.tabBarProps = null
})
afterEach(cleanup)

describe('TabGroupPanel review tab', () => {
  it('renders ReviewTabHost for the active review tab and passes review state to the TabBar', async () => {
    setModel(tab('review-1', 'review'))
    render(panel)
    const host = await screen.findByTestId('review-host')
    expect(host.dataset.worktree).toBe('wt-1')
    expect(host.dataset.tab).toBe('review-1')
    expect(screen.queryByTestId('editor-panel')).toBeNull()
    expect(h.tabBarProps?.activeTabType).toBe('review')
    expect(h.tabBarProps?.activeReviewTabId).toBe('review-1')
  })

  it('keeps EditorPanel for editor tabs and no review host', async () => {
    setModel(tab('file-1', 'editor'))
    render(panel)
    expect(await screen.findByTestId('editor-panel')).toBeTruthy()
    expect(screen.queryByTestId('review-host')).toBeNull()
    expect(h.tabBarProps?.activeReviewTabId).toBeNull()
  })

  it('renders no body content for terminal tabs (drawn by the worktree overlay)', () => {
    setModel(tab('term-1', 'terminal'))
    render(panel)
    expect(screen.queryByTestId('review-host')).toBeNull()
    expect(screen.queryByTestId('editor-panel')).toBeNull()
  })

  it('closes a review tab through its unified id', () => {
    setModel(tab('review-1', 'review'))
    render(panel)
    const onCloseFile = h.tabBarProps?.onCloseFile as (id: string) => void
    onCloseFile('review-1')
    expect(h.commands.closeItem).toHaveBeenCalledWith('review-1')
  })
})
