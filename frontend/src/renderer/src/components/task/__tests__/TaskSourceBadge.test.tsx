// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

const callRuntimeRpc = vi.fn()
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (...args: unknown[]) => callRuntimeRpc(...args),
  getActiveRuntimeTarget: () => ({ kind: 'remote' })
}))
vi.mock('../../../store', () => ({
  useAppStore: { getState: () => ({ settings: {} }) }
}))

import { TaskSourceBadge } from '../TaskSourceBadge'

describe('TaskSourceBadge', () => {
  beforeEach(() => callRuntimeRpc.mockReset())

  it('shows the Jira key as a link to the issue', async () => {
    callRuntimeRpc.mockResolvedValue({
      provider: 'jira',
      ref: 'ENG-1',
      url: 'https://x.atlassian.net/browse/ENG-1'
    })
    render(<TaskSourceBadge taskId="t1" />)
    const badge = await screen.findByTestId('task-source-badge')
    expect(badge.textContent).toBe('Jira ENG-1')
    expect(badge.getAttribute('href')).toBe('https://x.atlassian.net/browse/ENG-1')
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'task.getSource', {
      taskId: 't1'
    })
  })

  it('renders plain text when the url is not http(s)', async () => {
    callRuntimeRpc.mockResolvedValue({ provider: 'jira', ref: 'ENG-2', url: 'javascript:alert(1)' })
    render(<TaskSourceBadge taskId="t1" />)
    const badge = await screen.findByTestId('task-source-badge')
    expect(badge.getAttribute('href')).toBeNull()
  })

  it('renders nothing for a task without a source or when the call fails', async () => {
    callRuntimeRpc.mockResolvedValueOnce(null)
    const { container, rerender } = render(<TaskSourceBadge taskId="t1" />)
    await waitFor(() => expect(callRuntimeRpc).toHaveBeenCalled())
    expect(container.firstChild).toBeNull()

    callRuntimeRpc.mockRejectedValueOnce(new Error('unsupported'))
    rerender(<TaskSourceBadge taskId="t2" />)
    await waitFor(() => expect(callRuntimeRpc).toHaveBeenCalledTimes(2))
    expect(container.firstChild).toBeNull()
  })
})
