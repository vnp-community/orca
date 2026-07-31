// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// All vi.mock factories must be self-contained (no outer variable refs — factories are hoisted)
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn(),
}))

vi.mock('../../../store', () => ({
  useAppStore: vi.fn().mockImplementation((fn: any) => fn ? fn({ settings: {} }) : undefined),
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local', id: 'local' }),
}))

// Import AFTER mocks are declared
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'
import { FileSearchPanel } from '../FileSearchPanel'

const mockRpc = vi.mocked(callRuntimeRpc)
const mockUseWorkspace = vi.mocked(useWorkspace)
const mockAppStore = vi.mocked(useAppStore) as any

afterEach(() => cleanup())
beforeEach(() => {
  mockRpc.mockReset()
  mockAppStore.getState = vi.fn().mockReturnValue({ settings: {} })
  mockUseWorkspace.mockReturnValue({
    project: { id: 'p1', name: 'myapp', repoPath: '/repo/myapp' },
    currentWorktree: { path: '/repo/myapp' },
  } as any)
})

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
      await new Promise(r => setTimeout(r, 350))
    })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('≥2 chars → debounced fs.grep RPC call', async () => {
    mockRpc.mockResolvedValueOnce(['src/index.ts', 'src/App.tsx'])
    render(<FileSearchPanel onSelect={vi.fn()} />)
    await act(async () => {
      fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'in' } })
    })
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith(
        expect.objectContaining({ type: 'local' }),
        'fs.grep',
        expect.objectContaining({ pattern: 'in', projectId: 'p1' })
      )
    }, { timeout: 1000 })
  })

  it('results shown and click calls onSelect', async () => {
    mockRpc.mockResolvedValueOnce(['src/index.ts'])
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
    mockRpc.mockResolvedValueOnce([])
    render(<FileSearchPanel onSelect={vi.fn()} />)
    await act(async () => {
      fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'zz' } })
    })
    await waitFor(() => screen.getByText(/No files found/), { timeout: 1000 })
  })
})
