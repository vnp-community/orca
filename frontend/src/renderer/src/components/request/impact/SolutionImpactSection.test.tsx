// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))

import { SolutionImpactSection } from './SolutionImpactSection'
import type { Solution } from '../../../../../shared/request-types'

const solution: Solution = {
  id: 's1', requestId: 'r1', kind: 'solution', status: 'ready',
  options: [{ id: 'o1', title: 'A', raw: { recommended: true } }, { id: 'o2', title: 'B' }]
}
const ok = (value: unknown) => ({ ok: true, value })

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)

describe('SolutionImpactSection', () => {
  it('renders nothing for non-solution kinds without calling the backend', () => {
    const { container } = render(<SolutionImpactSection solution={{ ...solution, kind: 'answer' }} selectedId={null} />)
    expect(container).toBeEmptyDOMElement()
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('hides itself when the runtime has no impact channels', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'unsupported', code: 'x', message: 'm' } })
    const { container } = render(<SolutionImpactSection solution={solution} selectedId="o1" />)
    await waitFor(() => expect(container).toBeEmptyDOMElement())
  })

  it('shows the risk card and the dimension table for the focused option', async () => {
    callRequestRpc.mockImplementation(async (m: string) => {
      if (m === 'impact.get') {return ok({ assessment_id: 'a', digest: 'd', level: 'medium', status: 'ready', mode: 'enforce' })}
      if (m === 'impact.findings') {return ok({ findings: [{ id: 'f', dimension: 'data', level: 'medium', title: 'F' }] })}
      if (m === 'impact.compare') {return ok({ comparison: [{ option_id: 'o1', dimensions: { data: { level: 'medium', score: 2 } } }, { option_id: 'o2', dimensions: {} }] })}
      return { ok: false, error: { kind: 'unsupported', code: 'x', message: 'm' } }
    })
    const { container } = render(<SolutionImpactSection solution={solution} selectedId={null} />)
    await waitFor(() => expect(container.querySelector('[data-testid="solution-dimension-table"]')).not.toBeNull())
    expect(container.querySelector('[data-testid="risk-summary-card"]')).not.toBeNull()
    expect(callRequestRpc).toHaveBeenCalledWith('impact.get', { subjectType: 'solution_option', subjectId: 'o1' })
    expect(container.textContent).toContain('(Recommended)')
  })
})
