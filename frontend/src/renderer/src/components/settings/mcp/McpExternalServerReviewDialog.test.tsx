// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpExternalServerReviewDialog } from './McpExternalServerReviewDialog'

const probe = vi.fn()
const review = vi.fn()
const onClose = vi.fn()
const server = (over: Partial<McpExternalServer> = {}): McpExternalServer => ({
  id: 's1',
  scope: 'tenant',
  name: 'srv',
  transport: 'http',
  url: 'https://x.example.com/mcp',
  envRefs: [],
  headerRefs: [],
  status: 'pending_review',
  toolsChanged: false,
  createdBy: 'u',
  ...over
})
const renderDialog = (s = server(), paused = false) =>
  render(
    <McpExternalServerReviewDialog
      server={s}
      api={{ probe, review }}
      paused={paused}
      onClose={onClose}
    />
  )

beforeEach(() => {
  probe.mockReset()
  review.mockReset()
  onClose.mockReset()
})
afterEach(cleanup)

describe('McpExternalServerReviewDialog', () => {
  it('renders hostile tool text as plain text and visualizes bidi characters', async () => {
    probe.mockResolvedValue({
      transport: 'http',
      digest: 'abcdef0123456789',
      tools: [
        { name: 'run', description: '<img src=x onerror=alert(1)> [x](javascript:alert(1)) ‮evil' }
      ]
    })
    const { container } = renderDialog()
    await screen.findByText(/<img src=x/)
    expect(document.body.querySelector('img')).toBeNull()
    expect(document.body.querySelector('a[href^="javascript"]')).toBeNull()
    expect(document.body.textContent).toContain('\\u{202E}')
    expect(container).toBeTruthy()
    expect(screen.getByText(/untrusted/)).toBeTruthy()
  })

  it('approves with the digest from the probe after the reviewed checkbox', async () => {
    probe.mockResolvedValue({
      transport: 'http',
      digest: 'dig-1',
      tools: [{ name: 'a', description: 'd' }]
    })
    review.mockResolvedValue(server({ status: 'approved' }))
    renderDialog()
    await screen.findByText('Tools (1)')
    const approve = screen.getByRole('button', { name: 'Approve' }) as HTMLButtonElement
    expect(approve.disabled).toBe(true)
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(approve)
    await waitFor(() => expect(review).toHaveBeenCalledWith('s1', 'approve', 'dig-1'))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('re-probes on a digest mismatch and requires a fresh confirmation', async () => {
    probe
      .mockResolvedValueOnce({ transport: 'http', digest: 'old', tools: [] })
      .mockResolvedValueOnce({ transport: 'http', digest: 'new', tools: [] })
    review.mockRejectedValueOnce(new McpRpcError('MCP_SERVER_DIGEST_MISMATCH', 'x'))
    renderDialog()
    await screen.findByText('Tools (0)')
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await screen.findByText(/changed while you were reviewing/)
    await waitFor(() => expect(probe).toHaveBeenCalledTimes(2))
    await screen.findByText('Tools (0)')
    expect((screen.getByRole('checkbox') as HTMLInputElement).getAttribute('aria-checked')).toBe(
      'false'
    )
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows the diff against approved tools', async () => {
    probe.mockResolvedValue({
      transport: 'http',
      digest: 'd',
      tools: [
        { name: 'a', description: 'new text' },
        { name: 'added', description: 'x' }
      ],
      approvedTools: [
        { name: 'a', description: 'old text' },
        { name: 'gone', description: 'y' }
      ]
    })
    renderDialog(server({ status: 'approved', toolsChanged: true }))
    await screen.findByText('Description changed')
    expect(screen.getByText('old text')).toBeTruthy()
    expect(screen.getByText('Added')).toBeTruthy()
    expect(screen.getByText('Removed')).toBeTruthy()
  })

  it('shows the command instead of tools for stdio', async () => {
    probe.mockResolvedValue({ transport: 'stdio', digest: 'd', tools: [] })
    renderDialog(
      server({ transport: 'stdio', url: undefined, command: 'npx', args: ['-y', 'pkg'] })
    )
    await screen.findByText(/doesn't run stdio servers/)
    expect(screen.getAllByText('npx -y pkg').length).toBeGreaterThan(0)
    expect(screen.queryByText(/Tools \(/)).toBeNull()
  })

  it('locks Approve when the probe is blocked by the SSRF guard', async () => {
    probe.mockRejectedValue(new McpRpcError('MCP_SERVER_SSRF_BLOCKED', 'x'))
    renderDialog()
    await screen.findByText(/isn't allowed/)
    expect((screen.getByRole('button', { name: 'Approve' }) as HTMLButtonElement).disabled).toBe(
      true
    )
  })
})
