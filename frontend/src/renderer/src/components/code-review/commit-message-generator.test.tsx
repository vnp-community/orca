// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { CommitMessageGenerator } from './commit-message-generator'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc } from '@/runtime/runtime-rpc-client'

vi.mock('../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) => selector({ settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockRpc = vi.mocked(callRuntimeRpc)

// TASK-FE-005.3: CommitMessageGenerator is real, wired code but lives in the
// unmounted CodeReviewPanel tree — see task note.
describe('CommitMessageGenerator tracing (CR-TRACE-005 BL-CR-04, unmounted tree)', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1', rootPath: '/repo' },
      worktreePath: '/repo'
    } as any)
  })

  async function loadTraceSink(): Promise<{
    events: { id: string; flow: string; level: string; fields: Record<string, unknown> }[]
    unregister: () => void
  }> {
    const { registerTraceSink } = await import('../../../../shared/trace')
    const events: { id: string; flow: string; level: string; fields: Record<string, unknown> }[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, unregister }
  }

  it("ok()s the span with entry:'code-review-panel' and messageChars on success", async () => {
    const { events, unregister } = await loadTraceSink()
    mockRpc.mockResolvedValue('feat: generated message')

    render(
      <CommitMessageGenerator
        value=""
        onChange={vi.fn()}
        onCommit={vi.fn().mockResolvedValue(undefined)}
        isCommitting={false}
      />
    )
    fireEvent.click(screen.getByTitle('Generate commit message with AI'))

    await waitFor(() => {
      const aiCommitEvents = events.filter((e) => e.flow === 'ui:codeReview.aiCommitMessage')
      const okEvent = aiCommitEvents.find((e) => e.level === 'ok')
      expect(okEvent).toBeDefined()
      expect(okEvent?.fields.messageChars).toBe('feat: generated message'.length)
    })
    const startEvent = events.find(
      (e) => e.flow === 'ui:codeReview.aiCommitMessage' && e.level === 'start'
    )
    expect(startEvent?.fields.entry).toBe('code-review-panel')
    unregister()
  })

  it('fail()s the span even for the GIT_NO_STAGED_CHANGES branch (different toast, still a fail)', async () => {
    const { events, unregister } = await loadTraceSink()
    const err: any = new Error('no staged changes')
    err.code = 'GIT_NO_STAGED_CHANGES'
    mockRpc.mockRejectedValue(err)

    render(
      <CommitMessageGenerator
        value=""
        onChange={vi.fn()}
        onCommit={vi.fn().mockResolvedValue(undefined)}
        isCommitting={false}
      />
    )
    fireEvent.click(screen.getByTitle('Generate commit message with AI'))

    await waitFor(() => {
      const aiCommitEvents = events.filter((e) => e.flow === 'ui:codeReview.aiCommitMessage')
      expect(aiCommitEvents.some((e) => e.level === 'fail')).toBe(true)
    })
    unregister()
  })
})
