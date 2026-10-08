// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { UseC4OverrideResult, C4SaveOutcome } from '../../../hooks/useC4Override'

const api = vi.hoisted(() => ({ current: null as unknown }))
vi.mock('../../../hooks/useC4Override', () => ({ useC4Override: () => api.current }))

import { C4OverrideEditor } from './C4OverrideEditor'

const record = { document: 'a: 1\n', version: 3, updatedBy: null, updatedAt: null, seedSource: null }
const make = (over: Partial<UseC4OverrideResult> = {}): UseC4OverrideResult => ({
  status: 'ready',
  record,
  loadError: null,
  readOnly: false,
  saving: false,
  save: vi.fn(async (): Promise<C4SaveOutcome> => ({ ok: true, version: 4, warnings: [{ code: 'w', message: 'heads up' }] })),
  reload: vi.fn(async () => ({ ...record, document: 'latest: 1\n', version: 9 })),
  overwrite: vi.fn(async (): Promise<C4SaveOutcome> => ({ ok: true, version: 10, warnings: [] })),
  ...over
})

const onDraftChange = vi.fn()
const onSaved = vi.fn()
const onClose = vi.fn()
const mount = () =>
  render(
    <C4OverrideEditor open worktreeId="w" environmentId={null} container="c" draft={null}
      onDraftChange={onDraftChange} onSaved={onSaved} onClose={onClose} />
  )
const box = () => screen.getByRole('textbox') as HTMLTextAreaElement
const type = (v: string) => act(async () => { fireEvent.change(box(), { target: { value: v } }); await new Promise((r) => setTimeout(r, 350)) })
const saveBtn = () => screen.getByRole('button', { name: 'Save' })

beforeEach(() => { api.current = make(); vi.clearAllMocks() })
afterEach(cleanup)

describe('C4OverrideEditor', () => {
  it('loads the server document and keeps Save disabled until changed', async () => {
    mount()
    await act(async () => {})
    expect(box().value).toBe('a: 1\n')
    expect(saveBtn()).toBeDisabled()
  })

  it('blocks save and lists the issue with line:column on a syntax error', async () => {
    mount()
    await act(async () => {})
    await type('a: [1\n')
    expect(saveBtn()).toBeDisabled()
    expect(screen.getByRole('list', { name: 'Issues' })).toBeInTheDocument()
  })

  it('blocks a non-mapping root', async () => {
    mount()
    await act(async () => {})
    await type('- x\n')
    expect(saveBtn()).toBeDisabled()
  })

  it('saves, reports Saved only after success, shows warnings and clears the draft', async () => {
    mount()
    await act(async () => {})
    await type('a: 2\n')
    expect(screen.queryByText(/Saved at/)).toBeNull()
    await act(async () => { fireEvent.click(saveBtn()) })
    expect((api.current as UseC4OverrideResult).save).toHaveBeenCalledWith('a: 2\n')
    expect(screen.getByText(/Saved at/)).toBeInTheDocument()
    expect(screen.getByText('heads up')).toBeInTheDocument()
    expect(onSaved).toHaveBeenCalled()
    expect(onDraftChange).toHaveBeenLastCalledWith(null)
  })

  it('offers three conflict actions; overwrite needs a second confirm', async () => {
    api.current = make({ save: vi.fn(async () => ({ ok: false, failure: { kind: 'conflict', currentVersion: 5 } }) as C4SaveOutcome) })
    mount()
    await act(async () => {})
    await type('a: 2\n')
    await act(async () => { fireEvent.click(saveBtn()) })
    expect(screen.getByRole('button', { name: 'Load latest' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy mine' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Overwrite with mine' }))
    const overwrite = (api.current as UseC4OverrideResult).overwrite
    expect(overwrite).not.toHaveBeenCalled()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /Confirm: replace/ })) })
    expect(overwrite).toHaveBeenCalledWith('a: 2\n')
  })

  it('loads the latest version into the editor on conflict', async () => {
    api.current = make({ save: vi.fn(async () => ({ ok: false, failure: { kind: 'conflict', currentVersion: 9 } }) as C4SaveOutcome) })
    mount()
    await act(async () => {})
    await type('a: 2\n')
    await act(async () => { fireEvent.click(saveBtn()) })
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Load latest' })) })
    expect(box().value).toBe('latest: 1\n')
  })

  it('keeps the draft and says Not saved when offline', async () => {
    api.current = make({ save: vi.fn(async () => ({ ok: false, failure: { kind: 'offline' } }) as C4SaveOutcome) })
    mount()
    await act(async () => {})
    await type('a: 2\n')
    await act(async () => { fireEvent.click(saveBtn()) })
    expect(screen.getByRole('alert')).toHaveTextContent(/Not saved/)
    expect(box().value).toBe('a: 2\n')
  })

  it('goes read-only when forbidden', async () => {
    api.current = make({ readOnly: true })
    mount()
    await act(async () => {})
    expect(box()).toHaveAttribute('readonly')
    expect(saveBtn()).toBeDisabled()
  })

  it('asks before closing with unsaved changes', async () => {
    mount()
    await act(async () => {})
    await type('a: 2\n')
    fireEvent.click(screen.getAllByRole('button', { name: 'Close' }).find((b) => b.textContent === 'Close')!)
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Discard' }))
    expect(onClose).toHaveBeenCalled()
  })
})
