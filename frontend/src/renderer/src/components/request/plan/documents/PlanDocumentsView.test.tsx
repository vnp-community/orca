// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { PlanDocumentsView } from './PlanDocumentsView'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'

afterEach(cleanup)

const request = { id: 'r1', version: 5, planTaskId: 'plan1' }
const rev = (n: number, over: Partial<PlanRevision> = {}): PlanRevision => ({
  revision: n,
  runId: `run-${n}`,
  status: n === 2 ? 'current' : 'superseded',
  engine: 'openspec',
  revisionFeedback: '',
  actorId: '',
  generatedAt: '2026-10-20T10:00:00Z',
  commit: `c${n}`,
  filesChanged: 0,
  fileCount: 0,
  textKept: true,
  ...over
})

const TASKS_MD =
  '<!-- orca:begin plan {"v":1} -->\n\n- [ ] 1.1 Write the test\n- [x] 1.2 Write the code\n\n<!-- orca:end plan -->'
const FILES: Record<string, string> = {
  'proposal.md': '# Why\nBecause.',
  'design.md': '## Design\n\n| a | b |\n|---|---|\n| 1 | 2 |',
  'specs/alpha/spec.md': '## ADDED Requirements',
  'specs/beta/spec.md': '## MODIFIED Requirements',
  'tasks.md': TASKS_MD
}

const bundle = (over: Record<string, unknown> = {}) => ({
  changeId: 'chg-1',
  branch: 'request/7-proposal',
  commit: 'c2',
  tasksMdDigest: 'dg2',
  files: [
    { path: 'tasks.md', size: 200, change: 'changed' },
    { path: 'specs/beta/spec.md', size: 50, change: 'added' },
    { path: 'proposal.md', size: 80, change: 'unchanged' },
    { path: 'design.md', size: 9_000_000, omitted: true, change: 'changed' },
    { path: 'specs/alpha/spec.md', size: 60, change: 'unchanged' }
  ],
  ...over
})

function serve(opts: { view?: Record<string, unknown>; fileError?: unknown } = {}) {
  callRequestRpc.mockImplementation(async (method: string, params: { path?: string }) => {
    if (method === 'request.planProposal') {
      return {
        ok: true,
        value: {
          runId: 'run-2',
          status: 'succeeded',
          engine: 'openspec',
          bundle: bundle(),
          ...opts.view
        }
      }
    }
    if (method === 'request.planBundleFile') {
      if (opts.fileError) {
        return opts.fileError
      }
      return {
        ok: true,
        value: { path: params.path, content: FILES[params.path ?? ''] ?? '', omitted: false }
      }
    }
    return { ok: true, value: {} }
  })
}
const fileCalls = () => callRequestRpc.mock.calls.filter(([m]) => m === 'request.planBundleFile')

beforeEach(() => callRequestRpc.mockReset())

describe('PlanDocumentsView', () => {
  it('lists the documents in reading order with a badge of what the version did', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    const tabs = await screen.findAllByRole('tab')
    expect(tabs.map((t) => t.getAttribute('data-testid'))).toEqual([
      'plan-document-tab-proposal.md',
      'plan-document-tab-design.md',
      'plan-document-tab-specs/alpha/spec.md',
      'plan-document-tab-specs/beta/spec.md',
      'plan-document-tab-tasks.md'
    ])
    expect(tabs[0]).toHaveTextContent('Proposal')
    expect(tabs[2]).toHaveTextContent('alpha')
    expect(tabs[3]).toHaveTextContent('New')
    expect(tabs[4]).toHaveTextContent('Changed')
    expect(within(tabs[0]).queryByText(/New|Changed/)).toBeNull()
    expect(within(tabs[1]).getByText('Too large to display')).toBeInTheDocument()
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
  })

  it('reads the open document only, once, and the others when their tab is opened', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    await screen.findByText('Because.')
    expect(fileCalls()).toHaveLength(1)
    expect(fileCalls()[0][1]).toEqual({ id: 'r1', path: 'proposal.md' })
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-tasks.md'), { button: 0 })
    await screen.findByRole('checkbox', { name: 'Not done' })
    expect(fileCalls()).toHaveLength(2)
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-proposal.md'), { button: 0 })
    await screen.findByText('Because.')
    expect(fileCalls()).toHaveLength(2)
  })

  it('moves between tabs with the arrow keys', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    const first = await screen.findByTestId('plan-document-tab-proposal.md')
    first.focus()
    fireEvent.keyDown(first, { key: 'ArrowRight' })
    await waitFor(() => expect(screen.getByTestId('plan-document-tab-design.md')).toHaveFocus())
  })

  it('renders tasks with read-only checkboxes and a table, and hides the machine comments', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    await screen.findByText('Because.')
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-tasks.md'), { button: 0 })
    const boxes = await screen.findAllByRole('checkbox')
    expect(boxes).toHaveLength(2)
    boxes.forEach((b) => expect(b).toBeDisabled())
    expect(screen.getByTestId('request-markdown-content').textContent).not.toContain('orca:begin')
  })

  it('shows the source text, comments included, and a copy button, and remembers the choice across tabs', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    await screen.findByText('Because.')
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-tasks.md'), { button: 0 })
    await screen.findAllByRole('checkbox')
    fireEvent.click(screen.getByRole('radio', { name: 'Source' }))
    const source = await screen.findByTestId('plan-document-source')
    expect(source.tagName).toBe('PRE')
    expect(source).toHaveTextContent('orca:begin plan')
    expect(screen.getByRole('button', { name: 'Copy document source' })).toBeInTheDocument()
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-proposal.md'), { button: 0 })
    expect((await screen.findByTestId('plan-document-source')).textContent).toContain('# Why')
  })

  it('says a file too large to display is, with its real size and the branch, and never reads it', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    await screen.findByText('Because.')
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-design.md'), { button: 0 })
    expect(await screen.findByTestId('plan-document-omitted')).toHaveTextContent(
      'This file is too large to display (8.6 MB). It is on the proposal branch request/7-proposal.'
    )
    expect(fileCalls().some(([, p]) => (p as { path: string }).path === 'design.md')).toBe(false)
  })

  it('shows a failed read with Try again', async () => {
    serve({
      fileError: {
        ok: false,
        error: { kind: 'network', code: 'NETWORK_ERROR', message: 'offline' }
      }
    })
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    const alert = await screen.findByTestId('plan-document-error')
    expect(alert).toHaveTextContent('Network error')
    serve()
    fireEvent.click(within(alert).getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('Because.')).toBeInTheDocument()
  })

  it('shows the checks and the knowledge report of the current version', async () => {
    serve({
      view: {
        checks: [{ code: 'STRUCTURE_OK', where: 'change', message: 'ok', severity: 'info' }],
        knowledge: { available: true, snapshot: 's', read: ['A'], gaps: [], conflicts: [] }
      }
    })
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    fireEvent.click(await screen.findByRole('button', { name: /Checks/ }))
    expect(screen.getByTestId('plan-checks-list')).toHaveTextContent('STRUCTURE_OK')
    expect(screen.getByTestId('plan-knowledge-report')).toBeInTheDocument()
  })

  it('reads an older version by its number, keeps the chosen document, and says checks are not kept for it', async () => {
    serve()
    const { rerender } = render(
      <PlanDocumentsView request={request} shown={null} current={rev(2)} />
    )
    await screen.findByText('Because.')
    fireEvent.mouseDown(screen.getByTestId('plan-document-tab-specs/alpha/spec.md'), { button: 0 })
    await screen.findByText('ADDED Requirements')
    rerender(<PlanDocumentsView request={request} shown={rev(1)} current={rev(2)} />)
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', {
        id: 'r1',
        revision: 1
      })
    )
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.planBundleFile', {
        id: 'r1',
        path: 'specs/alpha/spec.md',
        revision: 1
      })
    )
    expect(screen.getByTestId('plan-checks-current-only')).toBeInTheDocument()
    expect(screen.getByTestId('plan-document-tab-specs/alpha/spec.md')).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  it('shows no change badges for the first version, where everything would be new', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={rev(1)} current={rev(2)} />)
    await screen.findAllByRole('tab')
    expect(screen.queryByText('New')).toBeNull()
    expect(screen.queryByText('Changed')).toBeNull()
  })

  it('lists a version whose text is no longer kept without reading its files', async () => {
    serve()
    render(
      <PlanDocumentsView request={request} shown={rev(1, { textKept: false })} current={rev(2)} />
    )
    await screen.findAllByRole('tab')
    expect(await screen.findByTestId('plan-document-pruned')).toHaveTextContent(
      'The text of this version is no longer kept.'
    )
    expect(fileCalls()).toHaveLength(0)
  })

  it('says so for a Request older than the history, which kept no file list', async () => {
    serve({ view: { bundle: bundle({ files: [] }) } })
    render(
      <PlanDocumentsView request={request} shown={rev(1, { textKept: false })} current={rev(2)} />
    )
    expect(await screen.findByTestId('plan-documents-empty')).toHaveTextContent(
      'The text of this version is no longer kept.'
    )
  })

  it('tells a change without documents, and shows a removed file without reading it', async () => {
    serve({ view: { bundle: bundle({ files: [] }) } })
    const { unmount } = render(
      <PlanDocumentsView request={request} shown={null} current={rev(2)} />
    )
    expect(await screen.findByTestId('plan-documents-empty')).toHaveTextContent(
      'This change has no documents yet.'
    )
    unmount()
    serve({
      view: { bundle: bundle({ files: [{ path: 'design.md', size: 10, change: 'removed' }] }) }
    })
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    expect(await screen.findByTestId('plan-document-removed')).toHaveTextContent(
      'This file was removed in this version.'
    )
    expect(fileCalls()).toHaveLength(0)
  })

  it('works on a gateway without versions: the latest run, no revision numbers', async () => {
    serve()
    render(<PlanDocumentsView request={request} shown={null} current={null} />)
    await screen.findByText('Because.')
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', { id: 'r1' })
    expect(fileCalls()[0][1]).toEqual({ id: 'r1', path: 'proposal.md' })
  })

  it('shows a failed load of the document list with Try again', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'unavailable', code: 'x', message: 'down' }
    })
    render(<PlanDocumentsView request={request} shown={null} current={rev(2)} />)
    const alert = await screen.findByTestId('plan-documents-error')
    expect(alert).toHaveTextContent('The request service is unavailable')
    serve()
    fireEvent.click(within(alert).getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('Because.')).toBeInTheDocument()
  })
})
