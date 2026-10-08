import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const appStoreSnapshot: {
  activeTabId: string | null
  activeTabType: 'terminal' | 'editor' | 'browser' | 'simulator' | 'review' | null
  unifiedTabsByWorktree: Record<string, unknown[]>
  activeGroupIdByWorktree: Record<string, string>
} = {
  activeTabId: 'old-terminal',
  activeTabType: 'terminal',
  unifiedTabsByWorktree: {},
  activeGroupIdByWorktree: {}
}
const pinTabMock: (tabId: string) => void = vi.fn()
const unpinTabMock: (tabId: string) => void = vi.fn()

const useAppStoreMock = vi.fn(
  (
    selector: (state: {
      activeTabId: string | null
      activeTabType: 'terminal' | 'editor' | 'browser' | 'simulator' | 'review' | null
      gitStatusByWorktree: Record<string, never[]>
      repos: never[]
      worktreesByRepo: Record<string, never[]>
      unifiedTabsByWorktree: Record<string, unknown[]>
      activeGroupIdByWorktree: Record<string, string>
      pinTab: typeof pinTabMock
      unpinTab: typeof unpinTabMock
      settings: {
        terminalWindowsShell: 'powershell.exe' | 'cmd.exe' | 'wsl.exe' | 'git-bash'
        terminalWindowsPowerShellImplementation: 'auto' | 'powershell.exe' | 'pwsh.exe'
      }
    }) => unknown
  ) =>
    selector({
      activeTabId: appStoreSnapshot.activeTabId,
      activeTabType: appStoreSnapshot.activeTabType,
      gitStatusByWorktree: {},
      repos: [],
      worktreesByRepo: {},
      unifiedTabsByWorktree: appStoreSnapshot.unifiedTabsByWorktree,
      activeGroupIdByWorktree: appStoreSnapshot.activeGroupIdByWorktree,
      pinTab: pinTabMock,
      unpinTab: unpinTabMock,
      settings: {
        terminalWindowsShell: 'powershell.exe',
        terminalWindowsPowerShellImplementation: 'auto'
      }
    })
)

vi.mock('react', async () => {
  const actual = await vi.importActual<typeof import('react')>('react') // eslint-disable-line @typescript-eslint/consistent-type-imports -- vi.importActual requires inline import()
  return {
    ...actual,
    memo: <T>(component: T) => component,
    useEffect: () => {},
    useLayoutEffect: () => {},
    useCallback: <T>(callback: T) => callback,
    useMemo: <T>(factory: () => T) => factory(),
    useRef: <T>(current: T) => ({ current }),
    useState: <T>(initial: T) => [initial, vi.fn()] as const
  }
})

// The headless React mock above stubs hooks, so zustand's useShallow (which
// calls useRef) has no dispatcher; make it a pass-through like the store mock.
vi.mock('zustand/react/shallow', () => ({
  useShallow: (selector: unknown) => selector
}))

vi.mock('lucide-react', () => ({
  FilePlus: function FilePlus() {
    return null
  },
  FileText: function FileText() {
    return null
  },
  Globe: function Globe() {
    return null
  },
  Plus: function Plus() {
    return null
  },
  Smartphone: function Smartphone() {
    return null
  },
  TerminalSquare: function TerminalSquare() {
    return null
  }
}))

vi.mock('@dnd-kit/sortable', () => ({
  SortableContext: function SortableContext(props: { children?: unknown }) {
    return props.children
  }
}))

vi.mock('./tab-strip-drag-scroll', () => ({
  useTabStripDragScrollHandlers: () => ({
    isTabDragActive: false,
    onDragScrollStartEnter: vi.fn(),
    onDragScrollEndEnter: vi.fn(),
    onDragScrollLeave: vi.fn()
  })
}))

const useAppStoreExport = (selector: Parameters<typeof useAppStoreMock>[0]): unknown =>
  useAppStoreMock(selector)
useAppStoreExport.getState = vi.fn(() => ({
  activeTabId: appStoreSnapshot.activeTabId,
  activeTabType: appStoreSnapshot.activeTabType,
  gitStatusByWorktree: {},
  repos: [],
  worktreesByRepo: {},
  unifiedTabsByWorktree: appStoreSnapshot.unifiedTabsByWorktree,
  activeGroupIdByWorktree: appStoreSnapshot.activeGroupIdByWorktree,
  pinTab: pinTabMock,
  unpinTab: unpinTabMock,
  settings: {
    terminalWindowsShell: 'powershell.exe',
    terminalWindowsPowerShellImplementation: 'auto'
  }
}))

vi.mock('../../store', () => ({
  useAppStore: useAppStoreExport
}))

vi.mock('../right-sidebar/status-display', () => ({
  buildStatusMap: () => new Map()
}))

vi.mock('../tab-group/tab-insertion', () => ({
  resolveTabIndicatorEdges: () => []
}))

vi.mock('@/components/editor/editor-labels', () => ({
  getEditorDisplayLabel: () => ''
}))

vi.mock('./SortableTab', () => ({
  default: function SortableTab(props: Record<string, unknown>) {
    return { type: 'SortableTab', props }
  }
}))

vi.mock('./EditorFileTab', () => ({
  default: function EditorFileTab(props: Record<string, unknown>) {
    return { type: 'EditorFileTab', props }
  }
}))

vi.mock('./BrowserTab', () => ({
  default: function BrowserTab(props: Record<string, unknown>) {
    return { type: 'BrowserTab', props }
  },
  getBrowserTabLabel: () => ''
}))

vi.mock('./QuickLaunchButton', () => ({
  QuickLaunchAgentMenuItems: function QuickLaunchAgentMenuItems() {
    return null
  }
}))

vi.mock('./shell-icons', () => ({
  ShellIcon: function ShellIcon() {
    return null
  }
}))

vi.mock('@/lib/focus-terminal-tab-surface', () => ({
  focusTerminalTabSurface: vi.fn()
}))

vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: function DropdownMenu(props: { children?: unknown }) {
    return { type: 'DropdownMenu', props }
  },
  DropdownMenuContent: function DropdownMenuContent(props: { children?: unknown }) {
    return { type: 'DropdownMenuContent', props }
  },
  DropdownMenuItem: function DropdownMenuItem(props: {
    children?: unknown
    onSelect?: () => void
  }) {
    return { type: 'DropdownMenuItem', props }
  },
  DropdownMenuSeparator: function DropdownMenuSeparator() {
    return { type: 'DropdownMenuSeparator', props: {} }
  },
  DropdownMenuShortcut: function DropdownMenuShortcut(props: { children?: unknown }) {
    return { type: 'DropdownMenuShortcut', props }
  },
  DropdownMenuLabel: function DropdownMenuLabel(props: { children?: unknown }) {
    return { type: 'DropdownMenuLabel', props }
  },
  DropdownMenuSub: function DropdownMenuSub(props: { children?: unknown }) {
    return { type: 'DropdownMenuSub', props }
  },
  DropdownMenuSubContent: function DropdownMenuSubContent(props: { children?: unknown }) {
    return { type: 'DropdownMenuSubContent', props }
  },
  DropdownMenuSubTrigger: function DropdownMenuSubTrigger(props: { children?: unknown }) {
    return { type: 'DropdownMenuSubTrigger', props }
  },
  DropdownMenuTrigger: function DropdownMenuTrigger(props: { children?: unknown }) {
    return { type: 'DropdownMenuTrigger', props }
  }
}))

type ReactElementLike = {
  type: unknown
  props: Record<string, unknown>
  ref?: unknown
}

function findChildrenByType(node: unknown, typeName: string): ReactElementLike[] {
  const results: ReactElementLike[] = []
  const visit = (current: unknown): void => {
    if (current == null) {
      return
    }
    if (Array.isArray(current)) {
      for (const child of current) {
        visit(child)
      }
      return
    }
    if (typeof current === 'string' || typeof current === 'number') {
      return
    }
    const el = current as ReactElementLike
    const type = el.type as { name?: string } | string | undefined
    const matchedName = typeof type === 'string' ? type : type?.name
    if (matchedName === typeName) {
      results.push(el)
    }
    if (el.props && 'children' in el.props) {
      visit(el.props.children)
    }
  }
  visit(node)
  return results
}

async function renderTabBar(props: Record<string, unknown>): Promise<unknown> {
  const tabBarModule = await import('./TabBar')
  const candidate = tabBarModule.default as unknown as
    | ((props: Record<string, unknown>) => unknown)
    | { type: (props: Record<string, unknown>) => unknown }
  const TabBar = typeof candidate === 'function' ? candidate : candidate.type
  return TabBar({
    activeTabId: null,
    worktreeId: 'wt-1',
    expandedPaneByTabId: {},
    onActivate: () => {},
    onClose: () => {},
    onCloseOthers: () => {},
    onCloseToRight: () => {},
    onNewTerminalTab: () => {},
    onNewBrowserTab: () => {},
    onSetCustomTitle: () => {},
    onSetTabColor: () => {},
    onTogglePaneExpand: () => {},
    ...props
  })
}

const reviewTab = (id: string, groupId = 'wt-1', label = 'Review') => ({
  id,
  entityId: id,
  groupId,
  worktreeId: 'wt-1',
  contentType: 'review',
  label,
  customLabel: null,
  color: null,
  sortOrder: 1,
  createdAt: 1
})

const TERMINAL_TAB = {
  id: 'term-1',
  unifiedTabId: 'unified-term-1',
  ptyId: null,
  worktreeId: 'wt-1',
  title: 'Terminal',
  customTitle: null,
  color: null,
  sortOrder: 0,
  createdAt: 0
}

// FE-CV-TASK-050-17: the Review tab renders through TabBar like the simulator tab (EditorFileTab chrome).
describe('TabBar review tab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.resetModules()
    appStoreSnapshot.activeTabId = 'term-1'
    appStoreSnapshot.activeTabType = 'terminal'
    appStoreSnapshot.unifiedTabsByWorktree = {}
    appStoreSnapshot.activeGroupIdByWorktree = {}
    vi.stubGlobal('navigator', { userAgent: 'Mac' })
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0)
      return 1
    })
    vi.stubGlobal('cancelAnimationFrame', vi.fn())
    vi.stubGlobal('window', {
      setTimeout,
      clearTimeout,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn()
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders a review unified tab of this group as an EditorFileTab with the review icon language', async () => {
    appStoreSnapshot.unifiedTabsByWorktree = {
      'wt-1': [
        reviewTab('review-1', 'wt-1', 'Review · feat/x'),
        reviewTab('review-other', 'group-2')
      ]
    }
    const element = await renderTabBar({
      tabs: [TERMINAL_TAB],
      editorFiles: [],
      browserTabs: [],
      tabBarOrder: ['term-1', 'review-1']
    })
    const tabs = findChildrenByType(element, 'EditorFileTab')
    // The review tab of another group stays out of this strip.
    expect(tabs).toHaveLength(1)
    const file = tabs[0].props.file as { id: string; language: string; relativePath: string }
    expect(file).toMatchObject({
      id: 'review-1',
      language: 'review',
      relativePath: 'Review · feat/x'
    })
    expect(tabs[0].props.isActive).toBe(false)
    expect(findChildrenByType(element, 'SortableTab')[0].props.tabCount).toBe(2)
  })

  it('marks the review tab active only while a review tab is the active type', async () => {
    appStoreSnapshot.unifiedTabsByWorktree = { 'wt-1': [reviewTab('review-1')] }
    appStoreSnapshot.activeTabType = 'review'
    const element = await renderTabBar({
      tabs: [TERMINAL_TAB],
      editorFiles: [],
      browserTabs: [],
      tabBarOrder: ['review-1', 'term-1'],
      activeTabType: 'review',
      activeReviewTabId: 'review-1'
    })
    const [tab] = findChildrenByType(element, 'EditorFileTab')
    expect(tab.props.isActive).toBe(true)
    expect(tab.props.hasTabsToRight).toBe(true)
  })

  it('routes activate, close and close-to-right through the unified review tab id', async () => {
    appStoreSnapshot.unifiedTabsByWorktree = { 'wt-1': [reviewTab('review-1')] }
    const onActivateFile = vi.fn()
    const onCloseFile = vi.fn()
    const onCloseToRight = vi.fn()
    const element = await renderTabBar({
      tabs: [],
      editorFiles: [],
      browserTabs: [],
      tabBarOrder: ['review-1'],
      onActivateFile,
      onCloseFile,
      onCloseToRight
    })
    const [tab] = findChildrenByType(element, 'EditorFileTab')
    ;(tab.props.onActivate as () => void)()
    ;(tab.props.onClose as () => void)()
    ;(tab.props.onCloseToRight as () => void)()
    expect(onActivateFile).toHaveBeenCalledWith('review-1')
    expect(onCloseFile).toHaveBeenCalledWith('review-1')
    expect(onCloseToRight).toHaveBeenCalledWith('review-1')
  })

  it('appends a review tab missing from the stored order instead of dropping it', async () => {
    appStoreSnapshot.unifiedTabsByWorktree = { 'wt-1': [reviewTab('review-1')] }
    const element = await renderTabBar({
      tabs: [TERMINAL_TAB],
      editorFiles: [],
      browserTabs: [],
      tabBarOrder: ['term-1']
    })
    expect(
      findChildrenByType(element, 'EditorFileTab').map((t) => (t.props.file as { id: string }).id)
    ).toEqual(['review-1'])
  })
})
