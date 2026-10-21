// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const toast = vi.hoisted(() => Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }))
vi.mock('sonner', () => ({ toast }))
const rpc = vi.hoisted(() => vi.fn())
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => rpc(...a)
}))

import { resetReviewArtifactCache } from '../../../../hooks/useReviewArtifact'
import { ReviewArtifactFrame } from './ReviewArtifactFrame'
import type { DiagramRef } from '../../../../../../shared/outcome-review-types'

const diagram: DiagramRef = {
  section: 'flow',
  variant: 'delta',
  type: 'sequence',
  inputDigest: 'd',
  status: 'cached',
  artifactId: 'a2'
}
const meta = (id: string, rev: number, extra: Record<string, unknown> = {}) => ({
  artifactId: id,
  section: 'flow',
  variant: 'delta',
  blobDigest: `d-${id}`,
  viewBox: '0 0 1000 500',
  revision: rev,
  origin: rev === 1 ? 'auto' : 'regenerated',
  diagramType: 'sequence',
  skillVersion: 'diagram-design@abc',
  createdAt: '2026-10-20T10:00:00Z',
  state: 'active',
  ...extra
})

beforeEach(() => {
  rpc.mockReset()
  resetReviewArtifactCache()
  toast.error.mockReset()
  toast.success.mockReset()
  rpc.mockImplementation(async (method: string, p: Record<string, unknown>) => {
    switch (method) {
      case 'request.review.artifact':
        return {
          ok: true,
          value: {
            artifact: meta(String(p.artifactId), 2, { title: 'Bug status flow' }),
            mediaType: 'text/html',
            content: '<svg><text>hello</text></svg>'
          }
        }
      case 'request.review.revisions':
        return {
          ok: true,
          value: { revisions: [meta('a2', 2), meta('a1', 1), meta('a0', 0, { state: 'failed' })] }
        }
      case 'request.review.compare':
        return { ok: true, value: { diff: { addedNodes: ['ms-new'], removedEdges: ['a>b'] } } }
      case 'request.review.restore':
        return { ok: true, value: { artifact: meta('a1', 1) } }
      default:
        return { ok: false, error: { kind: 'not_found' } }
    }
  })
})
afterEach(cleanup)

const show = (over: Record<string, unknown> = {}) =>
  render(
    <ReviewArtifactFrame
      requestId="r1"
      diagram={diagram}
      label="Flow, after code"
      canRegenerate
      onRegenerate={vi.fn()}
      onChanged={vi.fn()}
      {...over}
    />
  )

describe('ReviewArtifactFrame', () => {
  it('says Orca drew it, not an agent that may be wrong, when the drawing came from the builtin engine', async () => {
    rpc.mockImplementationOnce(async (_m: string, p: Record<string, unknown>) => ({
      ok: true,
      value: {
        artifact: meta(String(p.artifactId), 1, { skillVersion: 'orca-builtin/1' }),
        mediaType: 'text/html',
        content: '<svg><text>hello</text></svg>'
      }
    }))
    show()
    await screen.findByTestId('review-artifact-iframe')
    expect(screen.getByTestId('review-artifact-info')).toHaveTextContent(
      'Drawn by Orca from the checked model above: every box and arrow is in the model.'
    )
    expect(screen.getByTestId('review-artifact-info')).not.toHaveTextContent('may be wrong')
  })

  it('sizes the frame from the viewBox, names it by the stored title, and says an agent drew it', async () => {
    show()
    const frame = await screen.findByTestId('review-artifact-iframe')
    expect(frame.style.aspectRatio).toMatch(/^2( \/ 1)?$/)
    expect(frame.style.maxHeight).toBe('70vh')
    expect(frame).toHaveAttribute('title', 'Bug status flow')
    expect(frame.getAttribute('sandbox')).toBe('')
    const info = screen.getByTestId('review-artifact-info')
    expect(info).toHaveTextContent('Drawn by an agent from the model above. It may be wrong.')
    expect(screen.getByTestId('review-artifact-meta')).toHaveTextContent(
      'sequence · skill diagram-design@abc · revision 2'
    )
    expect(screen.getByTestId('review-artifact-meta')).toHaveTextContent('regenerated')
  })

  it('falls back to the section label when the drawing has no title, and to 16:9 without a viewBox', async () => {
    rpc.mockImplementation(async () => ({
      ok: true,
      value: {
        artifact: meta('a2', 1, { viewBox: undefined, title: undefined }),
        mediaType: 'text/html',
        content: '<svg/>'
      }
    }))
    show()
    const frame = await screen.findByTestId('review-artifact-iframe')
    expect(frame).toHaveAttribute('title', 'Flow, after code')
    expect(frame.style.aspectRatio).toBe('16 / 9')
  })

  it('repaints from the tokens in force when the theme changes, without asking the server again', async () => {
    show()
    const frame = await screen.findByTestId('review-artifact-iframe')
    expect(frame.getAttribute('srcdoc')).toContain('color-scheme:light')
    const calls = rpc.mock.calls.length
    document.documentElement.classList.add('dark')
    await waitFor(() =>
      expect(screen.getByTestId('review-artifact-iframe').getAttribute('srcdoc')).toContain(
        'color-scheme:dark'
      )
    )
    expect(rpc.mock.calls.length).toBe(calls)
    document.documentElement.classList.remove('dark')
  })

  it('opens larger in a dialog that closes with Escape, zooming outside the frame', async () => {
    show()
    await screen.findByTestId('review-artifact-iframe')
    fireEvent.click(screen.getByRole('button', { name: 'Open larger' }))
    const dialog = await screen.findByTestId('review-artifact-large')
    expect(within(dialog).getByTestId('review-artifact-iframe').getAttribute('sandbox')).toBe('')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Zoom in' }))
    expect(dialog).toHaveTextContent('150%')
    fireEvent.keyDown(dialog, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByTestId('review-artifact-large')).toBeNull())
  })

  it('shows the revisions, compares one with the one in force without asking an agent, and restores an older one', async () => {
    const onChanged = vi.fn()
    show({ onChanged })
    await screen.findByTestId('review-artifact-iframe')
    fireEvent.click(screen.getByRole('button', { name: 'Revisions' }))
    const rows = await screen.findAllByTestId('review-artifact-revision')
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent('(in force)')
    expect(within(rows[2]).queryByRole('button')).toBeNull() // a failed attempt can be neither compared nor restored

    fireEvent.click(within(rows[1]).getByRole('button', { name: 'Compare' }))
    const dialog = await screen.findByTestId('review-artifact-compare')
    await waitFor(() =>
      expect(within(dialog).getByTestId('review-artifact-model-diff')).toHaveTextContent('ms-new')
    )
    expect(within(dialog).getByTestId('review-artifact-model-diff')).toHaveTextContent(
      'Arrows removed'
    )
    expect(rpc).toHaveBeenCalledWith('request.review.compare', {
      id: 'r1',
      fromArtifactId: 'a1',
      toArtifactId: 'a2'
    })
    expect(rpc.mock.calls.map((c) => c[0])).not.toContain('request.review.regenerate')
    fireEvent.click(within(dialog).getAllByRole('button', { name: 'Close' })[0])
    await waitFor(() => expect(screen.queryByTestId('review-artifact-compare')).toBeNull())

    // The popover closes when the dialog opens; opening it again lists the same revisions.
    fireEvent.click(screen.getByRole('button', { name: 'Revisions' }))
    const again = await screen.findAllByTestId('review-artifact-revision')
    fireEvent.click(within(again[1]).getByRole('button', { name: 'Use this one' }))
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
    expect(rpc).toHaveBeenCalledWith('request.review.restore', { id: 'r1', artifactId: 'a1' })
    expect(toast.success).toHaveBeenCalled()
  })

  it('hides regenerate and restore from those who may not act', async () => {
    show({ canRegenerate: false })
    await screen.findByTestId('review-artifact-iframe')
    expect(screen.queryByRole('button', { name: 'Regenerate…' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Revisions' }))
    const rows = await screen.findAllByTestId('review-artifact-revision')
    expect(within(rows[1]).queryByRole('button', { name: 'Use this one' })).toBeNull()
    expect(within(rows[1]).getByRole('button', { name: 'Compare' })).toBeInTheDocument()
  })

  it('regenerate asks for this section only', async () => {
    const onRegenerate = vi.fn()
    show({ onRegenerate })
    await screen.findByTestId('review-artifact-iframe')
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate…' }))
    expect(onRegenerate).toHaveBeenCalledWith({ sections: ['flow'] })
  })

  it('a drawing that is being drawn says so and has no frame', () => {
    show({ diagram: { ...diagram, status: 'drawing', artifactId: undefined } })
    expect(screen.getByTestId('review-artifact-drawing')).toHaveTextContent('Drawing…')
    expect(screen.queryByTestId('review-artifact-iframe')).toBeNull()
    expect(rpc).not.toHaveBeenCalled()
  })

  it('drawing turned off says so; a drawing the server no longer has asks to reload', async () => {
    const { unmount } = show({ diagram: { ...diagram, status: 'disabled', artifactId: undefined } })
    expect(screen.getByTestId('review-artifact-notice')).toHaveTextContent(
      'Drawing is turned off, showing the table'
    )
    unmount()
    rpc.mockResolvedValue({
      ok: false,
      error: { kind: 'not_found', backendCode: 'REQUEST_REVIEW_ARTIFACT_NOT_FOUND', message: 'x' }
    })
    const onChanged = vi.fn()
    show({ onChanged })
    expect(await screen.findByTestId('review-artifact-notice')).toHaveTextContent(
      'This drawing was replaced. Reload the review.'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onChanged).toHaveBeenCalled()
  })

  it('an unreadable drawing says so and can be read again', async () => {
    rpc.mockResolvedValueOnce({ ok: false, error: { kind: 'network', message: 'x' } })
    show()
    expect(await screen.findByTestId('review-artifact-notice')).toHaveTextContent(
      'The drawing could not be loaded'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.getByTestId('review-artifact-iframe')).toBeInTheDocument())
  })
})
