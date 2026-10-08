// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DiffComment } from '../../../../../shared/types'

const mocks = vi.hoisted(() => ({
  order: [] as string[],
  state: {} as Record<string, unknown>,
  updateNotes: vi.fn(),
  markSent: vi.fn(),
  decision: vi.fn(),
  captured: { props: null as null | { onDelivered: (n: readonly unknown[]) => void; scopes: { id: string; notes: DiffComment[]; prompt: string }[]; source?: string } }
}))
vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    (selector: (s: Record<string, unknown>) => unknown) => selector(mocks.state),
    { getState: () => mocks.state }
  )
}))
vi.mock('@/lib/review-surface-decision', () => ({ recordReviewSurfaceDecision: mocks.decision }))
vi.mock('@/lib/annotation-mark-sent-best-effort', () => ({ markAnnotationsSentBestEffort: mocks.markSent }))
vi.mock('@/lib/use-composed-all-notes-prompt', () => ({
  useComposedAllNotesPrompt: () => ({ prompt: 'ALL PROMPT', annotationIds: ['ann-1'] })
}))
vi.mock('../../../hooks/useReviewNotesPersistence', () => ({
  useReviewNotesPersistence: () => ({
    ready: true,
    notes: {
      anchors: {
        a: { kind: 'graph-node', lens: 'erd', nodeKey: 't', filePath: 'a.ts', label: 'orders' },
        b: { kind: 'graph-node', lens: 'impact', nodeKey: 'k', filePath: 'b.ts', label: 'f' }
      },
      sentBatches: []
    },
    saveStatus: 'saved',
    updateNotes: mocks.updateNotes
  })
}))
vi.mock('../../editor/NotesSendMenu', () => ({
  NotesSendMenu: (props: never) => {
    mocks.captured.props = props
    return <span>notes-send-menu</span>
  }
}))

import { ReviewNotesSendMenu } from './ReviewNotesSendMenu'

const note = (id: string, filePath: string): DiffComment =>
  ({ id, worktreeId: 'wt', filePath, lineNumber: 1, body: `n ${id}`, createdAt: 1, side: 'modified' }) as DiffComment

beforeEach(() => {
  mocks.decision.mockClear()
  mocks.order.length = 0
  mocks.updateNotes.mockReset().mockImplementation(() => {
    mocks.order.push('record')
    return true
  })
  mocks.markSent.mockReset().mockImplementation(() => mocks.order.push('mark'))
  mocks.state = {
    getDiffComments: () => [note('a', 'a.ts'), note('b', 'b.ts'), note('plain', 'p.ts')],
    clearDeliveredDiffComments: vi.fn(async () => {
      mocks.order.push('clear')
    }),
    reviewUiByWorktree: { wt: { lens: 'erd' } },
    worktreesByRepo: {},
    gitBranchCompareSummaryByWorktree: { wt: { headOid: 'H', mergeBase: 'M' } },
    gitStatusByWorktree: { wt: [{ path: 'a.ts', status: 'modified', area: 'unstaged', added: 1, removed: 0 }] }
  }
})
afterEach(cleanup)

describe('ReviewNotesSendMenu', () => {
  it('offers all / lens / selection scopes and keeps source diff-notes', () => {
    render(<ReviewNotesSendMenu worktreeId="wt" selectedCommentIds={['b']} />)
    const scopes = mocks.captured.props!.scopes
    expect(scopes.map((s) => s.id)).toEqual(['all', 'lens', 'selection'])
    expect(scopes[0].notes).toHaveLength(3)
    expect(scopes[0].prompt).toBe('ALL PROMPT')
    expect(scopes[1].notes.map((n) => n.id)).toEqual(['a'])
    expect(scopes[2].notes.map((n) => n.id)).toEqual(['b'])
    expect(mocks.captured.props!.source).toBe('diff-notes')
  })

  it('records the batch before clearing, and marks annotations sent only for the all scope', async () => {
    render(<ReviewNotesSendMenu worktreeId="wt" />)
    const scopes = mocks.captured.props!.scopes
    mocks.captured.props!.onDelivered(scopes[0].notes)
    await waitFor(() => expect(mocks.order).toEqual(['record', 'clear', 'mark']))
    expect(mocks.markSent).toHaveBeenCalledWith(['ann-1'])
    const update = mocks.updateNotes.mock.calls[0][0] as (p: unknown) => { sentBatches: { notes: { fileIdentityAtSend?: string }[] }[] }
    const next = update({ anchors: {}, sentBatches: [] })
    expect(next.sentBatches).toHaveLength(1)
    expect(next.sentBatches[0].notes[0].fileIdentityAtSend).toBeTruthy()
  })

  it('a lens-scoped delivery does not call annotation.markSent', async () => {
    render(<ReviewNotesSendMenu worktreeId="wt" />)
    mocks.captured.props!.onDelivered(mocks.captured.props!.scopes[1].notes)
    await waitFor(() => expect(mocks.order).toEqual(['record', 'clear']))
    expect(mocks.markSent).not.toHaveBeenCalled()
  })

  it('warns inline when the history row could not be written, without undoing the send', async () => {
    mocks.updateNotes.mockImplementation(() => false)
    render(<ReviewNotesSendMenu worktreeId="wt" />)
    mocks.captured.props!.onDelivered(mocks.captured.props!.scopes[0].notes)
    await screen.findByText('Sent, but the batch history was not saved.')
    expect(mocks.state.clearDeliveredDiffComments).toHaveBeenCalled()
  })

  it('opens the read-only preview of the all-notes prompt', () => {
    render(<ReviewNotesSendMenu worktreeId="wt" />)
    fireEvent.click(screen.getByText('Preview'))
    expect(screen.getByText('ALL PROMPT')).toBeTruthy()
  })

  it('records one send_to_agent decision per delivery', () => {
    render(<ReviewNotesSendMenu worktreeId="wt" />)
    mocks.captured.props!.onDelivered(mocks.captured.props!.scopes[0].notes)
    expect(mocks.decision).toHaveBeenCalledTimes(1)
    expect(mocks.decision).toHaveBeenCalledWith('wt', 'send_to_agent')
  })
})
