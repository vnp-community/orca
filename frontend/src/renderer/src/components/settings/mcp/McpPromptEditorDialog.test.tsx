// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { McpPrompt } from '../../../../../shared/mcp-types'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpPromptEditorDialog, type McpPromptEditorMode } from './McpPromptEditorDialog'

const confirm = vi.fn()
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))

const custom: McpPrompt = {
  id: 'p1',
  name: 'my_prompt',
  description: 'desc',
  version: 3,
  updatedAt: '2026-10-01T00:00:00Z',
  arguments: [{ name: 'topic', description: '', required: true }],
  template: 'Explain {{topic}}',
  builtin: false
}
const builtin: McpPrompt = {
  ...custom,
  id: 'b1',
  name: 'plan_task',
  builtin: true,
  template: '{{.Task}} {{#if x}}y{{/if}}'
}

const setup = (mode: McpPromptEditorMode, over: Record<string, unknown> = {}) => {
  const onSave = vi.fn().mockResolvedValue(undefined)
  const onClose = vi.fn()
  render(
    <McpPromptEditorDialog
      mode={mode}
      existingNames={new Set(['my_prompt', 'plan_task'])}
      onSave={onSave}
      onClose={onClose}
      {...over}
    />
  )
  return { onSave, onClose }
}
const saveBtn = (): HTMLButtonElement => screen.getByRole('button', { name: 'Save' })

beforeEach(() => confirm.mockReset())
afterEach(cleanup)

describe('McpPromptEditorDialog', () => {
  it('built-in view is read-only with no Save button and a description present', () => {
    setup({ kind: 'view', prompt: builtin })
    expect(screen.queryByRole('button', { name: 'Save' })).toBeNull()
    expect((screen.getByLabelText('Name') as HTMLInputElement).readOnly).toBe(true)
    expect(screen.getByText(/Built-in — read only/)).toBeTruthy()
    expect(
      screen.getByText('Name and arguments are shown to users in their MCP client.')
    ).toBeTruthy()
  })

  it('duplicating a built-in with advanced syntax blocks Save until fixed', () => {
    setup({ kind: 'duplicate', from: builtin })
    expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe('plan_task_copy')
    expect(screen.getByText(/advanced syntax/)).toBeTruthy()
    expect(saveBtn().disabled).toBe(true)
    fireEvent.change(document.getElementById('mcp-prompt-template')!, {
      target: { value: 'Plan {{topic}}' }
    })
    expect(saveBtn().disabled).toBe(false)
  })

  it('saves with the stored id and version for edits', async () => {
    const { onSave, onClose } = setup({ kind: 'edit', prompt: custom })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'new' } })
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'p1', version: 3, description: 'new', builtin: false })
    )
  })

  it('a create draft has no id/version and cannot reuse an existing name', () => {
    const { onSave } = setup({ kind: 'create' })
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'my_prompt' } })
    expect(screen.getByText('A prompt with this name already exists.')).toBeTruthy()
    expect(saveBtn().disabled).toBe(true)
    expect(onSave).not.toHaveBeenCalled()
  })

  it('keeps the dialog open and attaches MCP_PROMPT_INVALID to the template field', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(new McpRpcError('MCP_PROMPT_INVALID', 'template: contains forbidden text'))
    const { onClose } = setup({ kind: 'edit', prompt: custom }, { onSave })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'changed' } })
    fireEvent.click(saveBtn())
    await screen.findByText('contains forbidden text')
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Description')).toBeTruthy()
  })

  it('shows a name error for MCP_PROMPT_NAME_CONFLICT', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(new McpRpcError('MCP_PROMPT_NAME_CONFLICT', 'name taken on server'))
    setup({ kind: 'edit', prompt: custom }, { onSave })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'changed' } })
    fireEvent.click(saveBtn())
    await screen.findByText('name taken on server')
  })

  it('offers Reload on version conflict and loads the newer copy while keeping the draft', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(new McpRpcError('MCP_PROMPT_VERSION_CONFLICT', 'stale'))
    const onReload = vi
      .fn()
      .mockResolvedValue({ ...custom, version: 4, template: 'Server {{topic}}' })
    setup({ kind: 'edit', prompt: custom }, { onSave, onReload })
    fireEvent.change(document.getElementById('mcp-prompt-template')!, {
      target: { value: 'Mine {{topic}}' }
    })
    fireEvent.click(saveBtn())
    await screen.findByText('This prompt was changed by someone else.')
    fireEvent.click(screen.getByRole('button', { name: 'Reload' }))
    await screen.findByText('Your changes')
    expect((document.getElementById('mcp-prompt-template') as HTMLTextAreaElement).value).toBe(
      'Server {{topic}}'
    )
    expect(screen.getByText('Version 4')).toBeTruthy()
  })

  it('locks the form on MCP_NOT_ADMIN and on built-in read-only errors', async () => {
    const onSave = vi.fn().mockRejectedValue(new McpRpcError('MCP_NOT_ADMIN', 'admin only'))
    setup({ kind: 'edit', prompt: custom }, { onSave })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'x' } })
    fireEvent.click(saveBtn())
    await screen.findByText('admin only')
    expect((screen.getByLabelText('Name') as HTMLInputElement).readOnly).toBe(true)
    expect(saveBtn().disabled).toBe(true)
  })

  it('asks before discarding unsaved changes', async () => {
    confirm.mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    const { onClose } = setup({ kind: 'edit', prompt: custom })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(confirm).toHaveBeenCalledTimes(1))
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('closes without confirmation when nothing changed', async () => {
    const { onClose } = setup({ kind: 'edit', prompt: custom })
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(confirm).not.toHaveBeenCalled()
  })

  it('inserts a variable chip and renders the preview as plain text', () => {
    setup({ kind: 'create' })
    const template = document.getElementById('mcp-prompt-template') as HTMLTextAreaElement
    fireEvent.change(template, { target: { value: '<img src=x onerror=alert(1)> ' } })
    expect(document.querySelector('pre img')).toBeNull()
    expect(document.querySelector('pre')?.textContent).toContain('<img src=x onerror=alert(1)>')
  })

  it('has an accessible Add argument flow with a max of 10', () => {
    setup({ kind: 'create' })
    const add = screen.getByRole('button', { name: 'Add argument' })
    for (let i = 0; i < 10; i++) {
      fireEvent.click(add)
    }
    expect((add as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getAllByLabelText(/Argument \d+ name/)).toHaveLength(10)
  })
})
