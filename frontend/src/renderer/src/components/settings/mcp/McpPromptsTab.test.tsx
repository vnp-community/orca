// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpPromptsTab } from './McpPromptsTab'

const call = vi.fn()
const confirm = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const prompt = (id: string, name: string, builtin = false) => ({
  id,
  name,
  description: `${name} desc`,
  version: 1,
  updatedAt: '2026-10-01T00:00:00Z',
  arguments: [{ name: 'a', description: '', required: true }],
  template: '{{a}}',
  builtin
})

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
})
afterEach(cleanup)

describe('McpPromptsTab', () => {
  it('lists built-ins first, without Edit/Delete, and hints that no custom prompts exist', async () => {
    call.mockResolvedValue([prompt('c', 'zeta_custom'), prompt('b', 'plan_task', true)])
    render(<McpPromptsTab />)
    await screen.findByText('plan_task')
    const rows = screen.getAllByRole('row').slice(1)
    expect(within(rows[0]).getByText('plan_task')).toBeTruthy()
    expect(within(rows[0]).queryByRole('button', { name: /Edit prompt|Delete prompt/ })).toBeNull()
    expect(within(rows[0]).getByRole('button', { name: 'View prompt plan_task' })).toBeTruthy()
    expect(within(rows[1]).getByRole('button', { name: 'Delete prompt zeta_custom' })).toBeTruthy()
    expect(call).toHaveBeenCalledWith('mcp.admin.prompt.list')
  })

  it('shows the empty-custom hint when only built-ins exist', async () => {
    call.mockResolvedValue([prompt('b', 'plan_task', true)])
    render(<McpPromptsTab />)
    await screen.findByText(/No custom prompts yet/)
  })

  it('deletes only after a destructive confirmation and removes the row', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(m === 'mcp.admin.prompt.delete' ? { ok: true } : [prompt('c', 'mine_one')])
    )
    confirm.mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    render(<McpPromptsTab />)
    const del = await screen.findByRole('button', { name: 'Delete prompt mine_one' })
    fireEvent.click(del)
    await waitFor(() => expect(confirm).toHaveBeenCalledTimes(1))
    expect(call).not.toHaveBeenCalledWith('mcp.admin.prompt.delete', expect.anything())
    expect(confirm.mock.calls[0][0].confirmVariant).toBe('destructive')
    fireEvent.click(del)
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.prompt.delete', { promptId: 'c' })
    )
    await waitFor(() => expect(screen.queryByText('mine_one')).toBeNull())
  })

  it('disables New prompt at the 50 custom limit', async () => {
    call.mockResolvedValue(Array.from({ length: 50 }, (_, i) => prompt(`c${i}`, `custom_${i}`)))
    render(<McpPromptsTab />)
    const btn = (await screen.findByRole('button', { name: 'New prompt' })) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
  })

  it('opens the editor from New prompt and filters', async () => {
    call.mockResolvedValue([prompt('b', 'plan_task', true), prompt('c', 'other_one')])
    render(<McpPromptsTab />)
    fireEvent.change(await screen.findByLabelText('Filter prompts'), { target: { value: 'zzz' } })
    await screen.findByText('No prompts match.')
    fireEvent.click(screen.getByRole('button', { name: 'New prompt' }))
    await screen.findByRole('dialog')
  })

  it('error, disabled and forbidden states', async () => {
    call.mockRejectedValueOnce(new McpRpcError('MCP_INTERNAL', 'kaput'))
    const a = render(<McpPromptsTab />)
    await screen.findByText('kaput')
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy()
    a.unmount()
    call.mockRejectedValueOnce(new McpRpcError('MCP_DISABLED', 'off'))
    const b = render(<McpPromptsTab />)
    await screen.findByText('MCP is turned off for this organization.')
    b.unmount()
    call.mockRejectedValueOnce(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    render(<McpPromptsTab />)
    await screen.findByText('You need admin rights to manage prompts.')
  })
})
