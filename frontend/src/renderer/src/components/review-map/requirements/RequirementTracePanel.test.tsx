// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { UseRequirementTraceResult } from './use-requirement-trace'
import { buildRequirementTraceViewModel } from './requirement-trace-view-model'
import { evidenceWire, requirementWire, traceFixture } from './requirement-trace.fixture'

const hook = vi.hoisted(() => ({ value: null as unknown }))
vi.mock('./use-requirement-trace', () => ({ useRequirementTrace: () => hook.value }))
vi.mock('./WorktreeTaskLinkPicker', () => ({
  WorktreeTaskLinkPicker: (p: { onLink: (id: string) => void }) => (
    <button type="button" onClick={() => p.onLink('task-9')}>
      picker
    </button>
  )
}))

import { RequirementTracePanel } from './RequirementTracePanel'

const translate = (key: string) => key

function setHook(over: Partial<UseRequirementTraceResult> & { trace?: Record<string, unknown> }) {
  const { trace, ...rest } = over
  hook.value = {
    status: 'ready',
    view: buildRequirementTraceViewModel(traceFixture(trace ?? {}), { showInferred: true }),
    stale: false,
    readOnly: false,
    actionError: null,
    showInferred: true,
    setShowInferred: vi.fn(),
    confirm: vi.fn(async () => {}),
    reject: vi.fn(async () => {}),
    linkTask: vi.fn(async () => {}),
    unlinkTask: vi.fn(async () => {}),
    refetch: vi.fn(),
    ...rest
  } satisfies UseRequirementTraceResult
  return hook.value as UseRequirementTraceResult
}

const renderPanel = (onOpenDiff = vi.fn()) =>
  render(<RequirementTracePanel worktreeId="wt" projectId="p" onOpenDiff={onOpenDiff} translate={translate} />)

beforeEach(() => setHook({}))
afterEach(cleanup)

describe('RequirementTracePanel', () => {
  it('renders nothing when disabled and a status line while loading', () => {
    setHook({ status: 'disabled', view: null })
    expect(renderPanel().container.innerHTML).toBe('')
    cleanup()
    setHook({ status: 'loading', view: null })
    expect(screen.queryByRole('status')).toBeNull()
    renderPanel()
    expect(screen.getByRole('status')).toBeTruthy()
  })

  it('offers retry on error', () => {
    const state = setHook({ status: 'error', view: null })
    renderPanel()
    fireEvent.click(screen.getByText(/trace\.retry/))
    expect(state.refetch).toHaveBeenCalled()
  })

  it('shows groups with "no evidence" before "evidence found" and renders requirement text as plain text', () => {
    setHook({
      trace: {
        requirements: [
          requirementWire({ key: 'a', text: '<b>bold</b> **md**', state: 'has_evidence' }),
          requirementWire({ key: 'b', text: 'second', state: 'no_evidence', evidence: [] })
        ]
      }
    })
    const { container } = renderPanel()
    const headings = [...container.querySelectorAll('h3')].map((h) => h.textContent)
    expect(headings[0]).toContain('group.noEvidence')
    expect(headings[1]).toContain('group.hasEvidence')
    expect(screen.getByText('<b>bold</b> **md**')).toBeTruthy()
    expect(container.querySelector('b')).toBeNull()
  })

  it('confirm and dismiss on a suggestion call the actions with the evidence ref', async () => {
    const state = setHook({
      trace: {
        requirements: [
          requirementWire({ key: 'k', state: 'no_evidence', evidence: [evidenceWire({ confidence: 'inferred', ref: 'guess.ts', label: 'guess.ts' })] })
        ]
      }
    })
    renderPanel()
    fireEvent.click(screen.getByText(/trace\.confirm/))
    expect(state.confirm).toHaveBeenCalledWith('k', { kind: 'change', ref: 'guess.ts' })
    // Buttons lock while a write is in flight; wait for it to settle before the next click.
    await vi.waitFor(() => expect((screen.getByText(/trace\.dismiss/).closest('button') as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(screen.getByText(/trace\.dismiss/))
    expect(state.reject).toHaveBeenCalledWith('k', { kind: 'change', ref: 'guess.ts' })
  })

  it('hides write actions when read-only', () => {
    setHook({
      readOnly: true,
      trace: { requirements: [requirementWire({ state: 'manual_pending', evidence: [evidenceWire({ confidence: 'inferred' })] })] }
    })
    renderPanel()
    expect(screen.queryByText(/trace\.confirm/)).toBeNull()
    expect(screen.queryByText(/trace\.manualConfirm/)).toBeNull()
    expect(screen.getByText(/trace\.readOnly/)).toBeTruthy()
  })

  it('"I checked this" records a manual confirmation', () => {
    const state = setHook({ trace: { requirements: [requirementWire({ key: 'm', state: 'manual_pending', evidence: [] })] } })
    renderPanel()
    fireEvent.click(screen.getByText(/trace\.manualConfirm/))
    expect(state.confirm).toHaveBeenCalledWith('m', { kind: 'manual_confirmation', ref: 'm' })
  })

  it('opens the diff for change evidence and for unlinked files', () => {
    const onOpenDiff = vi.fn()
    setHook({})
    renderPanel(onOpenDiff)
    fireEvent.click(screen.getByText('a.ts'))
    expect(onOpenDiff).toHaveBeenCalledWith('src/a.ts')
    fireEvent.click(screen.getByText('src/other.ts'))
    expect(onOpenDiff).toHaveBeenCalledWith('src/other.ts')
  })

  it('shows the picker when no task is linked and links through it', () => {
    const state = setHook({ trace: { linkConfidence: 'none', requirements: [] } })
    renderPanel()
    fireEvent.click(screen.getByText('picker'))
    expect(state.linkTask).toHaveBeenCalledWith('task-9')
    expect(screen.getByText(/trace\.empty/)).toBeTruthy()
  })

  it('shows change/unlink instead of the picker for an explicit link, and unlinks', () => {
    const state = setHook({ trace: { linkConfidence: 'explicit' } })
    renderPanel()
    expect(screen.queryByText('picker')).toBeNull()
    fireEvent.click(screen.getByText(/taskPicker\.unlink/))
    expect(state.unlinkTask).toHaveBeenCalled()
  })

  it('toggles inferred suggestions', () => {
    const state = setHook({ showInferred: false })
    renderPanel()
    fireEvent.click(within(document.body).getByRole('checkbox'))
    expect(state.setShowInferred).toHaveBeenCalledWith(true)
  })
})
