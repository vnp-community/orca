import { expect, test } from '@playwright/test'
import { createFakeMcpBackend, mockOrcaApp, mockUser } from './support/mock-orca-ws'
import { SPA_URL, bootApp, simulatePushClick } from './support/mcp-app-navigation'
import { devStackUrl, DEV_STACK_SKIP_REASON } from './support/mcp-dev-backend'

const approval = (over: Record<string, unknown> = {}) => ({
  id: 'ap-1',
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 300_000).toISOString(),
  status: 'pending' as const,
  tool: { name: 'exec', title: 'Run command', risk: 'exec' as const },
  clientName: 'Claude Code',
  sessionId: 'session-abcdef',
  argsPreview: { text: 'rm -rf ./build && make', redacted: false },
  paramsHash: 'hash-1',
  ...over
})

test.describe('approvals (mocked WebSocket backend)', () => {
  test.skip(!!devStackUrl, 'mocked variant; needs a seeded stack with MCP_E2E_BASE_URL otherwise')

  test('approve: dialog appears from the event stream and releases the parked tools/call', async ({
    page
  }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
    await bootApp(page, backend)

    const call = backend.requestToolCall(approval())
    await expect(page.getByText('An AI agent is asking permission')).toBeVisible()
    await expect(page.getByLabel('Exact tool arguments')).toHaveText('rm -rf ./build && make')
    // Approve unlocks only after the risk delay, and only a real click can use it.
    await page.getByRole('button', { name: /^Approve / }).click({ timeout: 15_000 })
    await expect(page.getByText('An AI agent is asking permission')).toHaveCount(0)
    await expect(call.outcome).resolves.toEqual({ isError: false })
    expect(backend.calls.find((c) => c.method === 'mcp.approval.decide')?.params).toEqual({
      approvalId: 'ap-1',
      decision: 'approve',
      paramsHash: 'hash-1'
    })
  })

  test('deny: the agent call fails with isError', async ({ page }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
    await bootApp(page, backend)
    const call = backend.requestToolCall(approval())
    await page.getByRole('button', { name: /^Deny / }).click()
    await expect(call.outcome).resolves.toEqual({ isError: true })
  })

  test('a changed request (hash mismatch) is flagged and needs a fresh approval', async ({
    page
  }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
    await bootApp(page, backend)
    backend.requestToolCall(approval())
    await expect(page.getByLabel('Exact tool arguments')).toBeVisible()
    backend.mutateApprovalHash('ap-1', 'hash-2')
    await page.getByRole('button', { name: /^Approve / }).click({ timeout: 15_000 })
    await expect(page.getByText('This request changed. Review it again.')).toBeVisible()
  })

  test('push-click deep link opens Settings > MCP > Approvals', async ({ page }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
    await bootApp(page, backend)
    // Decide later: the inbox row must still be reachable from the notification.
    backend.requestToolCall(approval())
    await page.getByRole('button', { name: 'Decide later' }).click()
    await simulatePushClick(page, '/?section=mcp&tab=approvals&approval=ap-1')
    // The prompt is modal (hides the tablist from the a11y tree), so check the active tab by DOM state.
    await expect(page.getByText('An AI agent is asking permission')).toBeVisible()
    await expect(
      page.locator('[role="tab"][data-state="active"]', { hasText: 'Approvals' })
    ).toBeAttached()
  })

  test('cold-start deep link opens Settings > MCP > Approvals focused on that approval', async ({
    page
  }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    backend.requestToolCall(approval({ id: 'ap-cold' }))
    // Why: a cold start races startup UI hydration, which only completes with the boot channels.
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL, bootChannels: true })
    await page.goto('/web-index.html?section=mcp&tab=approvals&approval=ap-cold')
    // The global prompt is modal; deciding later exposes the inbox row that was focused.
    await page.getByRole('button', { name: 'Decide later' }).click()
    // Boot stubs close onboarding, so the first-run feature tip may sit on top; dismiss if shown.
    await page
      .getByRole('button', { name: 'Got it' })
      .click({ timeout: 3000 })
      .catch(() => {})
    await expect(
      page.locator('[role="tab"][data-state="active"]', { hasText: 'Approvals' })
    ).toBeVisible()
    await expect(page.getByRole('listitem').filter({ hasText: 'Run command' })).toBeVisible()
    await expect.poll(() => new URL(page.url()).search).toBe('')
  })
})

test.describe('@dev-stack approvals', () => {
  test.skip(
    true,
    `${DEV_STACK_SKIP_REASON}; also needs backend-go tests/mcpconformance seed-dev (BE-MCP-SOL-015), not available here`
  )
  test('placeholder: real tools/call -> dialog -> approve', async () => {})
})
