// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ReviewNoteAnchor, ReviewSentBatch } from '../../../../../shared/code-intel-types'
import type { DiffComment } from '../../../../../shared/types'

const mocks = vi.hoisted(() => ({
  state: {} as Record<string, unknown>,
  updateNotes: vi.fn((_update: unknown) => true),
  persistence: { value: null as unknown }
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mocks.state)
}))
vi.mock('../../../hooks/useReviewNotesPersistence', () => ({
  useReviewNotesPersistence: () => mocks.persistence.value
}))
vi.mock('./ReviewNotesSendMenu', () => ({ ReviewNotesSendMenu: () => <span>send-menu</span> }))

import { ReviewNoteButton } from './ReviewNoteButton'
import { ReviewNodeNoteBadge } from './ReviewNodeNoteBadge'
import { ReviewNotesPanel } from './ReviewNotesPanel'
import { ReviewSentBatchList } from './ReviewSentBatchList'
import { ReviewSendPreviewDialog } from './ReviewSendPreviewDialog'

const erd: ReviewNoteAnchor = { kind: 'graph-node', lens: 'erd', nodeKey: 'orders', filePath: 'db/0042.sql', label: 'orders' }
const comment = (id: string, over: Partial<DiffComment> = {}): DiffComment =>
  ({ id, worktreeId: 'wt', filePath: 'db/0042.sql', lineNumber: 0, body: `[Review map · erd · orders] hello ${id}`, createdAt: 1, side: 'modified', ...over }) as DiffComment

function setUserAgent(ua: string): void {
  vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(ua)
}

beforeEach(() => {
  mocks.updateNotes.mockClear()
  mocks.persistence.value = {
    ready: true,
    notes: { anchors: {}, sentBatches: [] },
    saveStatus: 'saved',
    updateNotes: mocks.updateNotes
  }
  mocks.state = {
    addDiffComment: vi.fn(async (input: Record<string, unknown>) => ({ ...input, id: 'new-1', createdAt: 5 })),
    getDiffComments: () => [],
    updateDiffComment: vi.fn(async () => true),
    deleteDiffComment: vi.fn(async () => {}),
    setReviewLens: vi.fn(),
    selectReviewSymbol: vi.fn(),
    selectErdTable: vi.fn(),
    selectStorageNode: vi.fn(),
    setReviewDataFlowId: vi.fn()
  }
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('ReviewNoteButton / composer', () => {
  it('is disabled with a visible reason for a node without a file', () => {
    render(<ReviewNoteButton worktreeId="wt" anchor={{ ...erd, filePath: '' }} />)
    expect((screen.getByRole('button', { name: /Note/ }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('No file to attach to')).toBeTruthy()
  })

  it('shows the real file:line and saves a prefixed DiffComment plus the anchor', async () => {
    render(<ReviewNoteButton worktreeId="wt" anchor={{ ...erd, startLine: 4, endLine: 7 }} />)
    fireEvent.click(screen.getByRole('button', { name: /Note/ }))
    expect(screen.getByText('Attaches to db/0042.sql:7')).toBeTruthy()
    fireEvent.change(screen.getByLabelText('Note'), { target: { value: ' check tenant ' } })
    fireEvent.click(screen.getByText('Save note'))
    await waitFor(() => expect(mocks.updateNotes).toHaveBeenCalled())
    const add = mocks.state.addDiffComment as ReturnType<typeof vi.fn>
    expect(add).toHaveBeenCalledWith({
      worktreeId: 'wt',
      filePath: 'db/0042.sql',
      startLine: 4,
      lineNumber: 7,
      body: '[Review map · erd · orders] check tenant',
      side: 'modified'
    })
    const update = mocks.updateNotes.mock.calls[0][0] as (p: unknown) => { anchors: Record<string, unknown> }
    expect(update({ anchors: {}, sentBatches: [] }).anchors['new-1']).toMatchObject({ nodeKey: 'orders' })
  })

  it('a file-level note (no lines) shows only the path', () => {
    render(<ReviewNoteButton worktreeId="wt" anchor={erd} />)
    fireEvent.click(screen.getByRole('button', { name: /Note/ }))
    expect(screen.getByText('Attaches to db/0042.sql')).toBeTruthy()
  })

  it('keeps the draft and shows an error when saving fails', async () => {
    ;(mocks.state.addDiffComment as ReturnType<typeof vi.fn>).mockResolvedValue(null)
    render(<ReviewNoteButton worktreeId="wt" anchor={erd} />)
    fireEvent.click(screen.getByRole('button', { name: /Note/ }))
    fireEvent.change(screen.getByLabelText('Note'), { target: { value: 'draft text' } })
    fireEvent.click(screen.getByText('Save note'))
    await screen.findByRole('alert')
    expect((screen.getByLabelText('Note') as HTMLTextAreaElement).value).toBe('draft text')
    expect(mocks.updateNotes).not.toHaveBeenCalled()
  })

  it('Mod+Enter saves: Cmd on macOS, Ctrl elsewhere', async () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X)')
    render(<ReviewNoteButton worktreeId="wt" anchor={erd} />)
    fireEvent.click(screen.getByRole('button', { name: /Note/ }))
    const area = screen.getByLabelText('Note')
    fireEvent.change(area, { target: { value: 'x' } })
    fireEvent.keyDown(area, { key: 'Enter', ctrlKey: true })
    expect(mocks.state.addDiffComment).not.toHaveBeenCalled()
    fireEvent.keyDown(area, { key: 'Enter', metaKey: true })
    await waitFor(() => expect(mocks.state.addDiffComment).toHaveBeenCalledTimes(1))
  })

  it('does not double-submit while a save is in flight', async () => {
    let release!: (c: unknown) => void
    ;(mocks.state.addDiffComment as ReturnType<typeof vi.fn>).mockImplementation(() => new Promise((r) => (release = r)))
    setUserAgent('Mozilla/5.0 (X11; Linux x86_64)')
    render(<ReviewNoteButton worktreeId="wt" anchor={erd} />)
    fireEvent.click(screen.getByRole('button', { name: /Note/ }))
    const area = screen.getByLabelText('Note')
    fireEvent.change(area, { target: { value: 'x' } })
    fireEvent.keyDown(area, { key: 'Enter', ctrlKey: true })
    fireEvent.keyDown(area, { key: 'Enter', ctrlKey: true })
    expect(mocks.state.addDiffComment).toHaveBeenCalledTimes(1)
    release({ id: 'z' })
  })

  it('n opens the composer only when hotkey is on and focus is not in an input', () => {
    const { rerender } = render(<ReviewNoteButton worktreeId="wt" anchor={erd} />)
    fireEvent.keyDown(document.body, { key: 'n' })
    expect(screen.queryByLabelText('Note', { selector: 'textarea' })).toBeNull()
    rerender(<ReviewNoteButton worktreeId="wt" anchor={erd} hotkey />)
    const input = document.createElement('input')
    document.body.appendChild(input)
    fireEvent.keyDown(input, { key: 'n' })
    expect(screen.queryByLabelText('Note', { selector: 'textarea' })).toBeNull()
    fireEvent.keyDown(document.body, { key: 'n' })
    expect(screen.getByLabelText('Note', { selector: 'textarea' })).toBeTruthy()
    input.remove()
  })
})

describe('ReviewNodeNoteBadge', () => {
  it('hides at zero and announces the count', () => {
    const { container, rerender } = render(<ReviewNodeNoteBadge count={0} />)
    expect(container.textContent).toBe('')
    rerender(<ReviewNodeNoteBadge count={3} />)
    expect(screen.getByLabelText('3 unsent notes')).toBeTruthy()
  })
})

describe('ReviewNotesPanel', () => {
  beforeEach(() => {
    mocks.persistence.value = {
      ready: true,
      notes: { anchors: { a: erd, s: erd }, sentBatches: [] },
      saveStatus: 'saved',
      updateNotes: mocks.updateNotes
    }
    mocks.state.getDiffComments = () => [comment('a'), comment('s', { sentAt: 9 }), comment('plain')]
  })

  it('lists anchored notes by lens without the prefix and ignores plain line notes', () => {
    render(<ReviewNotesPanel worktreeId="wt" onOpenDiff={() => {}} />)
    expect(screen.getByText('hello a')).toBeTruthy()
    expect(screen.queryByText(/hello plain/)).toBeNull()
    expect(screen.getByText('Sent')).toBeTruthy()
    expect(screen.getByText('Review notes (2)')).toBeTruthy()
  })

  it('edit keeps the prefix when saving; delete removes; jump navigates', async () => {
    const onOpenDiff = vi.fn()
    render(<ReviewNotesPanel worktreeId="wt" onOpenDiff={onOpenDiff} />)
    const row = document.querySelector('[data-comment-id="a"]') as HTMLElement
    fireEvent.click(row.querySelector('button:nth-of-type(1)')!) // Go to
    expect(mocks.state.setReviewLens).toHaveBeenCalledWith('wt', 'erd')
    expect(onOpenDiff).toHaveBeenCalledWith('db/0042.sql', undefined)

    fireEvent.click(screen.getAllByText('Edit')[0])
    fireEvent.change(screen.getByLabelText('Note'), { target: { value: 'changed' } })
    fireEvent.click(screen.getByText('Save note'))
    await waitFor(() =>
      expect(mocks.state.updateDiffComment).toHaveBeenCalledWith('wt', 'a', '[Review map · erd · orders] changed')
    )

    fireEvent.click(screen.getAllByText('Delete')[0])
    expect(mocks.state.deleteDiffComment).toHaveBeenCalledWith('wt', 'a')
  })

  it('shows an inline message when the history could not be saved and an empty state', () => {
    mocks.persistence.value = { ready: true, notes: { anchors: {}, sentBatches: [] }, saveStatus: 'error', updateNotes: mocks.updateNotes }
    render(<ReviewNotesPanel worktreeId="wt" onOpenDiff={() => {}} />)
    expect(screen.getByText(/Could not save the note history/)).toBeTruthy()
    expect(screen.getByText(/No notes yet/)).toBeTruthy()
  })
})

describe('ReviewSentBatchList', () => {
  const batch: ReviewSentBatch = {
    batchId: 'b',
    sentAt: 1000,
    turnId: null,
    targetPaneKey: null,
    agentType: 'claude',
    notes: [
      { commentId: '1', anchor: erd, filePath: 'a.ts', lineNumber: 3, body: 'fix it', fileIdentityAtSend: 'H1' },
      { commentId: '2', anchor: erd, filePath: 'b.ts', lineNumber: 0, body: 'and this', fileIdentityAtSend: 'H2' },
      { commentId: '3', anchor: erd, filePath: 'c.ts', lineNumber: 0, body: 'no identity' }
    ]
  }

  it('shows estimate-labelled hints and never says "resolved"', () => {
    render(<ReviewSentBatchList batches={[batch]} currentIdentityByPath={{ 'a.ts': 'H1', 'b.ts': 'H9' }} />)
    expect(screen.getByText('No change seen in this file (estimate)')).toBeTruthy()
    expect(screen.getByText('File changed since sending (estimate)')).toBeTruthy()
    expect(screen.queryByText(/resolved|addressed|done/i)).toBeNull()
    expect(screen.getAllByText(/estimate/)).toHaveLength(2)
  })

  it('shows an empty state', () => {
    render(<ReviewSentBatchList batches={[]} currentIdentityByPath={{}} />)
    expect(screen.getByText('Nothing sent yet.')).toBeTruthy()
  })
})

describe('ReviewSendPreviewDialog', () => {
  it('shows the prompt read-only and warns on large batches', () => {
    const { rerender } = render(<ReviewSendPreviewDialog open onOpenChange={() => {}} prompt="THE PROMPT" noteCount={3} />)
    expect(screen.getByText('THE PROMPT')).toBeTruthy()
    expect(screen.queryByRole('alert')).toBeNull()
    rerender(<ReviewSendPreviewDialog open onOpenChange={() => {}} prompt="P" noteCount={51} />)
    expect(screen.getByRole('alert')).toBeTruthy()
  })
})
