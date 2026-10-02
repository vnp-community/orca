// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type * as CsvModule from './mcp-audit-csv'
import { McpAuditTab } from './McpAuditTab'

const call = vi.fn()
const download = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('./mcp-audit-csv', async (orig) => ({
  ...(await orig<typeof CsvModule>()),
  downloadMcpAuditCsv: (...a: unknown[]) => download(...a)
}))

const row = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  at: '2026-10-01T00:00:00Z',
  actorType: 'agent',
  userId: '12345678-aaaa-bbbb-cccc-123456789012',
  clientName: 'Claude',
  sessionId: 's',
  tool: `tool_${id}`,
  risk: 'exec',
  decision: 'approved',
  argsSummary: '<script>alert(1)</script>',
  result: 'ok',
  durationMs: 12,
  ...over
})

beforeEach(() => {
  call.mockReset()
  download.mockReset()
  call.mockImplementation((m: string) =>
    Promise.resolve(m === 'mcp.admin.tool.list' ? [] : { entries: [row('1'), row('2')] })
  )
})
afterEach(cleanup)

describe('McpAuditTab', () => {
  it('lists rows with the agent badge and opens a plain-text detail', async () => {
    render(<McpAuditTab />)
    await screen.findByText('tool_1')
    expect(screen.getAllByText('Agent').length).toBeGreaterThan(0)
    fireEvent.click(screen.getAllByRole('button', { name: /Open details for tool_1/ })[0])
    const pre = await screen.findByText('<script>alert(1)</script>')
    expect(pre.tagName).toBe('PRE')
    expect(document.querySelector('script')).toBeNull()
  })

  it('exports only the loaded rows', async () => {
    render(<McpAuditTab />)
    await screen.findByText('tool_1')
    fireEvent.click(screen.getByRole('button', { name: 'Export loaded rows (CSV)' }))
    expect(download).toHaveBeenCalledTimes(1)
    expect((download.mock.calls[0][0] as string).split('\r\n').length).toBe(4)
  })

  it('shows the empty states and queries with the debounced tool filter', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.admin.tool.list' ? Promise.resolve([]) : Promise.resolve({ entries: [] })
    )
    render(<McpAuditTab />)
    await screen.findByText(/No agent activity yet/)
    fireEvent.change(screen.getByLabelText('Tool'), { target: { value: 'zzz' } })
    await waitFor(() => screen.getByText('No entries match these filters.'), { timeout: 2000 })
    expect(call).toHaveBeenCalledWith('mcp.admin.audit.query', { tool: 'zzz', limit: 50 })
  })
})
