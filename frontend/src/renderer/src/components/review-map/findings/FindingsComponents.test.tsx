// @vitest-environment happy-dom
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FindingDismissPopover } from './FindingDismissPopover'
import { FindingRow } from './FindingRow'
import type { FindingRowProps } from './FindingRow'
import { toFindingRow } from './finding-view-model'
import { makeFinding } from '../../../test-support/contract-findings-fixtures'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const t = (_k: string, fallback: string): string => fallback

function rowProps(over: Partial<FindingRowProps> & { finding?: Parameters<typeof makeFinding>[0] } = {}): FindingRowProps {
  const { finding, ...rest } = over
  return {
    row: toFindingRow(makeFinding(finding), t),
    busy: false,
    error: null,
    onViewInGraph: () => {},
    onOpenLocation: () => {},
    openLocationLabel: 'View diff',
    onDismiss: vi.fn(),
    onResolve: vi.fn(),
    onRestore: vi.fn(),
    onRetry: vi.fn(),
    ...rest
  }
}

function setUserAgent(ua: string): void {
  vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(ua)
}

describe('FindingDismissPopover', () => {
  const open = (onConfirm = vi.fn()) => {
    render(
      <FindingDismissPopover onConfirm={onConfirm}>
        <button type="button">Ignore</button>
      </FindingDismissPopover>
    )
    fireEvent.click(screen.getByText('Ignore'))
    return onConfirm
  }

  it('requires a reason preset before confirming and states the repo-wide scope', () => {
    const onConfirm = open()
    expect(screen.getByText(/Applies to the whole repository/)).toBeTruthy()
    const confirm = screen.getAllByRole('button', { name: 'Ignore' }).at(-1)!
    expect((confirm as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByLabelText('False positive'))
    fireEvent.change(screen.getByLabelText('Note'), { target: { value: ' why ' } })
    fireEvent.click(screen.getAllByRole('button', { name: 'Ignore' }).at(-1)!)
    expect(onConfirm).toHaveBeenCalledWith({ reason: 'false_positive', note: 'why' })
  })

  it('Mod+Enter confirms: Cmd on macOS, Ctrl elsewhere', () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X)')
    const onConfirm = open()
    fireEvent.click(screen.getByLabelText('Accepted risk'))
    const note = screen.getByLabelText('Note')
    fireEvent.keyDown(note, { key: 'Enter', ctrlKey: true })
    expect(onConfirm).not.toHaveBeenCalled()
    fireEvent.keyDown(note, { key: 'Enter', metaKey: true })
    expect(onConfirm).toHaveBeenCalledWith({ reason: 'accepted_risk' })
  })

  it('Ctrl+Enter confirms on Linux and Cmd does not', () => {
    setUserAgent('Mozilla/5.0 (X11; Linux x86_64)')
    const onConfirm = open()
    fireEvent.click(screen.getByLabelText('Fix later'))
    const note = screen.getByLabelText('Note')
    fireEvent.keyDown(note, { key: 'Enter', metaKey: true })
    expect(onConfirm).not.toHaveBeenCalled()
    fireEvent.keyDown(note, { key: 'Enter', ctrlKey: true })
    expect(onConfirm).toHaveBeenCalledWith({ reason: 'later' })
  })

  it('does not confirm with Mod+Enter when no reason is chosen', () => {
    const onConfirm = open()
    fireEvent.keyDown(screen.getByLabelText('Note'), { key: 'Enter', ctrlKey: true, metaKey: true })
    expect(onConfirm).not.toHaveBeenCalled()
  })
})

describe('FindingRow', () => {
  it('shows severity text, location, origin and the open-row actions', () => {
    render(<FindingRow {...rowProps()} />)
    const item = screen.getByRole('listitem')
    expect(item.getAttribute('aria-label')).toContain('Error: ')
    expect(within(item).getByText('svc/handlers/order.go:40')).toBeTruthy()
    expect(within(item).getByText('Introduced by this change')).toBeTruthy()
    expect(within(item).getByText('View in graph')).toBeTruthy()
    expect(within(item).getByText('Mark resolved')).toBeTruthy()
    expect(within(item).queryByText('Reopen')).toBeNull()
    expect(within(item).queryByText('Note')).toBeNull()
  })

  it('hides "View in graph" without a target and shows Note only when supported', () => {
    render(<FindingRow {...rowProps({ onViewInGraph: null, noteSlot: <button type="button">Note</button> })} />)
    expect(screen.queryByText('View in graph')).toBeNull()
    expect(screen.getByText('Note')).toBeTruthy()
  })

  it('a dismissed row offers Reopen instead of Ignore / Mark resolved', () => {
    const onRestore = vi.fn()
    render(
      <FindingRow
        {...rowProps({
          onRestore,
          finding: { dismissed: { by: 'u', at: 'x', reason: 'false_positive', disposition: 'ignored' } }
        })}
      />
    )
    expect(screen.getByText('Ignored: False positive')).toBeTruthy()
    fireEvent.click(screen.getByText('Reopen'))
    expect(onRestore).toHaveBeenCalled()
    expect(screen.queryByText('Mark resolved')).toBeNull()
  })

  it('busy disables the actions; forbidden shows a permission message without Retry', async () => {
    const { rerender } = render(<FindingRow {...rowProps({ busy: true })} />)
    expect((screen.getByText('Mark resolved') as HTMLButtonElement).disabled).toBe(true)
    rerender(
      <FindingRow
        {...rowProps({ error: { kind: 'forbidden', code: 'CODEINTEL_NOT_AUTHORIZED', message: '', retryable: false } })}
      />
    )
    expect(screen.getByRole('alert').textContent).toContain('You do not have permission to dismiss findings.')
    expect(screen.queryByText('Retry')).toBeNull()
    const onRetry = vi.fn()
    rerender(
      <FindingRow {...rowProps({ onRetry, error: { kind: 'offline', code: null, message: '', retryable: true } })} />
    )
    await act(async () => {
      fireEvent.click(screen.getByText('Retry'))
    })
    expect(onRetry).toHaveBeenCalled()
  })
})
