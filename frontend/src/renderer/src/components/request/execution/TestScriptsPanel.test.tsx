// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { TestScriptsPanel } from './TestScriptsPanel'
import { parseTestScripts } from '../../../../../shared/request-testscript-parsers'
import type { UseTestScriptsResult } from '../../../hooks/useTestScripts'

vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: vi.fn(async (method: string, args: { path?: string }) => ({
    ok: true,
    value: {
      path: args.path,
      text: method === 'request.testScriptReport' ? '# TSA: TDD-NOTES v2' : 'import pytest',
      hasNext: false
    }
  }))
}))

function hook(raw: unknown, over: Partial<UseTestScriptsResult> = {}): UseTestScriptsResult {
  return {
    testScripts: parseTestScripts(raw),
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

const done = {
  id: 'a',
  attempt: 1,
  status: 'needs_review',
  reasonCode: 'REQUEST_TESTSCRIPT_NEEDS_REVIEW',
  feature: 'TDD-NOTES',
  specVersion: 2,
  branch: 'tsa/REQ-12-TDD-NOTES-v2',
  commitSha: '20d7eb4ffff',
  summary: {
    counts: { files: 2, generated: 9, 'needs-review': 1, failed: 0, skipped: 0 },
    files: [
      { path: 'tests/tsa/x/test_a.py', status: 'generated', errors: 0 },
      { path: 'tests/tsa/x/test_b.py', status: 'needs-review', errors: 2 }
    ]
  }
}

describe('TestScriptsPanel', () => {
  afterEach(cleanup)

  it('renders nothing when the lane is off or the gateway lacks it', () => {
    const { container } = render(
      <TestScriptsPanel requestId="r1" testScripts={hook({ enabled: false })} />
    )
    expect(container).toBeEmptyDOMElement()
    const { container: c2 } = render(
      <TestScriptsPanel
        requestId="r1"
        testScripts={hook({ enabled: true }, { unsupported: true })}
      />
    )
    expect(c2).toBeEmptyDOMElement()
  })

  it('says when it starts while there is no run yet', () => {
    render(<TestScriptsPanel requestId="r1" testScripts={hook({ enabled: true })} />)
    expect(screen.getByTestId('testscript-none')).toBeTruthy()
  })

  it('shows where the scripts are, the counts and the files to review, and opens the output', async () => {
    render(<TestScriptsPanel requestId="r1" testScripts={hook({ enabled: true, current: done })} />)
    expect(screen.getByTestId('testscript-status').textContent).toContain('TDD-NOTES v2')
    expect(screen.getByTestId('testscript-where').textContent).toContain('tsa/REQ-12-TDD-NOTES-v2')
    expect(screen.getByTestId('testscript-where').textContent).toContain('20d7eb4f')
    expect(screen.getByTestId('testscript-counts').textContent).toContain('9')
    const flagged = screen.getByTestId('testscript-flagged')
    expect(flagged.textContent).toContain('test_b.py')
    expect(flagged.textContent).not.toContain('test_a.py')
    fireEvent.click(screen.getByRole('button', { name: /open output/i }))
    expect(await screen.findByTestId('testscript-output')).toBeTruthy()
    expect((await screen.findByTestId('testscript-code')).textContent).toContain('import pytest')
  })

  it('opens the output on a flagged file', async () => {
    render(<TestScriptsPanel requestId="r1" testScripts={hook({ enabled: true, current: done })} />)
    fireEvent.click(screen.getByRole('button', { name: 'tests/tsa/x/test_b.py' }))
    const picked = await screen.findAllByRole('button', { pressed: true })
    expect(picked.some((b) => b.textContent?.includes('test_b.py'))).toBe(true)
  })

  it('shows why a run is blocked and offers a Re-run, but no Re-run while it runs', () => {
    const blocked = hook({
      enabled: true,
      current: {
        id: 'a',
        attempt: 2,
        status: 'blocked',
        reasonCode: 'REQUEST_TESTSCRIPT_TOOL_MISSING',
        reasonMessage: 'not allowed'
      }
    })
    render(<TestScriptsPanel requestId="r1" testScripts={blocked} />)
    expect(screen.getByTestId('testscript-reason').textContent).toContain('not allowed')
    fireEvent.click(screen.getByRole('button', { name: /re-run/i }))
    expect(blocked.restart).toHaveBeenCalled()
    cleanup()
    render(
      <TestScriptsPanel
        requestId="r1"
        testScripts={hook({ enabled: true, current: { id: 'b', status: 'running' } })}
      />
    )
    expect(screen.queryByRole('button', { name: /re-run/i })).toBeNull()
  })
})
