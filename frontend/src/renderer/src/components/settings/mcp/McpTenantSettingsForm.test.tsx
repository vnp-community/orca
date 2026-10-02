// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpTenantSettingsForm } from './McpTenantSettingsForm'
import { McpAdminEnableCard } from './McpAdminEnableCard'

const call = vi.fn()
const confirm = vi.fn()
const refresh = vi.fn().mockResolvedValue(undefined)
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({ refreshMcpServerInfo: (...a: unknown[]) => refresh(...a) }))
  }
})

const settings = {
  enabled: true,
  dcrEnabled: false,
  maxTokenDays: 30,
  approvalTtlSeconds: 120,
  killSwitch: { active: false }
}
beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
  call.mockImplementation((m: string, p: object) =>
    Promise.resolve(m === 'mcp.admin.settings.get' ? settings : { ...settings, ...p })
  )
})
afterEach(cleanup)

describe('McpTenantSettingsForm', () => {
  it('sends only changed fields', async () => {
    render(<McpTenantSettingsForm />)
    const days = (await screen.findByLabelText('Max token lifetime (days)')) as HTMLInputElement
    fireEvent.change(days, { target: { value: '45' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save settings' }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.settings.set', { maxTokenDays: 45 })
    )
  })

  it('blocks out-of-range values', async () => {
    render(<McpTenantSettingsForm />)
    fireEvent.change(await screen.findByLabelText('Max token lifetime (days)'), {
      target: { value: '91' }
    })
    fireEvent.change(screen.getByLabelText('Approval expiry (seconds)'), {
      target: { value: '10' }
    })
    expect(
      (screen.getByRole('button', { name: 'Save settings' }) as HTMLButtonElement).disabled
    ).toBe(true)
    expect(screen.getAllByText(/Enter a whole number/)).toHaveLength(2)
  })

  it('asks for confirmation before turning MCP off', async () => {
    confirm.mockResolvedValue(false)
    render(<McpTenantSettingsForm />)
    fireEvent.click(await screen.findByLabelText('Enable MCP for this organization'))
    fireEvent.click(screen.getByRole('button', { name: 'Save settings' }))
    await waitFor(() => expect(confirm).toHaveBeenCalled())
    expect(confirm.mock.calls[0][0].confirmVariant).toBe('default')
    expect(call).not.toHaveBeenCalledWith('mcp.admin.settings.set', expect.anything())
  })

  it('shows forbidden for non-admins', async () => {
    const { McpRpcError } = await import('@/runtime/runtime-mcp-error')
    call.mockRejectedValue(new McpRpcError('MCP_NOT_ADMIN', 'x'))
    render(<McpTenantSettingsForm />)
    await screen.findByText('Administrator access required.')
  })
})

describe('McpAdminEnableCard', () => {
  it('turns MCP on and refreshes availability', async () => {
    render(<McpAdminEnableCard />)
    fireEvent.click(screen.getByRole('button', { name: 'Turn on MCP' }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.settings.set', { enabled: true })
    )
    await waitFor(() => expect(refresh).toHaveBeenCalled())
  })
})
