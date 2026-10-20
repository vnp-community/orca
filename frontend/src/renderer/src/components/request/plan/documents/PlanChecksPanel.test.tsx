// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { PlanCheck, PlanKnowledge } from '../../../../../../shared/request-plan-revision'
import { PlanChecksPanel } from './PlanChecksPanel'

afterEach(cleanup)

const ok: PlanCheck = {
  code: 'STRUCTURE_OK',
  where: 'change',
  message: 'The change is valid',
  severity: 'info'
}
const warn: PlanCheck = {
  code: 'KNOWLEDGE_GAP',
  where: 'design.md',
  message: 'No ADR cited',
  severity: 'warning'
}
const redacted: PlanCheck = {
  code: 'SECRETS_REDACTED',
  where: '',
  message: '2 secrets were masked in the stored copy',
  severity: 'warning'
}
const knowledge: PlanKnowledge = {
  available: true,
  snapshot: 'snap-7',
  read: ['REQ-1', 'ADR-4'],
  gaps: ['No retention rule'],
  conflicts: ['REQ-1 vs ADR-4']
}

describe('PlanChecksPanel', () => {
  it('says checks are for the current version only when an older one is on screen', () => {
    render(<PlanChecksPanel checks={[ok]} knowledge={knowledge} isCurrent={false} />)
    expect(screen.getByTestId('plan-checks-current-only')).toHaveTextContent(
      'Checks are shown for the current version only.'
    )
    expect(screen.queryByTestId('plan-checks-panel')).toBeNull()
  })

  it('renders nothing when there is nothing to report', () => {
    const { container } = render(<PlanChecksPanel checks={[]} knowledge={null} isCurrent />)
    expect(container).toBeEmptyDOMElement()
  })

  it('stays closed when every check is information, and opens on a click', () => {
    render(<PlanChecksPanel checks={[ok]} knowledge={null} isCurrent />)
    expect(screen.queryByTestId('plan-checks-list')).toBeNull()
    expect(screen.queryByTestId('plan-checks-warning-count')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Checks/ }))
    expect(screen.getByTestId('plan-checks-list')).toHaveTextContent('STRUCTURE_OK')
  })

  it('opens at once for a warning and names the level in words, with code, place and message', () => {
    render(<PlanChecksPanel checks={[ok, warn]} knowledge={null} isCurrent />)
    expect(screen.getByTestId('plan-checks-warning-count')).toHaveTextContent('1 to review')
    const rows = screen.getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent('Info')
    expect(rows[1]).toHaveTextContent('Warning')
    expect(rows[1]).toHaveTextContent('KNOWLEDGE_GAP')
    expect(rows[1]).toHaveTextContent('design.md')
    expect(rows[1]).toHaveTextContent('No ADR cited')
  })

  it('shows the secrets warning like any other check', () => {
    render(<PlanChecksPanel checks={[redacted]} knowledge={null} isCurrent />)
    expect(screen.getByTestId('plan-checks-list')).toHaveTextContent('SECRETS_REDACTED')
    expect(screen.getByTestId('plan-checks-list')).toHaveTextContent('2 secrets were masked')
  })

  it('reports the knowledge as the agent’s own word, with what it read, its gaps and conflicts', () => {
    render(<PlanChecksPanel checks={[warn]} knowledge={knowledge} isCurrent />)
    const report = screen.getByTestId('plan-knowledge-report')
    expect(report).toHaveTextContent('Reported by the authoring agent')
    expect(report).toHaveTextContent('Design knowledge was available.')
    expect(report).toHaveTextContent('Snapshot snap-7.')
    expect(report).toHaveTextContent('Elements read: 2')
    expect(screen.getByTestId('plan-knowledge-gaps')).toHaveTextContent('No retention rule')
    expect(screen.getByTestId('plan-knowledge-conflicts')).toHaveTextContent('REQ-1 vs ADR-4')
    expect(screen.getByTestId('plan-knowledge-disclaimer')).toHaveTextContent(
      "This is the agent's own report; Orca did not verify it."
    )
  })

  it('says when no design knowledge was available and omits empty lists', () => {
    render(
      <PlanChecksPanel
        checks={[warn]}
        knowledge={{ available: false, snapshot: '', read: [], gaps: [], conflicts: [] }}
        isCurrent
      />
    )
    expect(screen.getByTestId('plan-knowledge-report')).toHaveTextContent(
      'Design knowledge was not available.'
    )
    expect(screen.queryByTestId('plan-knowledge-gaps')).toBeNull()
    expect(screen.queryByTestId('plan-knowledge-conflicts')).toBeNull()
    expect(screen.queryByText(/Elements read/)).toBeNull()
  })
})
