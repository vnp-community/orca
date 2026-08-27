// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { DiffViewer } from '../DiffViewer'
import { useWorkspace } from '../../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../../runtime/runtime-rpc-client'

vi.mock('../../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('../../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) => selector({ settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

vi.mock('../../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

// Mock Monaco DiffEditor because it doesn't render well in happy-dom out of the box
vi.mock('@monaco-editor/react', () => ({
  DiffEditor: ({
    original,
    modified,
    language
  }: {
    original: string
    modified: string
    language: string
  }) => (
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
      currentWorktree: {
        id: 'repo1::/repo/worktree',
        path: '/repo/worktree',
        branch: 'main',
        isMain: true
      }
    } as unknown as ReturnType<typeof useWorkspace>)
  })

  it('shows Skeleton while isLoading=true', () => {
    // Return a promise that never resolves so it stays loading
    mockRpc.mockReturnValue(new Promise(() => {}))
    render(<DiffViewer filePath="test.ts" />)
    expect(screen.getByTestId('diff-viewer-loading')).toBeInTheDocument()
  })

  // Why (crash reported by user, same contract bug as GitPanel.tsx's push):
  // this used to fetch via two separate broken calls ('git.getDiff' +
  // 'fs.readFile', both nonexistent methods). git.diff (real RPC,
  // {worktree, filePath, staged}) already returns both sides in one call.
  it('fetches original + modified content via a single git.diff call', async () => {
    mockRpc.mockResolvedValue({
      kind: 'text',
      originalContent: 'HEAD content',
      modifiedContent: 'worktree content',
      originalIsBinary: false,
      modifiedIsBinary: false
    })

    render(<DiffViewer filePath="test.ts" />)

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'git.diff', {
        worktree: 'id:repo1::/repo/worktree',
        filePath: 'test.ts',
        staged: false
      })
    })
  })

  it('Monaco DiffEditor rendered with original + modified', async () => {
    mockRpc.mockResolvedValue({
      kind: 'text',
      originalContent: 'HEAD content',
      modifiedContent: 'worktree content',
      originalIsBinary: false,
      modifiedIsBinary: false
    })

    render(<DiffViewer filePath="test.ts" />)

    await waitFor(() => {
      expect(screen.getByTestId('mock-diff-editor')).toBeInTheDocument()
      expect(screen.getByTestId('monaco-original')).toHaveTextContent('HEAD content')
      expect(screen.getByTestId('monaco-modified')).toHaveTextContent('worktree content')
    })
  })

  it('.ts extension detected as typescript language', async () => {
    mockRpc.mockResolvedValue({
      kind: 'text',
      originalContent: '',
      modifiedContent: '',
      originalIsBinary: false,
      modifiedIsBinary: false
    })
    render(<DiffViewer filePath="src/main.ts" />)

    await waitFor(() => {
      expect(screen.getByTestId('monaco-lang')).toHaveTextContent('typescript')
    })
  })

  it('shows error message on failure', async () => {
    mockRpc.mockRejectedValue(new Error('File not found'))
    render(<DiffViewer filePath="invalid.ts" staged={true} />)

    await waitFor(() => {
      expect(screen.getByTestId('diff-viewer-error')).toHaveTextContent(
        'Failed to load diff: File not found'
      )
    })
  })

  it('requests staged:true when staged prop is set', async () => {
    mockRpc.mockResolvedValue({
      kind: 'text',
      originalContent: '',
      modifiedContent: '',
      originalIsBinary: false,
      modifiedIsBinary: false
    })
    render(<DiffViewer filePath="test.ts" staged={true} />)

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'git.diff', {
        worktree: 'id:repo1::/repo/worktree',
        filePath: 'test.ts',
        staged: true
      })
    })
  })

  // TASK-FE-005.1: ui:codeReview.diff tracing.
  describe('tracing (CR-TRACE-005 BL-CR-01)', () => {
    async function loadTraceSink(): Promise<{
      events: { flow: string; level: string; label?: string; fields: Record<string, unknown> }[]
      unregister: () => void
    }> {
      const { registerTraceSink } = await import('../../../../../../shared/trace')
      const events: {
        flow: string
        level: string
        label?: string
        fields: Record<string, unknown>
      }[] = []
      const unregister = registerTraceSink((e) => events.push(e))
      return { events, unregister }
    }

    it('ok()s the span with staged:true on a successful staged diff load', async () => {
      const { events, unregister } = await loadTraceSink()
      mockRpc.mockResolvedValue({
        kind: 'text',
        originalContent: '',
        modifiedContent: '@@ hunk',
        originalIsBinary: false,
        modifiedIsBinary: false
      })

      render(<DiffViewer filePath="test.ts" staged={true} />)

      await waitFor(() => {
        const diffEvents = events.filter((e) => e.flow === 'ui:codeReview.diff')
        expect(diffEvents.some((e) => e.level === 'ok' && e.fields.staged === true)).toBe(true)
      })
      unregister()
    })

    it('fail()s the span with staged:true when the diff RPC rejects', async () => {
      const { events, unregister } = await loadTraceSink()
      mockRpc.mockRejectedValue(new Error('File not found'))

      render(<DiffViewer filePath="invalid.ts" staged={true} />)

      await waitFor(() => {
        const diffEvents = events.filter((e) => e.flow === 'ui:codeReview.diff')
        expect(diffEvents.some((e) => e.level === 'fail' && e.fields.staged === true)).toBe(true)
      })
      unregister()
    })

    it('ok()s the span with staged:false once the diff RPC resolves', async () => {
      const { events, unregister } = await loadTraceSink()
      mockRpc.mockResolvedValue({
        kind: 'text',
        originalContent: 'HEAD content',
        modifiedContent: 'worktree content',
        originalIsBinary: false,
        modifiedIsBinary: false
      })

      render(<DiffViewer filePath="test.ts" staged={false} />)

      await waitFor(() => {
        const diffEvents = events.filter((e) => e.flow === 'ui:codeReview.diff')
        expect(diffEvents.some((e) => e.level === 'ok' && e.fields.staged === false)).toBe(true)
      })
      unregister()
    })
  })
})
