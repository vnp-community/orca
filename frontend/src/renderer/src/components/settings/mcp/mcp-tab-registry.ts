import type { ComponentType } from 'react'
import type { McpServerInfo } from '../../../../../shared/mcp-types'
import type { McpTabId } from '@/store/slices/mcp-slice'

export type McpTabDefinition = {
  id: McpTabId
  titleKey: string
  titleDefault: string
  adminOnly: boolean
  visible?: (info: McpServerInfo) => boolean
  /** Lazy: one chunk per tab. */
  load: () => Promise<{ default: ComponentType }>
}

// Extension point: each FE-MCP solution appends exactly ONE entry here, e.g.
// { id: 'connect', titleKey: 'auto.mcp.tabs.connect', titleDefault: 'Connect', adminOnly: false,
//   load: () => import('./McpConnectTab') }
export const MCP_TABS: readonly McpTabDefinition[] = [
  {
    id: 'connect',
    titleKey: 'auto.mcp.tabs.connect',
    titleDefault: 'Connect',
    adminOnly: false,
    load: () => import('./McpConnectTab').then((m) => ({ default: m.McpConnectTab }))
  },
  {
    id: 'apps',
    titleKey: 'auto.mcp.tabs.apps',
    titleDefault: 'Connected apps',
    adminOnly: false,
    load: () =>
      import('./McpConnectedAppsTab').then((m) => ({
        default: m.McpConnectedAppsTab
      }))
  },
  {
    id: 'tokens',
    titleKey: 'auto.mcp.tabs.tokens',
    titleDefault: 'Access tokens',
    adminOnly: false,
    load: () =>
      import('./McpAccessTokensTab').then((m) => ({
        default: m.McpAccessTokensTab
      }))
  },
  {
    id: 'clients',
    titleKey: 'auto.mcp.tabs.clients',
    titleDefault: 'OAuth clients',
    adminOnly: true,
    load: () =>
      import('./McpOAuthClientsTab').then((m) => ({
        default: m.McpOAuthClientsTab
      }))
  },
  {
    id: 'grants',
    titleKey: 'auto.mcp.tabs.grants',
    titleDefault: 'All grants',
    adminOnly: true,
    load: () => import('./McpAllGrantsTab').then((m) => ({ default: m.McpAllGrantsTab }))
  },
  {
    id: 'tools',
    titleKey: 'auto.mcp.tabs.tools',
    titleDefault: 'Tools',
    adminOnly: true,
    load: () => import('./McpToolsTab').then((m) => ({ default: m.McpToolsTab }))
  },
  {
    id: 'prompts',
    titleKey: 'auto.mcp.tabs.prompts',
    titleDefault: 'Prompts',
    adminOnly: true,
    load: () => import('./McpPromptsTab').then((m) => ({ default: m.McpPromptsTab }))
  },
  {
    id: 'approvals',
    titleKey: 'auto.mcp.tabs.approvals',
    titleDefault: 'Approvals',
    adminOnly: false,
    load: () => import('./McpApprovalsTab').then((m) => ({ default: m.McpApprovalsTab }))
  },
  {
    id: 'policy',
    titleKey: 'auto.mcp.tabs.policy',
    titleDefault: 'Tool policies',
    adminOnly: true,
    load: () => import('./McpPoliciesTab').then((m) => ({ default: m.McpPoliciesTab }))
  },
  {
    id: 'audit',
    titleKey: 'auto.mcp.tabs.audit',
    titleDefault: 'MCP audit',
    adminOnly: true,
    load: () => import('./McpAuditTab').then((m) => ({ default: m.McpAuditTab }))
  },
  {
    id: 'servers',
    titleKey: 'auto.mcp.tabs.servers',
    titleDefault: 'External servers',
    adminOnly: false,
    load: () =>
      import('./McpExternalServersTab').then((m) => ({ default: m.McpExternalServersTab }))
  }
]

/** Admin-only setup card (FE-MCP-SOL-008 sets this); null until that solution lands. */
export const MCP_ADMIN_ENABLE_CARD: {
  load: (() => Promise<{ default: ComponentType }>) | null
} = { load: () => import('./McpAdminEnableCard') }

export function getVisibleMcpTabs(
  info: McpServerInfo,
  isAdmin: boolean,
  tabs: readonly McpTabDefinition[] = MCP_TABS
): McpTabDefinition[] {
  return tabs.filter((t) => (!t.adminOnly || isAdmin) && (t.visible ? t.visible(info) : true))
}

/** Unknown/forbidden target falls back to 'connect', then the first visible tab. */
export function resolveMcpTab(
  requested: McpTabId | undefined,
  visibleTabs: readonly McpTabDefinition[]
): McpTabId | null {
  if (requested && visibleTabs.some((t) => t.id === requested)) {
    return requested
  }
  return visibleTabs.find((t) => t.id === 'connect')?.id ?? visibleTabs[0]?.id ?? null
}
