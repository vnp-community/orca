// Settings-search entries for the MCP pane. Admin-only tabs are indexed only for admins.

import { translate } from '@/i18n/i18n'
import { createLocalizedCatalog } from '@/i18n/localized-catalog'
import { translateSearchKeyword } from './settings-search-keywords'
import type { SettingsSearchEntry } from './settings-search'

function entry(
  id: string,
  title: string,
  description: string,
  keywords: string[]
): SettingsSearchEntry {
  return {
    title: translate(`auto.mcp.search.${id}.title`, title),
    description: translate(`auto.mcp.search.${id}.description`, description),
    keywords: keywords.flatMap((k) => translateSearchKeyword(`auto.mcp.search.keyword.${k}`, k))
  }
}

const getBaseEntries = createLocalizedCatalog(() => [
  entry('server', 'MCP server', 'Let AI agents work in Orca through the Model Context Protocol.', [
    'mcp',
    'model context protocol',
    'agent',
    'claude',
    'cursor'
  ]),
  entry('connect', 'Connect an agent', 'Server URL and setup steps for MCP clients.', [
    'mcp',
    'connect',
    'url',
    'claude code',
    'claude desktop',
    'cursor'
  ]),
  entry('sessions', 'Active sessions', 'MCP sessions currently connected to your account.', [
    'mcp',
    'session',
    'agent'
  ]),
  entry('tokens', 'Access tokens', 'Personal access tokens for MCP clients and scripts.', [
    'mcp',
    'token',
    'pat',
    'access'
  ]),
  entry('apps', 'Connected apps', 'Apps you authorized to use Orca through MCP.', [
    'mcp',
    'apps',
    'grant',
    'oauth',
    'revoke'
  ]),
  entry('approvals', 'Approvals', 'Review actions an agent asks permission to run.', [
    'mcp',
    'approval',
    'approve',
    'permission'
  ])
])

const getAdminEntries = createLocalizedCatalog(() => [
  entry('tools', 'Tool catalog', 'Tools exposed to MCP agents and their risk levels.', [
    'mcp',
    'tools',
    'risk'
  ]),
  entry('policy', 'Policies', 'Allow, require approval for, or deny MCP tools.', [
    'mcp',
    'policy',
    'kill switch'
  ]),
  entry('prompts', 'Prompts', 'Prompt templates offered to MCP clients.', ['mcp', 'prompts']),
  entry('audit', 'Audit log', 'History of agent tool calls and decisions.', [
    'mcp',
    'audit',
    'log'
  ]),
  entry('servers', 'External MCP servers', 'Review and manage external MCP servers.', [
    'mcp',
    'external',
    'servers'
  ])
])

export function getMcpPaneSearchEntries(isAdmin: boolean): SettingsSearchEntry[] {
  return isAdmin ? [...getBaseEntries(), ...getAdminEntries()] : getBaseEntries()
}
