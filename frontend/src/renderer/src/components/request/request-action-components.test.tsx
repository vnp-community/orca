// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const actions = {
  classify: vi.fn(), confirmType: vi.fn(), changeType: vi.fn(), returnToBacklog: vi.fn(),
  reopen: vi.fn(), cancel: vi.fn(), spawnChild: vi.fn(), generatePlan: vi.fn(), startPhase: vi.fn()
}
const platform = vi.fn(() => 'linux')
const callRequestRpc = vi.fn()
vi.mock('../../hooks/useRequestActions', () => ({ useRequestActions: () => actions }))
vi.mock('../../lib/shortcut-platform', () => ({ getShortcutPlatform: () => platform() }))
vi.mock('../../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))
vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => [{ id: 'proj-1', name: 'Alpha' }]),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))

import { useAppStore } from '@/store'
import { CancelRequestDialog } from './CancelRequestDialog'
import { ChangeTypeConfirmDialog } from './ChangeTypeConfirmDialog'
import { CreateRequestDialog } from './CreateRequestDialog'
import { RequestHistoryTab } from './RequestHistoryTab'
import { RequestRelatedTab } from './RequestRelatedTab'
import { SpawnChildRequestDialog } from './SpawnChildRequestDialog'
import { TypeConfirmationCard } from './TypeConfirmationCard'
import type { OrcaRequest } from '../../../../shared/request-types'

const req = (over: Partial<OrcaRequest> = {}): OrcaRequest => ({
  id: 'r1', projectId: 'p', number: 3, title: 'Parent', type: 'bug', status: 'awaiting_type_confirmation',
  createdAt: '2026-01-01T00:00:00Z', updatedAt: new Date().toISOString(), ...over
})

beforeEach(() => {
  Object.values(actions).forEach((f) => f.mockReset().mockResolvedValue({ ok: true, value: {} }))
  callRequestRpc.mockReset()
  platform.mockReturnValue('linux')
  useAppStore.setState({ requestsById: {} })
})
afterEach(cleanup)

describe('TypeConfirmationCard', () => {
  it('confirms the proposed type with size/urgency and no typeSource', async () => {
    const onChanged = vi.fn()
    render(<TypeConfirmationCard request={req({ confidence: 0.9, size: 'M', urgency: 'urgent', classificationReason: 'because' })} onChanged={onChanged} />)
    expect(screen.getByText('90%')).toBeInTheDocument()
    expect(screen.getByText('because')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Confirm type'))
    await waitFor(() => expect(actions.confirmType).toHaveBeenCalledWith({ id: 'r1', type: 'bug', size: 'M', urgency: 'urgent' }))
    expect(onChanged).toHaveBeenCalled()
  })

  it('blocks confirmation when the AI proposed no type', () => {
    render(<TypeConfirmationCard request={req({ type: 'unknown' })} onChanged={vi.fn()} />)
    expect(screen.getByText('Confirm type')).toBeDisabled()
    expect(screen.getByText('Pick a type to continue.')).toBeInTheDocument()
  })

  it('locks reclassify after the classification limit', async () => {
    actions.classify.mockResolvedValue({ ok: false, error: { kind: 'rate_limited', code: 'REQUEST_CLASSIFICATION_LIMIT', message: '' } })
    render(<TypeConfirmationCard request={req()} onChanged={vi.fn()} />)
    fireEvent.click(screen.getByText('Reclassify'))
    await waitFor(() => expect(screen.getByText('Reclassify')).toBeDisabled())
    expect(screen.getByText(/limit reached/)).toBeInTheDocument()
  })

  it('shows the classifying state and offers reclassify only after 60 seconds', () => {
    const { unmount } = render(<TypeConfirmationCard request={req({ status: 'classifying' })} onChanged={vi.fn()} />)
    expect(screen.getByTestId('request-classifying')).toBeInTheDocument()
    expect(screen.queryByText('Reclassify')).toBeNull()
    unmount()
    const old = new Date(Date.now() - 120_000).toISOString()
    render(<TypeConfirmationCard request={req({ status: 'classifying', updatedAt: old })} onChanged={vi.fn()} />)
    expect(screen.getByText('Reclassify')).toBeInTheDocument()
  })
})

describe('reason dialogs and shortcuts', () => {
  it('Mod+Enter submits with ctrlKey on Linux/Windows and metaKey on macOS', async () => {
    const onConfirm = vi.fn(async () => true)
    const { unmount } = render(<CancelRequestDialog open onOpenChange={vi.fn()} onConfirm={onConfirm} />)
    fireEvent.change(screen.getByLabelText('Reason (optional)'), { target: { value: 'dup' } })
    fireEvent.keyDown(screen.getByLabelText('Reason (optional)'), { key: 'Enter', metaKey: true })
    expect(onConfirm).not.toHaveBeenCalled()
    fireEvent.keyDown(screen.getByLabelText('Reason (optional)'), { key: 'Enter', ctrlKey: true })
    await waitFor(() => expect(onConfirm).toHaveBeenCalledWith('dup'))
    unmount()

    platform.mockReturnValue('darwin')
    onConfirm.mockClear()
    render(<CancelRequestDialog open onOpenChange={vi.fn()} onConfirm={onConfirm} />)
    fireEvent.keyDown(screen.getByLabelText('Reason (optional)'), { key: 'Enter', ctrlKey: true })
    expect(onConfirm).not.toHaveBeenCalled()
    fireEvent.keyDown(screen.getByLabelText('Reason (optional)'), { key: 'Enter', metaKey: true })
    await waitFor(() => expect(onConfirm).toHaveBeenCalled())
  })

  it('change type does not send without a new type and reason', () => {
    render(<ChangeTypeConfirmDialog open onOpenChange={vi.fn()} request={req({ status: 'analyzing' })} onChanged={vi.fn()} onSpawnChild={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Change type' })).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Why is the type changing?'), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: 'Change type' }))
    expect(actions.changeType).not.toHaveBeenCalled()
  })
})

describe('RequestHistoryTab', () => {
  const noop = vi.fn()
  it('is empty, loading and error aware', () => {
    const { unmount } = render(<RequestHistoryTab entries={[]} isLoading={false} error={null} onRetry={noop} />)
    expect(screen.getByTestId('request-history-empty')).toBeInTheDocument()
    unmount()
    render(<RequestHistoryTab entries={[]} isLoading={false} error="network" onRetry={noop} />)
    fireEvent.click(screen.getByText('Retry'))
    expect(noop).toHaveBeenCalled()
  })

  it('lists newest first with AI actors labelled', () => {
    render(
      <RequestHistoryTab
        isLoading={false}
        error={null}
        onRetry={noop}
        entries={[
          { id: '1', requestId: 'r', fromType: 'bug', toType: 'task', occurredAt: '2026-01-01T00:00:00Z', actorKind: 'ai', reason: 'older' },
          { id: '2', requestId: 'r', fromType: 'task', toType: 'docs', occurredAt: '2026-02-01T00:00:00Z', actorId: 'bob', reason: 'newer' }
        ]}
      />
    )
    const items = screen.getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('newer')
    expect(items[1]).toHaveTextContent('AI')
  })
})

describe('RequestRelatedTab', () => {
  it('says so when links are unsupported or empty', () => {
    const { unmount } = render(<RequestRelatedTab requestId="r1" links={[]} linksSupported={false} />)
    expect(screen.getByTestId('request-related-unsupported')).toBeInTheDocument()
    unmount()
    render(<RequestRelatedTab requestId="r1" links={[]} linksSupported />)
    expect(screen.getByTestId('request-related-empty')).toBeInTheDocument()
  })

  it('opens a cached related request and marks unreadable ones', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'forbidden' } })
    useAppStore.setState({ requestsById: { kid: req({ id: 'kid', number: 9, title: 'Kid req', status: 'analyzing' }) } })
    render(
      <RequestRelatedTab
        requestId="r1"
        linksSupported
        links={[
          { id: '1', requestId: 'r1', relatedRequestId: 'kid', reason: 'child', createdAt: '' },
          { id: '2', requestId: 'r1', relatedRequestId: 'hidden', reason: 'escalation', createdAt: '' }
        ]}
      />
    )
    fireEvent.click(screen.getByText('Kid req'))
    expect(useAppStore.getState().requestPage.requestId).toBe('kid')
    await waitFor(() => expect(screen.getByText('Cannot view this request')).toBeInTheDocument())
  })
})

describe('SpawnChildRequestDialog', () => {
  it.each([
    ['spike', 'spawned_by_spike'], ['question', 'spawned_by_question'], ['hotfix', 'followup_hotfix'],
    ['bug', 'escalation'], ['security', 'escalation']
  ] as const)('defaults the link reason for %s', async (type, reason) => {
    actions.spawnChild.mockResolvedValue({ ok: true, value: { request: req({ id: 'kid', title: 'Kid' }) } })
    render(<SpawnChildRequestDialog open onOpenChange={vi.fn()} request={req({ type, status: 'awaiting_analysis_approval' })} onChanged={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(actions.spawnChild).toHaveBeenCalled())
    expect(actions.spawnChild.mock.calls[0][0]).toMatchObject({ id: 'r1', reason, title: expect.stringContaining('Parent') })
    expect(actions.spawnChild.mock.calls[0][0].type).toBeUndefined()
    await waitFor(() => expect(useAppStore.getState().requestPage.requestId).toBe('kid'))
  })

  it('disables create without a title', () => {
    render(<SpawnChildRequestDialog open onOpenChange={vi.fn()} request={req({ type: 'bug' })} onChanged={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: '  ' } })
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled()
  })
})

describe('CreateRequestDialog', () => {
  it('prefills, blocks without a project, then creates and opens the request', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { request: { id: 'new1', title: 'T', type: null, status: 'new' }, created: true } })
    render(<CreateRequestDialog open onOpenChange={vi.fn()} initial={{ title: 'From issue', body: 'b', source: { provider: 'jira', ref: 'A-1', url: 'https://x/A-1' } }} />)
    expect(screen.getByLabelText('Title')).toHaveValue('From issue')
    // single project auto-selects after load
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    const [method, params] = callRequestRpc.mock.calls[0]
    expect(method).toBe('request.create')
    expect(params).toMatchObject({ projectId: 'proj-1', title: 'From issue', source: { provider: 'jira', ref: 'A-1' } })
    expect(params.clientRequestId).toBeTruthy()
    await waitFor(() => expect(useAppStore.getState().requestPage.requestId).toBe('new1'))
  })

  it('shows the pending-limit error and keeps the dialog open', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'validation', code: 'REQUEST_PENDING_LIMIT', message: '' } })
    const onOpenChange = vi.fn()
    render(<CreateRequestDialog open onOpenChange={onOpenChange} initial={{ title: 'x' }} />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Too many requests'))
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('a duplicate (created=false) does not navigate away', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { request: { id: 'old', title: 'T' }, created: false } })
    useAppStore.setState({ requestPage: { section: 'requests', requestId: null, backlogView: 'requests', listFilters: {} } })
    render(<CreateRequestDialog open onOpenChange={vi.fn()} initial={{ title: 'x' }} />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(useAppStore.getState().requestPage.requestId).toBeNull()
  })
})
