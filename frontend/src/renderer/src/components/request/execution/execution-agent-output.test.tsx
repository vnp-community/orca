// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../request-page-navigation', () => ({ openRequestPage: vi.fn() }))
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: vi.fn() }))

import { ExecutionResultPanel } from './ExecutionResultPanel'
import { AGENT_OUTPUT_MAX_LINES, prepareAgentOutputTail } from './execution-agent-output-text'
import type { ExecutionResult } from '../../../../../shared/request-artifact-types'

afterEach(cleanup)

const tailResult = (over: Partial<ExecutionResult> = {}): ExecutionResult => ({
  taskId: 't1',
  attempt: 1,
  parseStatus: 'missing',
  filesChanged: [],
  checksRun: [],
  source: 'last_execution_output',
  stdoutTail: 'Ran 3 tests\nOK',
  ...over
})

describe('ExecutionResultPanel with the output of the last run', () => {
  it('shows a neutral "Agent output" view, not the red unexpected-format warning', () => {
    render(
      <ExecutionResultPanel taskId="t1" execution={{ result: tailResult(), status: 'ready' }} />
    )
    expect(screen.getByTestId('execution-agent-output')).toHaveTextContent('Agent output')
    expect(screen.getByTestId('execution-agent-output-text')).toHaveTextContent('Ran 3 tests OK')
    expect(screen.queryByTestId('execution-malformed')).toBeNull()
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByText(/Attempt/)).toBeNull()
    expect(screen.queryByTestId('execution-secret-scan')).toBeNull()
  })

  it('notes when the gateway hid secrets or cut the output', () => {
    render(
      <ExecutionResultPanel
        taskId="t1"
        execution={{ result: tailResult({ redacted: true, truncated: true }), status: 'ready' }}
      />
    )
    const note = screen.getByTestId('execution-agent-output')
    expect(note).toHaveTextContent('Secrets were hidden.')
    expect(note).toHaveTextContent('Only the last part is shown.')
  })

  it('masks credential shapes that slipped through and renders markup as text', () => {
    const { container } = render(
      <ExecutionResultPanel
        taskId="t1"
        execution={{
          result: tailResult({
            stdoutTail: 'key sk-ant-abcdefghijklmnopqrstuvwxyz0123\n<img src=x onerror=alert(1)>'
          }),
          status: 'ready'
        }}
      />
    )
    const text = screen.getByTestId('execution-agent-output-text')
    expect(text).not.toHaveTextContent('sk-ant')
    expect(text).toHaveTextContent('<img src=x onerror=alert(1)>')
    expect(container.querySelector('img')).toBeNull()
  })

  it('says "No output" for an empty tail', () => {
    render(
      <ExecutionResultPanel
        taskId="t1"
        execution={{ result: tailResult({ stdoutTail: undefined }), status: 'ready' }}
      />
    )
    expect(screen.getByTestId('execution-agent-output-empty')).toHaveTextContent('No output')
  })

  it('keeps the structured view and the malformed warning for contract runs', () => {
    const { unmount } = render(
      <ExecutionResultPanel
        taskId="t1"
        execution={{
          result: tailResult({ source: 'execution_record', parseStatus: 'ok', attempt: 2 }),
          status: 'ready'
        }}
      />
    )
    expect(screen.getByText('Attempt 2')).toBeInTheDocument()
    unmount()
    render(
      <ExecutionResultPanel
        taskId="t1"
        execution={{
          result: tailResult({ source: 'execution_record', parseStatus: 'invalid' }),
          status: 'ready'
        }}
      />
    )
    expect(screen.getByTestId('execution-malformed')).toBeInTheDocument()
  })
})

describe('prepareAgentOutputTail', () => {
  it('keeps the last lines, drops trailing blanks and normalises CRLF', () => {
    const text = `${Array.from({ length: AGENT_OUTPUT_MAX_LINES + 20 }, (_, i) => `l${i}`).join('\r\n')}\r\n\r\n`
    const out = prepareAgentOutputTail(text)
    const lines = out.text.split('\n')
    expect(lines).toHaveLength(AGENT_OUTPUT_MAX_LINES)
    expect(lines.at(-1)).toBe(`l${AGENT_OUTPUT_MAX_LINES + 19}`)
    expect(out.cut).toBe(true)
  })

  it('shortens very long lines', () => {
    const out = prepareAgentOutputTail(`${'a'.repeat(5000)}\nshort`)
    expect(out.text.split('\n')[0]?.length).toBeLessThanOrEqual(400)
    expect(out.cut).toBe(true)
    expect(prepareAgentOutputTail('short').cut).toBe(false)
  })
})
