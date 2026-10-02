import { describe, expect, it } from 'vitest'
import type { McpServerInfo } from '../../../../../shared/mcp-types'
import { getVisibleMcpTabs, resolveMcpTab, type McpTabDefinition } from './mcp-tab-registry'

const load = async () => ({ default: () => null })
const tab = (id: McpTabDefinition['id'], adminOnly = false): McpTabDefinition => ({
  id,
  titleKey: `k.${id}`,
  titleDefault: id,
  adminOnly,
  load
})
const info = {} as McpServerInfo

describe('mcp tab registry', () => {
  const tabs = [tab('connect'), tab('policy', true)]
  it('hides admin tabs from non-admins', () => {
    expect(getVisibleMcpTabs(info, false, tabs).map((t) => t.id)).toEqual(['connect'])
    expect(getVisibleMcpTabs(info, true, tabs).map((t) => t.id)).toEqual(['connect', 'policy'])
  })
  it('falls back to connect for forbidden or unknown tabs', () => {
    const visible = getVisibleMcpTabs(info, false, tabs)
    expect(resolveMcpTab('policy', visible)).toBe('connect')
    expect(resolveMcpTab(undefined, visible)).toBe('connect')
    expect(resolveMcpTab('connect', [])).toBeNull()
  })
})
