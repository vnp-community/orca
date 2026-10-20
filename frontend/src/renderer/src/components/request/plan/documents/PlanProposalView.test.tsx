// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { PlanProposalView } from './PlanProposalView'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'

afterEach(cleanup)
beforeEach(() => callRequestRpc.mockReset())

const request = {
  id: 'r1',
  status: 'awaiting_plan_approval' as const,
  version: 3,
  planTaskId: 'plan1'
}
const rev = (n: number): PlanRevision => ({
  revision: n,
  runId: `run-${n}`,
  status: n === 2 ? 'current' : 'superseded',
  engine: 'native',
  revisionFeedback: '',
  actorId: '',
  generatedAt: '',
  commit: '',
  filesChanged: 0,
  fileCount: 0,
  textKept: true
})

const PROPOSAL = {
  title: 'Fix the refund total',
  summary: 'The refund **rounds** wrongly.',
  notes: '- watch the **currency** column',
  phases: [
    {
      title: 'Investigate',
      description: 'Find the cause.',
      tasks: [
        {
          title: 'Reproduce',
          description: 'Write a failing test.',
          taskType: 'bug',
          estimatedHours: 1.5,
          promptTemplate: 'Run the suite and write a failing test.',
          labels: ['tests']
        },
        {
          title: 'Fix',
          description: 'Round half up.',
          taskType: 'task',
          dependsOnIndices: [0, 7],
          irreversible: true
        }
      ]
    },
    { title: 'Ship', tasks: [{ title: 'Release' }], dependsOnPhaseIndices: [0] }
  ]
}

function serve(view: unknown) {
  callRequestRpc.mockResolvedValue({ ok: true, value: view })
}

describe('PlanProposalView', () => {
  it('shows the summary, the notes, every phase and every task field the tree leaves out', async () => {
    serve({ runId: 'run-2', status: 'succeeded', engine: 'native', proposal: PROPOSAL })
    render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    expect(await screen.findByRole('heading', { name: 'Fix the refund total' })).toBeInTheDocument()
    expect(screen.getByText('rounds').tagName).toBe('STRONG')
    expect(within(screen.getByTestId('proposal-notes')).getByText('currency').tagName).toBe(
      'STRONG'
    )
    expect(screen.getByTestId('proposal-phase-1')).toHaveTextContent('Phase 1: Investigate')
    expect(screen.getByTestId('proposal-phase-2')).toHaveTextContent('Phase 2: Ship')
    expect(screen.getByTestId('proposal-phase-2')).toHaveTextContent('Depends on phase #1')
    const first = within(screen.getByTestId('proposal-phase-1')).getByTestId('proposal-task-1')
    expect(first).toHaveTextContent('1. Reproduce')
    expect(first).toHaveTextContent('bug')
    expect(first).toHaveTextContent('1.5 h')
    expect(first).toHaveTextContent('tests')
    expect(first).toHaveTextContent('Write a failing test.')
    expect(first).toHaveTextContent('Agent prompt')
  })

  it('marks an irreversible task in words and lists its valid dependencies only', async () => {
    serve({ runId: 'run-2', status: 'succeeded', proposal: PROPOSAL })
    render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    const second = await screen.findByTestId('proposal-task-2')
    expect(within(second).getByTestId('proposal-irreversible')).toHaveTextContent('Irreversible')
    expect(within(second).getByTestId('proposal-depends')).toHaveTextContent('Depends on #1')
    expect(within(second).getByTestId('proposal-depends')).not.toHaveTextContent('#8')
  })

  it('keeps the agent prompt in a closed details block as plain text', async () => {
    serve({ runId: 'run-2', status: 'succeeded', proposal: PROPOSAL })
    const { container } = render(
      <PlanProposalView request={request} shown={null} current={rev(2)} />
    )
    await screen.findByText('Agent prompt')
    const details = container.querySelector('details')
    expect(details).not.toBeNull()
    expect(details?.open).toBe(false)
    expect(details?.querySelector('pre')?.textContent).toBe(
      'Run the suite and write a failing test.'
    )
  })

  it('shows a plan with no phase as a flat list, and says so when the proposal has no task', async () => {
    serve({ status: 'succeeded', proposal: { title: 'Small', tasks: [{ title: 'Only' }] } })
    const { unmount } = render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    expect(await screen.findByTestId('proposal-task-1')).toHaveTextContent('1. Only')
    expect(screen.queryByTestId('proposal-phase-1')).toBeNull()
    unmount()
    serve({ status: 'succeeded', proposal: { title: 'Empty' } })
    render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    expect(await screen.findByTestId('proposal-no-tasks')).toBeInTheDocument()
  })

  it('has no button that edits or runs anything', async () => {
    serve({ runId: 'run-2', status: 'succeeded', proposal: PROPOSAL })
    render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    await screen.findByTestId('proposal-phase-1')
    const names = screen.getAllByRole('button').map((b) => b.textContent)
    expect(names.join(' ')).not.toMatch(/edit|run|save|delete/i)
  })

  it('drops raw HTML from the AI text', async () => {
    serve({
      status: 'succeeded',
      proposal: {
        title: 'T',
        summary: 'hello\n\n<script>window.__x = 1</script>\n\nbye',
        tasks: []
      }
    })
    const { container } = render(
      <PlanProposalView request={request} shown={null} current={rev(2)} />
    )
    await screen.findByText('hello')
    expect(container.querySelector('script')).toBeNull()
    expect(container.textContent).not.toContain('__x')
  })

  it('asks for the run and the number of the version on screen', async () => {
    serve({ status: 'succeeded', proposal: PROPOSAL })
    const { rerender } = render(
      <PlanProposalView request={request} shown={null} current={rev(2)} />
    )
    await screen.findByTestId('proposal-phase-1')
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', {
      id: 'r1',
      runId: 'run-2',
      revision: 2
    })
    rerender(<PlanProposalView request={request} shown={rev(1)} current={rev(2)} />)
    await screen.findByTestId('proposal-phase-1')
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', {
      id: 'r1',
      runId: 'run-1',
      revision: 1
    })
  })

  it('shows a skeleton while loading and a failed read with Try again', async () => {
    callRequestRpc.mockImplementation(() => new Promise(() => undefined))
    const { unmount } = render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    expect(screen.getByTestId('plan-proposal-loading')).toHaveAttribute('aria-busy', 'true')
    unmount()
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'network', code: 'NETWORK_ERROR', message: 'offline' }
    })
    render(<PlanProposalView request={request} shown={null} current={rev(2)} />)
    const alert = await screen.findByTestId('plan-proposal-error')
    serve({ status: 'succeeded', proposal: PROPOSAL })
    fireEvent.click(within(alert).getByRole('button', { name: 'Try again' }))
    expect(await screen.findByTestId('proposal-phase-1')).toBeInTheDocument()
  })

  it('says a run that is still writing, or failed, has no proposal to show', async () => {
    serve({ status: 'running', proposal: null })
    const { unmount } = render(<PlanProposalView request={request} shown={null} current={null} />)
    expect(await screen.findByTestId('plan-proposal-empty')).toHaveTextContent(
      'A new version is being written...'
    )
    unmount()
    serve({ status: 'failed', errorCode: 'REQUEST_AI_COMPLETE_TIMEOUT', proposal: null })
    render(<PlanProposalView request={request} shown={null} current={null} />)
    expect(await screen.findByTestId('plan-proposal-empty')).toHaveTextContent(
      'The latest run failed (REQUEST_AI_COMPLETE_TIMEOUT)'
    )
  })

  it('on a gateway without versions it reads the latest run and warns it may not be saved yet', async () => {
    serve({ status: 'succeeded', proposal: PROPOSAL })
    render(
      <PlanProposalView
        request={{ ...request, status: 'planning' as const }}
        shown={null}
        current={null}
      />
    )
    await screen.findByTestId('proposal-phase-1')
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', { id: 'r1' })
    expect(screen.getByTestId('plan-proposal-unsaved')).toHaveTextContent(
      'It is not saved as the plan yet.'
    )
  })

  it('does not warn about an unsaved proposal when the version is known', async () => {
    serve({ status: 'succeeded', proposal: PROPOSAL })
    render(
      <PlanProposalView
        request={{ ...request, status: 'planning' as const }}
        shown={null}
        current={rev(2)}
      />
    )
    await screen.findByTestId('proposal-phase-1')
    expect(screen.queryByTestId('plan-proposal-unsaved')).toBeNull()
  })
})
