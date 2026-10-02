/**
 * @vitest-environment happy-dom
 */
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SortableTabContextMenu } from './SortableTabContextMenu'

const origin = { type: 'mcp', clientName: 'Claude', mcpSessionId: 's', userId: 'u' }
const storeMock = vi.hoisted(() => ({
  state: {
    keybindings: {},
    ptyIdsByTabId: { 'term-1': ['h1'] },
    mcpOriginByHandle: {}
  } as Record<string, unknown>
}))

vi.mock('@/hooks/useShortcutLabel', () => ({
  formatShortcutLabel: () => '',
  useOptionalShortcutLabel: () => ''
}))
vi.mock('./TerminalTabSplitMenuSection', () => ({ TerminalTabSplitMenuSection: () => null }))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => vi.fn() }))
vi.mock('@/hooks/useStopMcpTerminal', () => ({ useStopMcpTerminal: () => vi.fn() }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/i18n/i18n', () => ({ translate: (_k: string, fallback: string) => fallback }))
vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: ({ children }: { children?: ReactNode }) => children,
  DropdownMenuContent: ({ children }: { children?: ReactNode }) => children,
  DropdownMenuItem: ({ children }: { children?: ReactNode }) => <button>{children}</button>,
  DropdownMenuSeparator: () => null,
  DropdownMenuShortcut: ({ children }: { children?: ReactNode }) => children,
  DropdownMenuTrigger: ({ children }: { children?: ReactNode }) => children
}))
vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (selector: (state: Record<string, unknown>) => unknown) => selector(storeMock.state),
    { getState: () => storeMock.state }
  )
}))

let mounted: { container: HTMLDivElement; root: Root } | null = null
function renderMenu(): HTMLDivElement {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => {
    root.render(
      <SortableTabContextMenu
        tab={{
          id: 'term-1',
          ptyId: null,
          worktreeId: 'wt',
          title: 'bash',
          customTitle: null,
          color: null,
          sortOrder: 0,
          createdAt: 0
        }}
        unifiedTabId="tab-1"
        groupId="g"
        isActive
        open
        point={{ x: 0, y: 0 }}
        tabCount={2}
        hasTabsToRight
        isPinned={false}
        onOpenChange={vi.fn()}
        onActivate={vi.fn()}
        onClose={vi.fn()}
        onCloseOthers={vi.fn()}
        onCloseToRight={vi.fn()}
        onRenameOpen={vi.fn()}
        onSetTabColor={vi.fn()}
        onTogglePin={vi.fn()}
      />
    )
  })
  mounted = { container, root }
  return container
}

afterEach(() => {
  act(() => mounted?.root.unmount())
  mounted?.container.remove()
  storeMock.state.mcpOriginByHandle = {}
})

describe('SortableTabContextMenu MCP origin item', () => {
  it('has no stop item when the tab has no MCP origin', () => {
    expect(renderMenu().textContent).not.toContain('Stop agent-created process')
  })

  it('adds the stop item for an MCP-created terminal', () => {
    storeMock.state.mcpOriginByHandle = { h1: origin }
    expect(renderMenu().textContent).toContain('Stop agent-created process')
  })
})
