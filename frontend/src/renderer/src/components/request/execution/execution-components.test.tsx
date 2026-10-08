// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const openRequestPage = vi.fn()
vi.mock('../request-page-navigation', () => ({ openRequestPage: (...a: unknown[]) => openRequestPage(...a) }))
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: vi.fn().mockResolvedValue({ ok: false, error: { kind: 'unsupported', code: 'x', message: 'm' } }) }))

import { ExecutionResultPanel } from './ExecutionResultPanel'
import type { ExecutionResult } from '../../../../../shared/request-artifact-types'

afterEach(cleanup)

const result = (over: Partial<ExecutionResult> = {}): ExecutionResult => ({
  taskId: 't1', attempt: 2, parseStatus: 'ok', status: 'done', summary: 'Added <b>tests</b>', filesChanged: ['src/a.ts', 'src/out.ts'],
  checksRun: [{ id: 'lint', exit: 0 }, { id: 'test', exit: 0 }],
  verdict: { status: 'failed', findings: [{ code: 'SCOPE_VIOLATION', message: 'src/out.ts is outside scope' }, { code: 'CHECK_MISMATCH', message: 'test exited 1' }] },
  ...over
})

describe('ExecutionResultPanel', () => {
  it('shows summary as text, scope labels, agent-vs-Orca checks and a not-run secret scan', () => {
    const { container } = render(<ExecutionResultPanel taskId="t1" execution={{ result: result(), status: 'ready' }} />)
    expect(container.querySelector('b')).toBeNull()
    expect(screen.getByText('Added <b>tests</b>')).toBeInTheDocument()
    expect(container.querySelector('[data-in-scope="false"]')?.textContent).toContain('Out of scope')
    expect(container.querySelector('[data-in-scope="true"]')?.textContent).toContain('In scope')
    expect(screen.getByTestId('execution-checks').textContent).toContain('Mismatch')
    expect(screen.getByTestId('execution-secret-scan').dataset.state).toBe('notRun')
    expect(screen.getByText('Attempt 2')).toBeInTheDocument()
  })

  it('shows "Not re-run yet" when there is no verdict', () => {
    render(<ExecutionResultPanel taskId="t1" execution={{ result: result({ verdict: undefined }), status: 'ready' }} />)
    expect(screen.getAllByText('Not re-run yet')).toHaveLength(2)
    expect(screen.getByTestId('execution-secret-scan').dataset.state).toBe('notRun')
  })

  it('malformed results show the plain-text tail; legacy shows the old stdout', () => {
    const { unmount } = render(<ExecutionResultPanel taskId="t1" execution={{ result: result({ parseStatus: 'invalid', stdoutTail: '<script>x</script> oops' }), status: 'ready' }} />)
    expect(screen.getByTestId('execution-malformed').textContent).toContain('<script>x</script> oops')
    expect(document.querySelector('script')).toBeNull()
    unmount()
    render(<ExecutionResultPanel taskId="t1" legacyOutput="old stdout" execution={{ result: null, status: 'legacy' }} />)
    expect(screen.getByTestId('execution-legacy').textContent).toContain('old stdout')
  })

  it('renders nothing for unsupported runtimes', () => {
    expect(render(<ExecutionResultPanel taskId="t1" execution={{ result: null, status: 'unsupported' }} />).container).toBeEmptyDOMElement()
  })

  it('routes needs_info to the request page and explains the failure route', () => {
    render(<ExecutionResultPanel taskId="t1" requestId="r1" execution={{ result: result({ status: 'needs_info', failureClass: 'needs_info', verdict: undefined }), status: 'ready' }} />)
    expect(screen.getByTestId('execution-failure-route')).toHaveTextContent('Turned into a question')
    fireEvent.click(screen.getByText('Answer the question'))
    expect(openRequestPage).toHaveBeenCalledWith({ section: 'requests', requestId: 'r1' })
  })

  it('shows output names and types and previews content as text', () => {
    render(<ExecutionResultPanel taskId="t1" execution={{ result: result({ outputs: { report: { a: 1 } } }), status: 'ready' }} />)
    expect(screen.getByText('report')).toBeInTheDocument()
    fireEvent.click(screen.getByText('View'))
    expect(screen.getByText(/"a": 1/)).toBeInTheDocument()
  })
})
