// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import React from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { TestSpecsPanel } from './TestSpecsPanel'
import { parseTestSpecs } from '../../../../../shared/request-testspec-parsers'
import type { UseTestSpecsResult } from '../../../hooks/useTestSpecs'

function hook(raw: unknown, over: Partial<UseTestSpecsResult> = {}): UseTestSpecsResult {
  return {
    testSpecs: parseTestSpecs(raw),
    isLoading: false,
    error: null,
    unsupported: false,
    restarting: false,
    restartError: null,
    refetch: vi.fn(),
    restart: vi.fn(),
    ...over
  }
}

describe('TestSpecsPanel', () => {
  afterEach(cleanup)

  it('renders nothing when the lane is off', () => {
    const { container } = render(
      <TestSpecsPanel requestId="r1" testSpecs={hook({ enabled: false })} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('says it starts with the execution when there is no run yet', () => {
    render(<TestSpecsPanel requestId="r1" testSpecs={hook({ enabled: true })} />)
    expect(screen.getByTestId('testspec-none')).toBeTruthy()
  })

  it('shows why a run is blocked and offers a Re-run', () => {
    const h = hook({
      enabled: true,
      current: {
        id: 'a',
        attempt: 2,
        status: 'blocked',
        reasonCode: 'REQUEST_TESTSPEC_NO_DEV_SERVER',
        reasonMessage: 'No dev server'
      }
    })
    render(<TestSpecsPanel requestId="r1" testSpecs={h} />)
    expect(screen.getByTestId('testspec-reason').textContent).toContain('No dev server')
    fireEvent.click(screen.getByRole('button', { name: /re-run/i }))
    expect(h.restart).toHaveBeenCalled()
  })

  it('shows counts and open questions of a finished run, without a Re-run while it is active', () => {
    const done = hook({
      enabled: true,
      current: {
        id: 'a',
        attempt: 1,
        status: 'completed',
        contextMode: 'solution',
        summary: {
          counts: { TestRequirement: 3, TestCase: 7, TestGap: 1 },
          gaps: [{ question: 'Max size?', owner: 'BA' }]
        }
      }
    })
    const { unmount } = render(<TestSpecsPanel requestId="r1" testSpecs={done} />)
    expect(screen.getByTestId('testspec-counts').textContent).toContain('7')
    expect(screen.getByTestId('testspec-gaps').textContent).toContain('Max size?')
    unmount()
    render(
      <TestSpecsPanel
        requestId="r1"
        testSpecs={hook({ enabled: true, current: { id: 'b', attempt: 1, status: 'running' } })}
      />
    )
    expect(screen.queryByRole('button', { name: /re-run/i })).toBeNull()
  })
})
