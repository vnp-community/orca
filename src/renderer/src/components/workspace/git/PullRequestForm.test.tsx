// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { PullRequestForm } from './PullRequestForm'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) => selector({ settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockRpc = vi.mocked(callRuntimeRpc)
const emit = vi.fn()

// TASK-FE-005.3: PullRequestForm is real, wired code but has no reachable UI
// entry point (only rendered from the unmounted CodeReviewPanel/PrCreateDialog
// tree) — see task note.
describe('PullRequestForm tracing (CR-TRACE-005 BL-CR-05, unmounted tree)', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({ emit } as any)
  })

  async function loadTraceSink(): Promise<{
    events: { id: string; flow: string; level: string; fields: Record<string, unknown> }[]
    unregister: () => void
  }> {
    const { registerTraceSink } = await import('../../../../../shared/trace')
    const events: { id: string; flow: string; level: string; fields: Record<string, unknown> }[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, unregister }
  }

  it('ok()s the ui:codeReview.createPr span with prUrl/exitCode on success', async () => {
    const { events, unregister } = await loadTraceSink()
    mockRpc.mockResolvedValue({ url: 'https://example.com/pr/1', exitCode: 0 })

    render(<PullRequestForm projectId="p1" worktreePath="/repo" currentBranch="feature" />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My PR' } })
    fireEvent.click(screen.getByText('Create Pull Request'))

    await waitFor(() => {
      const prEvents = events.filter((e) => e.flow === 'ui:codeReview.createPr')
      const okEvent = prEvents.find((e) => e.level === 'ok')
      expect(okEvent?.fields).toMatchObject({ prUrl: 'https://example.com/pr/1', exitCode: 0 })
    })
    expect(mockRpc).toHaveBeenCalledWith(
      'mock-target',
      'git.pr.create',
      expect.objectContaining({ traceId: expect.any(String) })
    )
    unregister()
  })

  it('includes the draft flag in the start span fields when creating a draft PR', async () => {
    const { events, unregister } = await loadTraceSink()
    mockRpc.mockResolvedValue({ url: 'https://example.com/pr/2', exitCode: 0 })

    render(<PullRequestForm projectId="p1" worktreePath="/repo" currentBranch="feature" />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Draft PR' } })
    fireEvent.click(screen.getByLabelText('Create as draft'))
    fireEvent.click(screen.getByText('Create Draft PR'))

    await waitFor(() => {
      const startEvent = events.find(
        (e) => e.flow === 'ui:codeReview.createPr' && e.level === 'start'
      )
      expect(startEvent?.fields).toMatchObject({ projectId: 'p1', base: 'main', draft: true })
    })
    unregister()
  })

  it('fail()s the ui:codeReview.createPr span with projectId/base when git.pr.create rejects', async () => {
    const { events, unregister } = await loadTraceSink()
    mockRpc.mockRejectedValue(new Error('pr create failed'))

    render(<PullRequestForm projectId="p1" worktreePath="/repo" currentBranch="feature" />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My PR' } })
    fireEvent.click(screen.getByText('Create Pull Request'))

    await waitFor(() => {
      const prEvents = events.filter((e) => e.flow === 'ui:codeReview.createPr')
      const failEvent = prEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields).toMatchObject({ projectId: 'p1', base: 'main' })
    })
    unregister()
  })
})
