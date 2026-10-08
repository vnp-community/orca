// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { RiskBadge } from './RiskBadge'
import type { GraphRisk } from '../../../../shared/graph-types'

afterEach(cleanup)

const LEVELS: GraphRisk[] = ['low', 'medium', 'high', 'critical', 'unknown']

describe('RiskBadge', () => {
  it.each(LEVELS)('renders text and icon for %s', (level) => {
    const { container } = render(<RiskBadge level={level} />)
    expect(container.textContent?.trim()).toBeTruthy()
    expect(container.querySelector('svg')).not.toBeNull()
    expect(container.querySelector(`[data-risk-level="${level}"]`)).not.toBeNull()
  })

  it('keeps an accessible name when the label is hidden', () => {
    render(<RiskBadge level="high" showLabel={false} />)
    expect(screen.getByLabelText('High')).toBeInTheDocument()
  })

  it('shows "Not assessed" for unknown and never claims safety', () => {
    const { container } = render(<RiskBadge level="unknown" />)
    expect(container.textContent).toContain('Not assessed')
    for (const l of LEVELS) {
      const r = render(<RiskBadge level={l} />)
      expect(r.container.textContent?.toLowerCase()).not.toContain('safe')
      r.unmount()
    }
  })
})
