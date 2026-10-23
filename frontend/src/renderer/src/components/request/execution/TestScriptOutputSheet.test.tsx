// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { TestScriptOutputSheet } from './TestScriptOutputSheet'
import { parseTestScriptRun } from '../../../../../shared/request-testscript-parsers'
import { callRequestRpc } from '../../../runtime/request-rpc-client'

const SCRIPT = [
  'import pytest',
  '',
  '@pytest.mark.tc("TC-1", spec_version=1)',
  'def test_one(api_client):',
  '    r = api_client.get("/notes")',
  '    assert r.status_code == 200  # @exp TC-1.E1'
].join('\n')

const MANIFEST = JSON.stringify({
  schema: 'tsa-manifest/v1',
  feature: 'TDD-NOTES',
  specVersion: 1,
  assertions: [
    {
      exp: 'TC-1.E1',
      case: 'TC-1',
      kind: 'status',
      op: '==',
      value: '200',
      status: 'ok',
      file: 'tests/tsa/x/test_a.py',
      line: 6,
      function: 'test_one',
      assertion: 'r.status_code == 200'
    },
    { exp: 'TC-2.E1', case: 'TC-2', kind: 'state', op: '==', value: 'NEW', status: 'missing' },
    {
      exp: 'TC-3.E1',
      case: 'TC-3',
      kind: 'errorCode',
      op: '==',
      value: 'Mã lỗi cần bổ sung',
      status: 'gap',
      reason: 'mã lỗi khi rỗng chưa có (TGAP-1)',
      file: 'tests/tsa/x/test_a.py',
      line: 9
    }
  ]
})

vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: vi.fn(async (method: string, args: { path?: string; offset?: number }) => {
    if (method === 'request.testScriptReport') {
      return { ok: true, value: { text: '# TSA report\n\n- Test case: đã sinh 1', hasNext: false } }
    }
    if (args.path?.endsWith('assertion-manifest.json')) {
      return { ok: true, value: { path: args.path, text: MANIFEST, hasNext: false } }
    }
    // A long file comes in two pages.
    if (args.offset === 0) {
      return {
        ok: true,
        value: { path: args.path, text: SCRIPT.slice(0, 20), hasNext: true, next: 20 }
      }
    }
    return { ok: true, value: { path: args.path, text: SCRIPT.slice(20), hasNext: false } }
  })
}))

const run = parseTestScriptRun({
  id: 'a',
  status: 'completed',
  branch: 'tsa/REQ-1-TDD-NOTES-v1',
  commitSha: 'abcdef123456',
  worktreePath: '/home/dev/notes-qa.tsa-worktrees/tsa-REQ-1-TDD-NOTES-v1',
  summary: {
    files: [
      { path: 'tests/tsa/x/test_a.py', status: 'generated', cases: ['TC-1'] },
      { path: 'tests/tsa/x/test_b.py', status: 'needs-review', cases: ['TC-2'] }
    ],
    manifest: 'tests/tsa/x/assertion-manifest.json',
    report: 'tests/tsa/x/tsa-report.md'
  }
})

describe('TestScriptOutputSheet', () => {
  afterEach(() => {
    cleanup()
    vi.mocked(callRequestRpc).mockClear()
  })

  it('shows the files and reads the first one whole, marking the assertions tied to the spec', async () => {
    render(<TestScriptOutputSheet requestId="r1" run={run} open onOpenChange={() => {}} />)
    expect(screen.getByTestId('testscript-files').textContent).toContain('test_b.py')
    const code = await screen.findByTestId('testscript-code')
    expect(code.textContent).toContain('def test_one(api_client):')
    const marked = code.querySelector('[data-exp="TC-1.E1"]')
    expect(marked?.getAttribute('data-line')).toBe('6')
    expect(vi.mocked(callRequestRpc)).toHaveBeenCalledWith('request.testScriptFile', {
      id: 'r1',
      path: 'tests/tsa/x/test_a.py',
      offset: 20
    })
  })

  it('lists the coverage and opens a location at its line', async () => {
    render(<TestScriptOutputSheet requestId="r1" run={run} open onOpenChange={() => {}} />)
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'Coverage' }))
    const table = await screen.findByTestId('testscript-coverage')
    expect(screen.getByTestId('testscript-coverage-summary').textContent).toContain('1 of 3')
    expect(screen.getByTestId('testscript-gap-reason').textContent).toContain('TGAP-1')
    expect(within(table).getByText('missing')).toBeTruthy()
    fireEvent.click(within(table).getByRole('button', { name: 'test_a.py:6' }))
    const code = await screen.findByTestId('testscript-code')
    expect(code.querySelector('[data-line="6"]')?.className).toContain('outline')
  })

  it('renders the report as markdown', async () => {
    render(<TestScriptOutputSheet requestId="r1" run={run} open onOpenChange={() => {}} />)
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'Report' }))
    expect(await screen.findByRole('heading', { name: 'TSA report' })).toBeTruthy()
  })

  it('opens on the file it is given', async () => {
    render(
      <TestScriptOutputSheet
        requestId="r1"
        run={run}
        initialFile="tests/tsa/x/test_b.py"
        open
        onOpenChange={() => {}}
      />
    )
    await screen.findByTestId('testscript-code')
    expect(vi.mocked(callRequestRpc)).toHaveBeenCalledWith('request.testScriptFile', {
      id: 'r1',
      path: 'tests/tsa/x/test_b.py',
      offset: 0
    })
  })
})
