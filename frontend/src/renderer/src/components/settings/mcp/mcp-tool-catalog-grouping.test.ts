import { describe, expect, it } from 'vitest'
import type { McpToolView } from '../../../../../shared/mcp-types'
import {
  DEFAULT_TOOL_FILTERS,
  filterTools,
  groupTools,
  hasActiveToolFilters,
  listToolNamespaces,
  summarizeTools
} from './mcp-tool-catalog-grouping'

const tool = (name: string, over: Partial<McpToolView> = {}): McpToolView => ({
  name,
  channel: name,
  title: name,
  description: `${name} description`,
  namespace: 'git',
  risk: 'read',
  requiredScope: 'orca:read',
  pack: 1,
  hardDenied: false,
  effective: 'allow',
  effectiveSource: 'default',
  annotations: { readOnly: true, destructive: false, idempotent: true, openWorld: false },
  ...over
})

const tools = [
  tool('git.status'),
  tool('terminal.run', {
    namespace: 'terminal',
    risk: 'exec',
    pack: 3,
    effective: 'require_approval'
  }),
  tool('fs.delete', {
    namespace: 'fs',
    risk: 'destructive',
    pack: 3,
    effective: 'deny',
    effectiveSource: 'hard_deny',
    hardDenied: true
  }),
  tool('admin.users', { namespace: 'admin', risk: 'admin', pack: 4, effective: 'deny' })
]

describe('filterTools', () => {
  it('matches name, title and description case-insensitively', () => {
    expect(filterTools(tools, { ...DEFAULT_TOOL_FILTERS, query: 'TERMINAL' })).toHaveLength(1)
    expect(
      filterTools(tools, { ...DEFAULT_TOOL_FILTERS, query: 'git.status description' })
    ).toHaveLength(1)
  })

  it('combines namespace, risk, pack, decision and hideHardDenied', () => {
    const f = DEFAULT_TOOL_FILTERS
    expect(filterTools(tools, { ...f, namespace: 'fs' }).map((t) => t.name)).toEqual(['fs.delete'])
    expect(filterTools(tools, { ...f, risk: 'exec' }).map((t) => t.name)).toEqual(['terminal.run'])
    expect(filterTools(tools, { ...f, pack: 3 })).toHaveLength(2)
    expect(filterTools(tools, { ...f, decision: 'deny' })).toHaveLength(2)
    expect(
      filterTools(tools, { ...f, decision: 'deny', hideHardDenied: true }).map((t) => t.name)
    ).toEqual(['admin.users'])
  })
})

describe('groupTools', () => {
  it('orders namespaces A-Z', () => {
    expect(groupTools(tools, 'namespace').map((g) => g.key)).toEqual([
      'admin',
      'fs',
      'git',
      'terminal'
    ])
  })

  it('orders packs numerically', () => {
    expect(groupTools(tools, 'pack').map((g) => [g.key, g.tools.length])).toEqual([
      ['1', 1],
      ['3', 2],
      ['4', 1]
    ])
  })

  it('orders risks from safest to most dangerous', () => {
    expect(groupTools(tools, 'risk').map((g) => g.key)).toEqual([
      'read',
      'exec',
      'destructive',
      'admin'
    ])
  })

  it('does not mutate the input', () => {
    const copy = [...tools]
    groupTools(tools, 'namespace')
    expect(tools).toEqual(copy)
  })
})

describe('summaries', () => {
  it('counts decisions and hard-denied tools', () => {
    expect(summarizeTools(tools)).toEqual({
      total: 4,
      byDecision: { allow: 1, require_approval: 1, deny: 2 },
      hardDenied: 1
    })
  })

  it('lists unique sorted namespaces and detects active filters', () => {
    expect(listToolNamespaces(tools)).toEqual(['admin', 'fs', 'git', 'terminal'])
    expect(hasActiveToolFilters(DEFAULT_TOOL_FILTERS)).toBe(false)
    expect(hasActiveToolFilters({ ...DEFAULT_TOOL_FILTERS, query: ' ' })).toBe(false)
    expect(hasActiveToolFilters({ ...DEFAULT_TOOL_FILTERS, hideHardDenied: true })).toBe(true)
  })
})
