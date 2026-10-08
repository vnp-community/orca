// @vitest-environment happy-dom
import { cleanup, render } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DiffComment } from '../../../../shared/types'

const mocks = vi.hoisted(() => ({
  clearDelivered: vi.fn(),
  markSent: vi.fn(),
  captured: { props: null as null | { onDelivered: (n: readonly unknown[]) => void; scopes: { id: string; notes: unknown[]; prompt: string }[] } }
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (s: unknown) => unknown) =>
    selector({ clearDeliveredDiffComments: mocks.clearDelivered, worktreesByRepo: {} })
}))
vi.mock('@/lib/annotation-mark-sent-best-effort', () => ({ markAnnotationsSentBestEffort: mocks.markSent }))
vi.mock('@/lib/use-composed-all-notes-prompt', () => ({
  useComposedAllNotesPrompt: () => ({ prompt: 'ALL', annotationIds: ['ann-1'] })
}))
vi.mock('./NotesSendMenu', () => ({
  NotesSendMenu: (props: never) => {
    mocks.captured.props = props
    return null
  }
}))

import { DiffNotesSendMenu } from './DiffNotesSendMenu'

const note = (id: string, filePath: string, sentAt?: number): DiffComment =>
  ({ id, filePath, lineNumber: 1, body: id, createdAt: 1, side: 'modified', sentAt }) as DiffComment

beforeEach(() => {
  mocks.clearDelivered.mockReset()
  mocks.markSent.mockReset()
})
afterEach(cleanup)

describe('DiffNotesSendMenu (regression for the markAnnotationsSentBestEffort extraction)', () => {
  const comments = [note('a', 'x.ts'), note('b', 'y.ts'), note('c', 'z.ts', 5)]

  it('exposes only the unsent notes as the "all" scope', () => {
    render(<DiffNotesSendMenu worktreeId="wt" groupId="g" comments={comments} />)
    const scopes = mocks.captured.props!.scopes
    expect(scopes.map((s) => s.id)).toEqual(['all'])
    expect(scopes[0].notes).toHaveLength(2)
  })

  it('clears delivered notes and marks annotations sent when the whole "all" scope was delivered', () => {
    render(<DiffNotesSendMenu worktreeId="wt" groupId="g" comments={comments} />)
    const delivered = mocks.captured.props!.scopes[0].notes
    mocks.captured.props!.onDelivered(delivered)
    expect(mocks.clearDelivered).toHaveBeenCalledWith('wt', delivered)
    expect(mocks.markSent).toHaveBeenCalledWith(['ann-1'])
  })

  it('does not mark annotations sent for a file-scoped delivery', () => {
    render(<DiffNotesSendMenu worktreeId="wt" groupId="g" comments={comments} filePath="x.ts" showFileScope />)
    const fileScope = mocks.captured.props!.scopes.find((s) => s.id === 'file')!
    mocks.captured.props!.onDelivered(fileScope.notes)
    expect(mocks.clearDelivered).toHaveBeenCalled()
    expect(mocks.markSent).not.toHaveBeenCalled()
  })
})
