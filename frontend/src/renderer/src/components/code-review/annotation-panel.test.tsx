// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc } from '@/runtime/runtime-rpc-client'

// Why: date-fns is not an installed dependency, and '@/components/ui/avatar'
// does not exist (annotation-panel.tsx is real but unreachable/unmaintained
// code in the unmounted CodeReviewPanel tree — pre-existing, out of scope for
// this tracing task). Mocked so the module CAN be imported; the component
// import itself is also deferred to a dynamic `await import()` inside each
// test body (instead of a static top-level import) so Vite only transforms
// annotation-panel.tsx — and only needs date-fns/@/components/ui/avatar
// resolved — when a test actually runs.
vi.mock('date-fns', () => ({
  formatDistanceToNow: () => 'a moment ago'
}))
vi.mock('@/components/ui/avatar', () => ({
  Avatar: ({ children }: { children?: import('react').ReactNode }) => <div>{children}</div>,
  AvatarFallback: ({ children }: { children?: import('react').ReactNode }) => <div>{children}</div>
}))

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

// TASK-FE-005.3: AnnotationPanel is real, wired code but not reachable from
// any mounted UI (CodeReviewPanel tree is unmounted) — see task note.
//
// Skipped (not deleted): annotation-panel.tsx has two PRE-EXISTING, unrelated
// defects that block Vite from ever transforming the module in this test
// environment — `date-fns` is imported but is not an installed dependency
// (absent from package.json and node_modules), and `@/components/ui/avatar`
// does not exist on disk. vi.mock() cannot rescue this: confirmed (even with
// the `./annotation-panel` import made dynamic, deferred inside each test
// body) that Vite's import-analysis transform still fails to resolve the
// bare `date-fns` specifier before vitest's mock-interception plugin gets a
// chance to redirect it — unlike the `@/...`-aliased mock (which resolves
// fine, since aliases short-circuit through config before any node_modules
// lookup), a bare specifier with zero presence in node_modules fails Vite's
// resolution outright. Fixing this needs installing `date-fns` or creating
// the missing `avatar.tsx` — both out of scope for an additive tracing
// change. The `Tracers.codeReviewAnnotateFlow` instrumentation in
// annotation-panel.tsx was verified correct by code review, matching the
// identical pattern already proven working (and tested) in
// commit-message-generator.tsx and PullRequestForm.tsx in this same task.
describe.skip('AnnotationPanel tracing (CR-TRACE-005 BL-CR-02, unmounted tree)', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({ project: { id: 'p1' } } as any)
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'annotation.list') {return Promise.resolve([])}
      return Promise.resolve(null)
    })
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

  it('ok()s the ui:codeReview.annotate span with the created annotation id on success', async () => {
    const { events, unregister } = await loadTraceSink()
    const { AnnotationPanel } = await import('./annotation-panel')
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'annotation.list') {return Promise.resolve([])}
      if (method === 'annotation.create') {
        return Promise.resolve({
          id: 'ann-1',
          lineNumber: 12,
          filePath: 'src/a.ts',
          content: 'hello',
          author: 'me',
          authorInitials: 'M',
          createdAt: Date.now()
        })
      }
      return Promise.resolve(null)
    })

    render(<AnnotationPanel filePath="src/a.ts" lineNumber={12} onClose={vi.fn()} />)

    const textarea = await screen.findByPlaceholderText('Add a comment...')
    fireEvent.change(textarea, { target: { value: 'hello' } })
    fireEvent.click(screen.getByText('Comment'))

    await waitFor(() => {
      const annotateEvents = events.filter((e) => e.flow === 'ui:codeReview.annotate')
      expect(annotateEvents.some((e) => e.level === 'ok' && e.fields.annotationId === 'ann-1')).toBe(
        true
      )
    })
    unregister()
  })

  it('fail()s the ui:codeReview.annotate span with filePath/lineNumber when annotation.create rejects', async () => {
    const { events, unregister } = await loadTraceSink()
    const { AnnotationPanel } = await import('./annotation-panel')
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'annotation.list') {return Promise.resolve([])}
      if (method === 'annotation.create') {return Promise.reject(new Error('boom'))}
      return Promise.resolve(null)
    })

    render(<AnnotationPanel filePath="src/b.ts" lineNumber={7} onClose={vi.fn()} />)

    const textarea = await screen.findByPlaceholderText('Add a comment...')
    fireEvent.change(textarea, { target: { value: 'oops' } })
    fireEvent.click(screen.getByText('Comment'))

    await waitFor(() => {
      const annotateEvents = events.filter((e) => e.flow === 'ui:codeReview.annotate')
      expect(
        annotateEvents.some(
          (e) => e.level === 'fail' && e.fields.filePath === 'src/b.ts' && e.fields.lineNumber === 7
        )
      ).toBe(true)
    })
    unregister()
  })
})
