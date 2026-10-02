// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpPolicyEditorDialog } from './McpPolicyEditorDialog'
import { McpPoliciesTab } from './McpPoliciesTab'

const call = vi.fn()
const confirm = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: { killSwitch: { active: false } },
      refreshMcpServerInfo: vi.fn()
    }))
  }
})
vi.mock('./McpTenantSettingsForm', () => ({ McpTenantSettingsForm: () => null }))
vi.mock('./McpKillSwitchPanel', () => ({ McpKillSwitchPanel: () => null }))

const policy = (over: Record<string, unknown> = {}) => ({
  id: 'p1',
  version: 1,
  updatedAt: '2026-10-01T00:00:00Z',
  updatedBy: 'admin',
  match: { tool: 'terminal_send' },
  decision: 'require_approval',
  ...over
})
const tools = [
  { name: 'terminal_send', title: 'Send', namespace: 'terminal', risk: 'exec', hardDenied: false },
  {
    name: 'credentials_set',
    title: 'Creds',
    namespace: 'credentials',
    risk: 'admin',
    hardDenied: true
  }
]

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
})
afterEach(cleanup)

describe('McpPoliciesTab', () => {
  it('lists policies and deletes only after a destructive confirmation', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.admin.policy.list'
          ? [policy()]
          : m === 'mcp.admin.policy.delete'
            ? { ok: true }
            : []
      )
    )
    confirm.mockResolvedValue(true)
    render(<McpPoliciesTab />)
    fireEvent.click(await screen.findByRole('button', { name: /Delete rule/ }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.policy.delete', { policyId: 'p1' })
    )
    expect(confirm.mock.calls[0][0].confirmVariant).toBe('destructive')
  })

  it('shows the empty state and forbidden state', async () => {
    call.mockImplementation((m: string) => Promise.resolve(m === 'mcp.admin.policy.list' ? [] : []))
    render(<McpPoliciesTab />)
    await screen.findByText(/No custom policies/)
    cleanup()
    call.mockRejectedValue(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    render(<McpPoliciesTab />)
    await screen.findByText('Administrator access required.')
  })
})

describe('McpPolicyEditorDialog', () => {
  const setup = (p = policy()) => {
    const onSaved = vi.fn()
    render(
      <McpPolicyEditorDialog
        policy={p as never}
        tools={tools as never}
        clients={[]}
        onClose={vi.fn()}
        onSaved={onSaved}
        onGone={vi.fn()}
      />
    )
    return onSaved
  }

  it('blocks Save for allow on a hard-denied tool', () => {
    setup(policy({ match: { tool: 'credentials_set' }, decision: 'deny' }))
    fireEvent.click(screen.getByRole('radio', { name: 'Allow' }))
    expect((screen.getByRole('button', { name: 'Save rule' }) as HTMLButtonElement).disabled).toBe(
      true
    )
    expect(screen.getByText(/Permanently denied tools cannot be allowed/)).toBeTruthy()
  })

  it('sends id+version on edit and omits empty dimensions', async () => {
    call.mockResolvedValue(policy({ version: 2 }))
    const onSaved = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Save rule' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
    expect(call).toHaveBeenCalledWith('mcp.admin.policy.upsert', {
      id: 'p1',
      version: 1,
      match: { tool: 'terminal_send' },
      decision: 'require_approval'
    })
  })

  it('version conflict: no silent overwrite; Reload latest keeps the draft; Overwrite uses the latest version', async () => {
    const latest = policy({ version: 5, decision: 'deny' })
    call.mockImplementation((m: string) =>
      m === 'mcp.admin.policy.upsert'
        ? Promise.reject(new McpRpcError('MCP_POLICY_VERSION_CONFLICT', 'conflict'))
        : Promise.resolve([latest])
    )
    setup()
    fireEvent.click(screen.getByRole('button', { name: 'Save rule' }))
    await screen.findByText('This rule was changed by someone else.')
    expect(call.mock.calls.filter((c) => c[0] === 'mcp.admin.policy.upsert')).toHaveLength(1)
    expect((screen.getByRole('button', { name: 'Save rule' }) as HTMLButtonElement).disabled).toBe(
      true
    )

    fireEvent.click(screen.getByRole('button', { name: 'Reload latest' }))
    await screen.findByText('Your draft')
    expect(screen.getByRole('radio', { name: 'Deny' }).getAttribute('data-state')).toBe('on')

    call.mockClear()
    call.mockImplementation((m: string) =>
      m === 'mcp.admin.policy.upsert'
        ? Promise.reject(new McpRpcError('MCP_POLICY_VERSION_CONFLICT', 'conflict'))
        : Promise.resolve([policy({ version: 6 })])
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save rule' }))
    await screen.findByText('This rule was changed by someone else.')
    call.mockResolvedValue(policy({ version: 7 }))
    fireEvent.click(screen.getByRole('button', { name: 'Overwrite with my changes' }))
    await waitFor(() =>
      expect(call).toHaveBeenLastCalledWith(
        'mcp.admin.policy.upsert',
        expect.objectContaining({ id: 'p1', version: 6 })
      )
    )
  })

  it('shows a server MCP_POLICY_HARD_DENY inline', async () => {
    call.mockRejectedValue(new McpRpcError('MCP_POLICY_HARD_DENY', 'tool is hard denied'))
    setup()
    fireEvent.click(screen.getByRole('button', { name: 'Save rule' }))
    await screen.findByText('tool is hard denied')
  })
})
