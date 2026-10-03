// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpKillSwitchPanel } from './McpKillSwitchPanel'

const call = vi.fn()
const track = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/lib/telemetry', () => ({ track: (...a: unknown[]) => track(...a) }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: { killSwitch: { active: false } },
      refreshMcpServerInfo: () => Promise.resolve()
    }))
  }
})

beforeEach(() => {
  call.mockReset()
  track.mockReset()
  call.mockImplementation((m: string) =>
    Promise.resolve(m === 'mcp.admin.killswitch.list' ? [] : { ok: true })
  )
})
afterEach(cleanup)

describe('kill switch telemetry', () => {
  it('reports scope and state without the free-text reason', async () => {
    render(<McpKillSwitchPanel />)
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'incident SECRET_42' } })
    fireEvent.click(screen.getByRole('button', { name: 'Activate kill switch…' }))
    fireEvent.change(screen.getByLabelText('Type STOP to confirm'), { target: { value: 'STOP' } })
    fireEvent.click(screen.getByRole('button', { name: 'Activate' }))
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1))
    expect(track).toHaveBeenCalledWith('mcp_killswitch_toggled', { scope: 'tenant', active: true })
    expect(JSON.stringify(track.mock.calls)).not.toContain('SECRET_42')
  })

  it('does not report a failed toggle', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.admin.killswitch.list' ? Promise.resolve([]) : Promise.reject(new Error('nope'))
    )
    render(<McpKillSwitchPanel />)
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'incident' } })
    fireEvent.click(screen.getByRole('button', { name: 'Activate kill switch…' }))
    fireEvent.change(screen.getByLabelText('Type STOP to confirm'), { target: { value: 'STOP' } })
    fireEvent.click(screen.getByRole('button', { name: 'Activate' }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.killswitch.set', expect.anything())
    )
    expect(track).not.toHaveBeenCalled()
  })
})
