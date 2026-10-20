// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

import { useAppStore } from '../../../store'
import { ProjectRequestExecutionSection } from '../ProjectRequestExecutionSection'

const stored = {
  projectId: 'p1',
  engine: 'native',
  openspecMinVersion: '',
  solutionAgent: false,
  executionMode: 'step_by_step',
  version: 3
}

function asRole(role: 'admin' | 'member' | null): void {
  useAppStore.setState({
    currentUser: role ? ({ id: 'u1', role } as never) : null
  })
}

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: stored })
  Object.values(toast).forEach((fn) => fn.mockReset())
  asRole('admin')
})
afterEach(cleanup)

describe('ProjectRequestExecutionSection', () => {
  it('shows the loading state, then the current mode', async () => {
    render(<ProjectRequestExecutionSection projectId="p1" />)
    expect(screen.getByTestId('project-execution-loading')).toBeInTheDocument()
    const group = await screen.findByRole('radiogroup', { name: 'Request execution' })
    expect(group).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Ask before each step' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(
      screen.getByText(
        'Applies to requests that start running from now on. Requests already running keep their mode.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByTestId('project-execution-first-save')).toBeNull()
  })

  it('lets an administrator switch and save, keeping the other settings as read', async () => {
    callRequestRpc.mockResolvedValueOnce({
      ok: true,
      value: { ...stored, engine: 'openspec', openspecMinVersion: '1.2.3', solutionAgent: true }
    })
    callRequestRpc.mockResolvedValueOnce({
      ok: true,
      value: { ...stored, engine: 'openspec', executionMode: 'continuous', version: 4 }
    })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    const save = await screen.findByTestId('project-execution-save')
    expect(save).toBeDisabled()
    fireEvent.click(screen.getByRole('radio', { name: 'Run all steps after one approval' }))
    expect(screen.getByTestId('project-execution-access-note')).toHaveTextContent(
      'with the access of the person who approves the plan'
    )
    expect(save).toBeEnabled()
    fireEvent.click(save)
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Request execution updated'))
    expect(callRequestRpc).toHaveBeenLastCalledWith('request.engineSettingsSet', {
      projectId: 'p1',
      engine: 'openspec',
      openspecMinVersion: '1.2.3',
      solutionAgent: true,
      executionMode: 'continuous',
      expectedVersion: 3
    })
    expect(screen.getByTestId('project-execution-save')).toBeDisabled()
  })

  it('tells the administrator the first save also stores the engine settings', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { ...stored, version: 0 } })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    expect(await screen.findByTestId('project-execution-first-save')).toHaveTextContent(
      "Saving also stores this project's current engine settings."
    )
  })

  it('shows anyone else the value read-only with the reason', async () => {
    asRole('member')
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { ...stored, executionMode: 'continuous' }
    })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    expect(await screen.findByTestId('project-execution-read-only')).toHaveTextContent(
      'Only an administrator can change this.'
    )
    const chosen = screen.getByRole('radio', { name: 'Run all steps after one approval' })
    expect(chosen).toHaveAttribute('aria-checked', 'true')
    expect(chosen).toBeDisabled()
    expect(screen.queryByTestId('project-execution-save')).toBeNull()
  })

  it('turns read-only when the backend refuses an administrator the client believed in', async () => {
    callRequestRpc.mockResolvedValueOnce({ ok: true, value: stored }).mockResolvedValueOnce({
      ok: false,
      error: { kind: 'forbidden', code: 'runtime_error', message: 'REQUEST_FORBIDDEN: admin only' }
    })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    fireEvent.click(await screen.findByRole('radio', { name: 'Run all steps after one approval' }))
    fireEvent.click(screen.getByTestId('project-execution-save'))
    expect(await screen.findByTestId('project-execution-read-only')).toBeInTheDocument()
    expect(toast.error).toHaveBeenCalledWith('Only an administrator can change this.')
  })

  it('says so and loads the latest values on a version conflict', async () => {
    callRequestRpc
      .mockResolvedValueOnce({ ok: true, value: stored })
      .mockResolvedValueOnce({
        ok: false,
        error: {
          kind: 'invalid_state',
          code: 'runtime_error',
          message: 'REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT: stale',
          backendCode: 'REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT'
        }
      })
      .mockResolvedValueOnce({
        ok: true,
        value: { ...stored, executionMode: 'continuous', version: 4 }
      })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    fireEvent.click(await screen.findByRole('radio', { name: 'Run all steps after one approval' }))
    fireEvent.click(screen.getByTestId('project-execution-save'))
    await waitFor(() =>
      expect(toast.warning).toHaveBeenCalledWith(
        'Someone else changed these settings. The latest values were loaded.'
      )
    )
    await waitFor(() =>
      expect(
        screen.getByRole('radio', { name: 'Run all steps after one approval' })
      ).toHaveAttribute('aria-checked', 'true')
    )
  })

  it('shows an error with Retry when the read fails', async () => {
    callRequestRpc
      .mockResolvedValueOnce({ ok: false, error: { kind: 'network', code: 'x', message: 'down' } })
      .mockResolvedValueOnce({ ok: true, value: stored })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    expect(await screen.findByRole('alert')).toHaveTextContent(
      "Couldn't load the execution settings."
    )
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByRole('radiogroup')).toBeInTheDocument()
  })

  it('renders nothing for a gateway that has no such channel', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'unsupported', code: 'method_not_found', message: 'm' }
    })
    const { container } = render(<ProjectRequestExecutionSection projectId="p1" />)
    await waitFor(() => expect(container).toBeEmptyDOMElement())
  })

  it('explains a missing read permission without offering Retry', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'forbidden', code: 'runtime_error', message: 'REQUEST_FORBIDDEN: no' }
    })
    render(<ProjectRequestExecutionSection projectId="p1" />)
    expect(await screen.findByRole('alert')).toHaveTextContent("don't have permission")
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  })
})
