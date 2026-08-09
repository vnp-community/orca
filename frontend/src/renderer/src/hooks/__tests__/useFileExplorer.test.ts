// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { renderHook, act, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const mockRefreshFileTree = vi.fn()
const mockOn = vi.fn()

// Default workspace context
let mockWorkspaceReturn = {
  project:         { id: 'p1', name: 'myapp' },
  fileTree:        { name: 'myapp', path: '.', type: 'directory', children: [] },
  refreshFileTree: mockRefreshFileTree,
  isOffline:       false,
  on:              mockOn,
}

vi.mock('../../context/WorkspaceContext', () => ({
  useWorkspace: () => mockWorkspaceReturn,
}))

import { useFileExplorer } from '../useFileExplorer'

afterEach(() => { cleanup(); vi.clearAllMocks() })

// Setup mockOn to capture event handlers
function setupEventMocks() {
  const handlers: Record<string, (payload?: any) => void> = {}
  mockOn.mockImplementation((event: string, handler: (payload?: any) => void) => {
    handlers[event] = handler
    return () => { delete handlers[event] }
  })
  return handlers
}

describe('useFileExplorer', () => {
  it('toggleDir: expand → add to expandedDirs + call refreshFileTree', async () => {
    mockOn.mockReturnValue(() => {})
    const { result } = renderHook(() => useFileExplorer())
    await act(async () => { await result.current.toggleDir('src') })
    expect(result.current.expandedDirs.has('src')).toBe(true)
    expect(mockRefreshFileTree).toHaveBeenCalledWith('src')
  })

  it('toggleDir: collapse → remove from expandedDirs', async () => {
    mockOn.mockReturnValue(() => {})
    const { result } = renderHook(() => useFileExplorer())
    await act(async () => { await result.current.toggleDir('src') })
    expect(result.current.expandedDirs.has('src')).toBe(true)
    await act(async () => { await result.current.toggleDir('src') })
    expect(result.current.expandedDirs.has('src')).toBe(false)
  })

  it('openFile → sets selectedPath + viewingFile', () => {
    mockOn.mockReturnValue(() => {})
    const { result } = renderHook(() => useFileExplorer())
    act(() => { result.current.openFile('src/index.ts') })
    expect(result.current.selectedPath).toBe('src/index.ts')
    expect(result.current.viewingFile).toBe('src/index.ts')
  })

  it('agent.complete event → refreshFileTree called', () => {
    const handlers = setupEventMocks()
    renderHook(() => useFileExplorer())
    // Simulate agent.complete event
    act(() => { handlers['agent.complete']?.() })
    expect(mockRefreshFileTree).toHaveBeenCalled()
  })

  it('files.changed event → refreshes parent dirs only', () => {
    const handlers = setupEventMocks()
    renderHook(() => useFileExplorer())
    act(() => { handlers['files.changed']?.({ paths: ['src/components/Button.tsx', 'src/utils.ts'] }) })
    expect(mockRefreshFileTree).toHaveBeenCalledWith('src/components')
    expect(mockRefreshFileTree).toHaveBeenCalledWith('src')
  })

  it('git.commit → refreshFileTree called', () => {
    const handlers = setupEventMocks()
    renderHook(() => useFileExplorer())
    act(() => { handlers['git.commit']?.() })
    expect(mockRefreshFileTree).toHaveBeenCalled()
  })
})
