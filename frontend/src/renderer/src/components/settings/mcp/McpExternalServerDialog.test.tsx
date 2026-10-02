// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpExternalServerDialog } from './McpExternalServerDialog'

vi.mock('./McpExternalServerTeamSelect', () => ({ McpExternalServerTeamSelect: () => null }))

const SECRET = 'tok-SECRET-123'
const saved = (over: Partial<McpExternalServer> = {}): McpExternalServer => ({
  id: 's1',
  scope: 'user',
  name: 'srv',
  transport: 'http',
  url: 'https://x.example.com/mcp',
  envRefs: [],
  headerRefs: [{ name: 'Authorization', hasSecret: false }],
  status: 'pending_review',
  toolsChanged: false,
  createdBy: 'u',
  ...over
})

const upsert = vi.fn()
const setSecret = vi.fn()
const onClose = vi.fn()
const renderDialog = (server: McpExternalServer | null = null, isAdmin = false) =>
  render(
    <McpExternalServerDialog
      server={server}
      isAdmin={isAdmin}
      api={{ upsert, setSecret }}
      onClose={onClose}
    />
  )

beforeEach(() => {
  upsert.mockReset()
  setSecret.mockReset()
  onClose.mockReset()
  window.localStorage.clear()
})
afterEach(cleanup)

const fillHttp = () => {
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'srv' } })
  fireEvent.change(screen.getByLabelText('URL'), { target: { value: 'https://x.example.com/mcp' } })
}

describe('McpExternalServerDialog', () => {
  it('locks non-admins to the "Just me" scope', () => {
    renderDialog()
    expect(screen.getByText('Just me')).toBeTruthy()
    expect(screen.queryByRole('radio', { name: 'Organization' })).toBeNull()
  })

  it('creates a server with a names-only body, then sends the secret once and wipes it', async () => {
    upsert.mockResolvedValue(saved())
    setSecret.mockResolvedValue(undefined)
    renderDialog()
    fillHttp()
    fireEvent.change(screen.getByLabelText('New header name'), {
      target: { value: 'Authorization' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    fireEvent.click(screen.getByRole('button', { name: 'Set value for Authorization' }))
    const input = screen.getByTestId('mcp-secret-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: SECRET } })
    fireEvent.click(screen.getByRole('button', { name: 'Set' }))
    expect(input.value).toBe('')
    expect(screen.getByText('Secret ready to save')).toBeTruthy()
    expect(document.body.innerHTML).not.toContain(SECRET)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    const body = upsert.mock.calls[0][0]
    expect(body).toEqual({
      name: 'srv',
      scope: 'user',
      transport: 'http',
      url: 'https://x.example.com/mcp',
      headerRefs: [{ name: 'Authorization' }],
      envRefs: []
    })
    expect(JSON.stringify(body)).not.toContain(SECRET)
    expect(setSecret).toHaveBeenCalledTimes(1)
    expect(setSecret).toHaveBeenCalledWith({
      serverId: 's1',
      kind: 'header',
      name: 'Authorization',
      value: SECRET
    })
    expect(JSON.stringify(window.localStorage)).not.toContain(SECRET)
  })

  it('clears the staged secret after a failed setSecret and shows a row error without the value', async () => {
    upsert.mockResolvedValue(saved())
    setSecret.mockRejectedValue(new McpRpcError('MCP_SERVER_INVALID', `bad ${SECRET}`))
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    renderDialog(saved())
    fireEvent.click(screen.getByRole('button', { name: 'Set value for Authorization' }))
    fireEvent.change(screen.getByTestId('mcp-secret-input'), { target: { value: SECRET } })
    fireEvent.click(screen.getByRole('button', { name: 'Set' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByText(/some secrets were not stored/)
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByText('No value yet')).toBeTruthy()
    // Retrying the same Save must not resend the cleared secret.
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(upsert).toHaveBeenCalledTimes(2))
    expect(setSecret).toHaveBeenCalledTimes(1)
    spy.mockRestore()
  })

  it('requires the stdio acknowledgement and shows the strong warning', async () => {
    upsert.mockResolvedValue(saved({ transport: 'stdio' }))
    renderDialog()
    fireEvent.click(screen.getByRole('radio', { name: 'stdio' }))
    expect(screen.getByText(/runs a program on the machine/)).toBeTruthy()
    expect(screen.getByText(/always need admin review/)).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('checkbox'))
    expect((screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('maps SSRF and name-conflict errors to inline field errors', async () => {
    upsert.mockRejectedValueOnce(new McpRpcError('MCP_SERVER_SSRF_BLOCKED', 'x'))
    renderDialog()
    fillHttp()
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByText(/isn't allowed/)
    upsert.mockRejectedValueOnce(new McpRpcError('MCP_SERVER_NAME_CONFLICT', 'x'))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByText(/already exists/)
  })

  it('warns that editing an approved server sends it back to review and never prefills secrets', () => {
    renderDialog(
      saved({ status: 'approved', headerRefs: [{ name: 'Authorization', hasSecret: true }] })
    )
    expect(screen.getByText('Secret set')).toBeTruthy()
    fireEvent.change(screen.getByLabelText('URL'), {
      target: { value: 'https://other.example.com' }
    })
    expect(screen.getByText('Changing this will send the server back to review.')).toBeTruthy()
  })

  it('wipes staged secrets when unmounted before saving', () => {
    const { unmount } = renderDialog(saved())
    fireEvent.click(screen.getByRole('button', { name: 'Set value for Authorization' }))
    fireEvent.change(screen.getByTestId('mcp-secret-input'), { target: { value: SECRET } })
    fireEvent.click(screen.getByRole('button', { name: 'Set' }))
    unmount()
    expect(upsert).not.toHaveBeenCalled()
    expect(document.body.innerHTML).not.toContain(SECRET)
  })
})
