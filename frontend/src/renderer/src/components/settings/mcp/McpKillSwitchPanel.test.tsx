// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpKillSwitchPanel } from './McpKillSwitchPanel'

const call = vi.fn()
const refresh = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: { killSwitch: { active: false } },
      refreshMcpServerInfo: (...a: unknown[]) => refresh(...a)
    }))
  }
})

beforeEach(() => {
  call.mockReset()
  refresh.mockReset()
  call.mockImplementation((m: string) =>
    Promise.resolve(m === 'mcp.admin.killswitch.list' ? [] : { ok: true })
  )
})
afterEach(cleanup)

const activate = (): HTMLButtonElement =>
  screen.getByRole('button', { name: 'Activate kill switch…' }) as HTMLButtonElement

describe('McpKillSwitchPanel', () => {
  it('requires a reason of at least 3 characters', async () => {
    render(<McpKillSwitchPanel />)
    expect(activate().disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'ab' } })
    expect(activate().disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'abc' } })
    expect(activate().disabled).toBe(false)
    expect(activate().getAttribute('data-variant')).toBe('destructive')
  })

  it('Activate stays disabled until STOP is typed, then sends the contract payload', async () => {
    render(<McpKillSwitchPanel />)
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'incident 42' } })
    fireEvent.click(activate())
    const confirmBtn = screen.getByRole('button', { name: 'Activate' }) as HTMLButtonElement
    expect(confirmBtn.getAttribute('data-variant')).toBe('destructive')
    expect(screen.getByRole('button', { name: 'Cancel' }).getAttribute('data-variant')).toBe(
      'ghost'
    )
    expect(confirmBtn.disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Type STOP to confirm'), { target: { value: 'stop' } })
    expect(confirmBtn.disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Type STOP to confirm'), { target: { value: 'STOP' } })
    fireEvent.click(confirmBtn)
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.killswitch.set', {
        scope: 'tenant',
        active: true,
        reason: 'incident 42'
      })
    )
    await waitFor(() => expect(refresh).toHaveBeenCalled())
  })

  it('lists active switches and resumes with a default-variant button', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.admin.killswitch.list'
          ? [
              {
                scope: 'client',
                targetId: 'c1',
                reason: 'bad app',
                at: '2026-10-01T00:00:00Z',
                by: 'a'
              }
            ]
          : { ok: true }
      )
    )
    render(<McpKillSwitchPanel />)
    const resume = await screen.findByRole('button', { name: 'Resume access' })
    expect(resume.getAttribute('data-variant')).toBe('default')
    fireEvent.click(resume)
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.killswitch.set', {
        scope: 'client',
        targetId: 'c1',
        active: false,
        reason: 'Resumed by administrator'
      })
    )
  })
})
