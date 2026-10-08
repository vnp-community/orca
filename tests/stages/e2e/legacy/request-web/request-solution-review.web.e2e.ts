/**
 * E2E for reviewing a Request Solution (FE-REQ-TASK-020-05), web SPA + mocked gateway WebSocket.
 * Channels follow CONTRACT-request-ui-api 2.2/2.3: solution.choose returns a fresh approvalDigest
 * that approval.approve must send back as expectedDigest.
 */
import { expect, test, type Page } from '@playwright/test'
import {
  approvalView,
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView,
  type FakeRequestBackend
} from './support/mock-request-ws'

const solution = {
  id: 'sol-1',
  requestId: 'req-1',
  kind: 'solution',
  status: 'proposed',
  version: 1,
  createdAt: '2026-10-08T09:00:00Z',
  options: {
    options: [
      {
        id: 'opt-a',
        title: 'Patch the parser',
        summary: 'Small fix',
        pros: ['Fast'],
        cons: ['Debt'],
        estimatedEffort: '2h'
      },
      {
        id: 'opt-b',
        title: 'Rewrite the parser',
        summary: 'Clean design',
        pros: ['Clean'],
        cons: ['Slow'],
        estimatedEffort: '3d'
      }
    ],
    recommendation: { optionId: 'opt-a' }
  }
}

function solutionBackend(type = 'change_request'): FakeRequestBackend {
  let status = 'awaiting_analysis_approval'
  const backend = createFakeRequestBackend({
    'request.list': () => ({ requests: [requestView({ type, status })] }),
    'request.get': () => ({ request: requestView({ type, status }) }),
    'solution.list': () => ({ solutions: [solution], runs: [] }),
    'approval.list': () => ({ approvals: status === 'planning' ? [] : [approvalView()] }),
    'approval.listPending': () => ({ approvals: status === 'planning' ? [] : [approvalView()] }),
    'solution.choose': (p) => ({
      solution: { ...solution, status: 'proposed', chosenOptionId: p.optionId },
      approvalDigest: 'digest-2'
    }),
    'approval.approve': () => {
      status = 'planning'
      return {
        approval: approvalView({ status: 'approved', version: 2 }),
        requestStatus: 'planning'
      }
    },
    'approval.reject': (p) => ({
      approval: approvalView({ status: 'rejected', comment: p.comment }),
      requestStatus: 'analyzing'
    })
  })
  return backend
}

async function openAnalysis(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-1' })
  await page.getByRole('tab', { name: 'Analysis' }).click()
}

test.describe('Request solution review (mocked request-service)', () => {
  test('a: change_request with two options shows cards and a comparison table', async ({
    page
  }) => {
    await openAnalysis(page, solutionBackend())
    const panel = page.getByRole('tabpanel', { name: 'Analysis' })
    const options = panel.getByRole('radiogroup', { name: 'Options' }).getByRole('radio')
    await expect(options).toHaveCount(2)
    await expect(options.first()).toContainText('Recommended')
    await panel.getByRole('button', { name: 'Compare' }).click()
    const table = page.getByRole('table')
    await expect(table).toBeVisible()
    await expect(table).toContainText('Patch the parser')
    await expect(table).toContainText('Rewrite the parser')
  })

  test('b: choose then approve calls solution.choose before approval.approve and moves to planning', async ({
    page
  }) => {
    const backend = solutionBackend()
    await openAnalysis(page, backend)
    const panel = page.getByRole('tabpanel', { name: 'Analysis' })
    const approve = panel.getByRole('button', { name: 'Approve this option' })
    await expect(approve).toBeDisabled()
    await panel.getByRole('radio', { name: /Rewrite the parser/ }).click()
    // Picking a non-recommended option needs a rationale (CR-REQ-028) before approve unlocks.
    await expect(approve).toBeDisabled()
    await panel
      .getByRole('textbox', { name: 'Reason for choosing' })
      .fill('Long-term maintenance matters more')
    await approve.click()
    await expect.poll(() => backend.callsTo('approval.approve').length).toBe(1)
    const order = backend.calls
      .map((c) => c.method)
      .filter((m) => m === 'solution.choose' || m === 'approval.approve')
    expect(order).toEqual(['solution.choose', 'approval.approve'])
    expect(backend.callsTo('solution.choose')[0]).toMatchObject({
      requestId: 'req-1',
      solutionId: 'sol-1',
      optionId: 'opt-b'
    })
    expect(backend.callsTo('approval.approve')[0]).toMatchObject({
      id: 'ap-1',
      expectedVersion: 1,
      expectedDigest: 'digest-2'
    })
    await expect(page.getByRole('banner').getByText('Planning')).toBeVisible()
  })

  test('c: reject is blocked for a short reason and sends approval.reject with a valid comment', async ({
    page
  }) => {
    const backend = solutionBackend()
    await openAnalysis(page, backend)
    await page
      .getByRole('tabpanel', { name: 'Analysis' })
      .getByRole('button', { name: 'Reject' })
      .click()
    const dialog = page.getByRole('dialog')
    const submit = dialog.getByRole('button', { name: 'Reject' })
    await dialog.getByRole('textbox').fill('too short')
    await expect(submit).toBeDisabled()
    await dialog.getByRole('textbox').fill('The rewrite option ignores the migration cost')
    await expect(submit).toBeEnabled()
    await submit.click()
    await expect.poll(() => backend.callsTo('approval.reject').length).toBe(1)
    expect(backend.callsTo('approval.reject')[0]).toMatchObject({
      id: 'ap-1',
      expectedVersion: 1,
      expectedDigest: 'digest-1',
      comment: 'The rewrite option ignores the migration cost'
    })
  })

  test('d: Mod+Enter submits the reject dialog (Meta on Mac, Control elsewhere)', async ({
    page
  }) => {
    const backend = solutionBackend()
    await openAnalysis(page, backend)
    await page
      .getByRole('tabpanel', { name: 'Analysis' })
      .getByRole('button', { name: 'Reject' })
      .click()
    const box = page.getByRole('dialog').getByRole('textbox')
    await box.fill('Needs a cheaper option first')
    // Plain Enter only adds a line; it must not submit.
    await box.press('Enter')
    expect(backend.callsTo('approval.reject')).toHaveLength(0)
    // Playwright maps ControlOrMeta to Meta on macOS and Control on Linux/Windows.
    await box.press('ControlOrMeta+Enter')
    await expect.poll(() => backend.callsTo('approval.reject').length).toBe(1)
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('e: hotfix shows no decision bar', async ({ page }) => {
    const backend = solutionBackend('hotfix')
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-1' })
    await expect(page.getByRole('heading', { level: 2, name: 'Request' })).toBeVisible()
    const analysisTab = page.getByRole('tab', { name: 'Analysis' })
    if (await analysisTab.count()) {
      await analysisTab.click()
    }
    await expect(page.getByRole('button', { name: /Approve/ })).toHaveCount(0)
    await expect(
      page.getByRole('tabpanel', { name: 'Requests' }).getByRole('button', { name: 'Reject' })
    ).toHaveCount(0)
  })
})
