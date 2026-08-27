// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// All vi.mock factories must be self-contained (no outer variable refs — factories are hoisted)
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('../../../store', () => ({
  useAppStore: vi
    .fn()
    .mockImplementation((fn?: (s: unknown) => unknown) => (fn ? fn({ settings: {} }) : undefined))
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local', id: 'local' })
}))

// Import AFTER mocks are declared
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'
import { FileSearchPanel } from '../FileSearchPanel'

const mockRpc = vi.mocked(callRuntimeRpc)
const mockUseWorkspace = vi.mocked(useWorkspace)
const mockAppStore = vi.mocked(useAppStore) as unknown as typeof useAppStore & {
  getState: () => unknown
}

afterEach(() => cleanup())
beforeEach(() => {
  mockRpc.mockReset()
  mockAppStore.getState = vi.fn().mockReturnValue({ settings: {} })
  mockUseWorkspace.mockReturnValue({
    project: { id: 'p1', name: 'myapp', repoPath: '/repo/myapp' },
    currentWorktree: { id: 'repo1::/repo/myapp', path: '/repo/myapp', branch: 'main', isMain: true }
  } as unknown as ReturnType<typeof useWorkspace>)
})

// Why (crash reported by user, same class as GitPanel.tsx's push): this used
// to call the nonexistent 'fs.grep' with a {projectId, cwd, pattern} shape —
// the real method is 'files.search' and requires a {worktree, query} selector
// (backend/src/main/runtime/rpc/methods/files.ts).
describe('FileSearchPanel', () => {
  it('renders search input', () => {
    render(<FileSearchPanel onSelect={vi.fn()} />)
    expect(screen.getByTestId('file-search-panel')).toBeInTheDocument()
    expect(screen.getByTestId('search-input')).toBeInTheDocument()
  })

  it('<2 chars → no RPC call after debounce', async () => {
    render(<FileSearchPanel onSelect={vi.fn()} />)
    await act(async () => {
      fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'a' } })
      // wait longer than debounce (200ms) to confirm no call
      await new Promise((r) => setTimeout(r, 350))
    })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('≥2 chars → debounced files.search RPC call', async () => {
    mockRpc.mockResolvedValueOnce({
      files: [
        { filePath: '/repo/myapp/src/index.ts', relativePath: 'src/index.ts', matches: [] },
        { filePath: '/repo/myapp/src/App.tsx', relativePath: 'src/App.tsx', matches: [] }
      ],
      totalMatches: 2,
      truncated: false
    })
    render(<FileSearchPanel onSelect={vi.fn()} />)
    await act(async () => {
      fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'in' } })
    })
    await waitFor(
      () => {
        expect(mockRpc).toHaveBeenCalledWith(
          expect.objectContaining({ type: 'local' }),
          'files.search',
          expect.objectContaining({ query: 'in', worktree: 'id:repo1::/repo/myapp' })
        )
      },
      { timeout: 1000 }
    )
  })

  it('results shown and click calls onSelect', async () => {
    mockRpc.mockResolvedValueOnce({
      files: [{ filePath: '/repo/myapp/src/index.ts', relativePath: 'src/index.ts', matches: [] }],
      totalMatches: 1,
      truncated: false
    })
    const onSelect = vi.fn()
    render(<FileSearchPanel onSelect={onSelect} />)
    await act(async () => {
      fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'in' } })
    })
    await waitFor(() => screen.getByTestId('search-result-src/index.ts'), { timeout: 1000 })
    fireEvent.click(screen.getByTestId('search-result-src/index.ts'))
    expect(onSelect).toHaveBeenCalledWith('src/index.ts')
  })

  it('"No files found" for empty results', async () => {
    mockRpc.mockResolvedValueOnce({ files: [], totalMatches: 0, truncated: false })
    render(<FileSearchPanel onSelect={vi.fn()} />)
    await act(async () => {
      fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'zz' } })
    })
    await waitFor(() => screen.getByText(/No files found/), { timeout: 1000 })
  })
})
