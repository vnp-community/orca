// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { DiffViewer } from '../DiffViewer'
import { useWorkspace } from '../../../../context/WorkspaceContext'
import { callRuntimeRpc } from '@/runtime/runtime-rpc-client'

vi.mock('../../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    vi.fn(selector => selector({ settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

// Mock Monaco DiffEditor because it doesn't render well in happy-dom out of the box
vi.mock('@monaco-editor/react', () => ({
  DiffEditor: ({ original, modified, language }: any) => (
    <div data-testid="mock-diff-editor">
      <span data-testid="monaco-lang">{language}</span>
      <div data-testid="monaco-original">{original}</div>
      <div data-testid="monaco-modified">{modified}</div>
    </div>
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('DiffViewer', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
    } as any)
  })

  it('shows Skeleton while isLoading=true', () => {
    // Return a promise that never resolves so it stays loading
    mockRpc.mockReturnValue(new Promise(() => {}))
    render(<DiffViewer filePath="test.ts" />)
    expect(screen.getByTestId('diff-viewer-loading')).toBeInTheDocument()
  })

  it('fetches original content via git.getDiff and modified via fs.readFile', async () => {
    mockRpc.mockImplementation((target, method, args) => {
      if (method === 'git.getDiff') return Promise.resolve('HEAD content')
      if (method === 'fs.readFile') return Promise.resolve({ content: 'worktree content' })
      return Promise.resolve(null)
    })
    
    render(<DiffViewer filePath="test.ts" />)
    
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'git.getDiff', { projectId: 'p1', path: 'test.ts', staged: false, side: 'original' })
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'fs.readFile', { projectId: 'p1', path: 'test.ts', encoding: 'utf-8' })
    })
  })

  it('Monaco DiffEditor rendered with original + modified', async () => {
    mockRpc.mockImplementation((target, method, args) => {
      if (method === 'git.getDiff') return Promise.resolve('HEAD content')
      if (method === 'fs.readFile') return Promise.resolve({ content: 'worktree content' })
      return Promise.resolve(null)
    })
    
    render(<DiffViewer filePath="test.ts" />)
    
    await waitFor(() => {
      expect(screen.getByTestId('mock-diff-editor')).toBeInTheDocument()
      expect(screen.getByTestId('monaco-original')).toHaveTextContent('HEAD content')
      expect(screen.getByTestId('monaco-modified')).toHaveTextContent('worktree content')
    })
  })

  it('.ts extension detected as typescript language', async () => {
    mockRpc.mockImplementation((target, method, args) => {
      if (method === 'git.getDiff') return Promise.resolve('HEAD content')
      if (method === 'fs.readFile') return Promise.resolve({ content: 'worktree content' })
      return Promise.resolve(null)
    })
    render(<DiffViewer filePath="src/main.ts" />)
    
    await waitFor(() => {
      expect(screen.getByTestId('monaco-lang')).toHaveTextContent('typescript')
    })
  })

  it('shows error message on failure', async () => {
    mockRpc.mockRejectedValue(new Error('File not found'))
    // Must use staged={true} because unstaged swallows errors inside Promise.all
    render(<DiffViewer filePath="invalid.ts" staged={true} />)
    
    await waitFor(() => {
      expect(screen.getByTestId('diff-viewer-error')).toHaveTextContent('Failed to load diff: File not found')
    })
  })
})
