// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))

import { ImpactFindingList } from './ImpactFindingList'
import { RiskSummaryCard } from './RiskSummaryCard'
import { SolutionDimensionTable } from './SolutionDimensionTable'

const ok = (value: unknown) => ({ ok: true, value })
const fail = (kind: string) => ({ ok: false, error: { kind, code: 'x', message: 'm' } })
function route(map: Record<string, unknown>) {
  callRequestRpc.mockImplementation(async (m: string) => map[m] ?? fail('unsupported'))
}

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(() => { cleanup(); vi.useRealTimers() })

describe('RiskSummaryCard', () => {
  const summary = { assessment_id: 'a', digest: 'd', level: 'high', score: 62, top_reasons: ['Touches billing', 'No tests'], assessed_at: '2026-10-07', tool: 'codegraph', mode: 'enforce', status: 'ready', hard_rules: ['migration'] }

  it('shows level, score, reasons, caption and the rule that raised the level', async () => {
    route({ 'impact.get': ok(summary), 'impact.findings': ok({ findings: [] }) })
    render(<RiskSummaryCard subjectType="plan" subjectId="p" />)
    expect(await screen.findByText('Touches billing')).toBeInTheDocument()
    expect(screen.getByText('Score 62/100')).toBeInTheDocument()
    expect(screen.getByText(/Raised because: migration/)).toBeInTheDocument()
    expect(screen.getByText(/based on codegraph/)).toBeInTheDocument()
    expect(screen.queryByText('Advisory')).toBeNull()
  })

  it('labels shadow mode as advisory and stale assessments', async () => {
    route({ 'impact.get': ok({ ...summary, mode: 'shadow', stale: true }), 'impact.findings': ok({ findings: [] }) })
    render(<RiskSummaryCard subjectType="plan" subjectId="p" />)
    expect(await screen.findByText('Advisory')).toBeInTheDocument()
    expect(screen.getByText(/out of date/)).toBeInTheDocument()
  })

  it('shows "Not assessed" (never Low) when unsupported or no assessment exists', async () => {
    route({})
    const a = render(<RiskSummaryCard subjectType="plan" subjectId="p" />)
    await waitFor(() => expect(a.container.textContent).toContain('Not assessed'))
    expect(a.container.textContent).not.toContain('Low')
    a.unmount()
    route({ 'impact.get': ok({}) })
    const b = render(<RiskSummaryCard subjectType="plan" subjectId="p2" />)
    expect(await screen.findByText('Run assessment')).toBeInTheDocument()
    expect(b.container.textContent).not.toContain('Low')
  })

  it('shows forbidden and no-dev-server states', async () => {
    route({ 'impact.get': fail('forbidden') })
    const a = render(<RiskSummaryCard subjectType="plan" subjectId="f" />)
    expect(await screen.findByText(/do not have permission/)).toBeInTheDocument()
    a.unmount()
    route({ 'impact.get': fail('no_dev_server') })
    const onConnect = vi.fn()
    render(<RiskSummaryCard subjectType="plan" subjectId="n" onConnectDevServer={onConnect} />)
    fireEvent.click(await screen.findByText('Connect'))
    expect(onConnect).toHaveBeenCalled()
  })

  it('shows the collecting skeleton only after a short delay', async () => {
    callRequestRpc.mockReturnValue(new Promise(() => {}))
    const { container } = render(<RiskSummaryCard subjectType="plan" subjectId="c" />)
    expect(container.querySelector('[role="status"]')).toBeNull()
    await waitFor(() => expect(container.querySelector('[role="status"]')).not.toBeNull(), { timeout: 1500 })
  })
})

describe('SolutionDimensionTable', () => {
  const options = [{ id: 'a', title: 'Option A' }, { id: 'b', title: 'Option B' }]
  it('renders badges, marks the recommended column by text and flags differences', () => {
    render(
      <SolutionDimensionTable
        options={options}
        recommendedId="a"
        comparison={[
          { optionId: 'a', dimensions: { data: { level: 'high', score: 3, note: 'schema change' } } },
          { optionId: 'b', dimensions: { data: { level: 'low', score: 1 } } }
        ]}
      />
    )
    expect(screen.getByText('(Recommended)')).toBeInTheDocument()
    expect(screen.getByText('schema change')).toBeInTheDocument()
    expect(screen.getAllByText('Not assessed').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Differs between options').length).toBe(1)
    expect(screen.queryByRole('radio')).toBeNull()
  })
  it('renders nothing without comparison data', () => {
    const { container } = render(<SolutionDimensionTable options={options} comparison={null} />)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('ImpactFindingList', () => {
  const findings = [
    { id: 'f1', dimension: 'data', level: 'medium' as const, title: 'Index drop' },
    { id: 'f2', dimension: 'data', level: 'critical' as const, title: 'Column removed', nodeIds: ['n1'] }
  ]
  it('sorts by risk within a dimension and offers graph view only with node ids', () => {
    const onView = vi.fn()
    render(<ImpactFindingList findings={findings} onViewOnGraph={onView} />)
    const items = screen.getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Column removed')
    expect(screen.getAllByText('View on graph')).toHaveLength(1)
    fireEvent.click(screen.getByText('View on graph'))
    expect(onView).toHaveBeenCalledWith(findings[1])
  })

  it('opens evidence as plain text, not HTML', async () => {
    route({ 'impact.evidence': ok({ evidence: '<img src=x onerror=alert(1)> call path', truncated: true }) })
    render(<ImpactFindingList findings={findings} />)
    fireEvent.click(screen.getAllByText('Evidence')[0])
    const pre = await screen.findByTestId('impact-evidence-text')
    expect(pre.textContent).toContain('<img src=x')
    expect(document.querySelector('img')).toBeNull()
    expect(screen.getByText('Truncated')).toBeInTheDocument()
    expect(callRequestRpc).toHaveBeenCalledWith('impact.evidence', { findingId: 'f2' })
  })

  it('shows an empty line', () => {
    render(<ImpactFindingList findings={[]} />)
    expect(screen.getByText(/No findings/)).toBeInTheDocument()
  })
})
