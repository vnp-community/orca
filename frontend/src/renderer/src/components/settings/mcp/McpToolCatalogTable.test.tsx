// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { McpToolView } from '../../../../../shared/mcp-types'
import { McpToolCatalogTable } from './McpToolCatalogTable'
import { groupTools } from './mcp-tool-catalog-grouping'

const tool = (name: string, over: Partial<McpToolView> = {}): McpToolView => ({
  name,
  channel: name,
  title: `Title ${name}`,
  description: 'desc',
  namespace: 'git',
  risk: 'read',
  requiredScope: 'orca:read',
  pack: 1,
  hardDenied: false,
  effective: 'allow',
  effectiveSource: 'default',
  annotations: { readOnly: true, destructive: false, idempotent: false, openWorld: false },
  ...over
})

afterEach(cleanup)

const renderTable = (tools: McpToolView[], onEditPolicy?: (n: string) => void) =>
  render(
    <McpToolCatalogTable
      groups={groupTools(tools, 'namespace')}
      groupBy="namespace"
      onEditPolicy={onEditPolicy}
    />
  )

describe('McpToolCatalogTable', () => {
  it('shows needs-approval label with its source and an Edit policy action', () => {
    const onEdit = vi.fn()
    renderTable(
      [tool('terminal.run', { effective: 'require_approval', effectiveSource: 'tenant_policy' })],
      onEdit
    )
    expect(screen.getByText('Needs approval')).toBeTruthy()
    expect(screen.getByText('Tenant policy')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Edit policy for terminal.run' }))
    expect(onEdit).toHaveBeenCalledWith('terminal.run')
  })

  it('marks hard-denied tools with a lock and offers no edit action', () => {
    renderTable(
      [tool('fs.delete', { hardDenied: true, effective: 'deny', effectiveSource: 'hard_deny' })],
      vi.fn()
    )
    expect(screen.getByLabelText('Always blocked')).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Edit policy/ })).toBeNull()
  })

  it('degrades gracefully without a policy handler (no action buttons)', () => {
    renderTable([tool('git.status')])
    expect(screen.queryByRole('button', { name: /Edit policy/ })).toBeNull()
  })

  it('renders hostile descriptions as text', () => {
    const { container } = renderTable([tool('x', { description: '<img src=x onerror=alert(1)>' })])
    expect(container.querySelector('img')).toBeNull()
    expect(container.textContent).toContain('<img src=x onerror=alert(1)>')
  })

  it('collapses large groups by default and toggles with aria-expanded', () => {
    const many = Array.from({ length: 13 }, (_, i) => tool(`git.t${i}`))
    renderTable(many)
    const toggle = screen.getByRole('button', { name: /git/ })
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByText('git.t0')).toBeNull()
    fireEvent.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByText('git.t0')).toBeTruthy()
  })
})
