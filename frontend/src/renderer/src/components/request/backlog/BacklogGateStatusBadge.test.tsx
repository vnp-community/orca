// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { BacklogGateStatusBadge } from './BacklogGateStatusBadge'
import { BacklogEngineBadge } from './BacklogEngineBadge'

afterEach(cleanup)

describe('BacklogGateStatusBadge', () => {
  const table: [Parameters<typeof BacklogGateStatusBadge>[0]['status'], string, string][] = [
    ['approved', 'Approved', 'status-success'], ['pending', 'Awaiting approval', 'text-primary'],
    ['rejected', 'Rejected', 'text-destructive'], ['none', 'No gate', 'text-muted-foreground'],
    ['unknown', 'Unknown', 'text-muted-foreground']
  ]
  it.each(table)('%s renders a label, an icon and only token classes', (status, label, token) => {
    render(<BacklogGateStatusBadge status={status} />)
    const badge = screen.getByTestId('gate-status-badge')
    expect(badge).toHaveTextContent(label)
    expect(badge.className).toContain(token)
    expect(badge.querySelector('svg')).not.toBeNull()
    expect(badge.className).not.toMatch(/#[0-9a-f]{3,6}|\b(red|green|blue|amber|slate|gray)-\d{3}\b/)
  })
})

describe('BacklogEngineBadge', () => {
  it('labels the three engines and dashes unknown or empty values', () => {
    const { rerender } = render(<BacklogEngineBadge engine="direct_agent" />)
    expect(screen.getByText('Direct agent')).toBeInTheDocument()
    rerender(<BacklogEngineBadge engine="orchestration" />)
    expect(screen.getByText('Orchestration')).toBeInTheDocument()
    rerender(<BacklogEngineBadge engine="mystery" />)
    expect(screen.getByText('-')).toBeInTheDocument()
    rerender(<BacklogEngineBadge engine={undefined} />)
    expect(screen.getByText('-')).toBeInTheDocument()
  })
})
