import type { McpTabId } from '../store/slices/mcp-slice'

const TAB_IDS: readonly McpTabId[] = [
  'connect',
  'apps',
  'tokens',
  'approvals',
  'tools',
  'policy',
  'prompts',
  'audit',
  'servers'
]

export type McpDeepLinkTarget = { tab: McpTabId; focusId?: string }

export function parseMcpDeepLinkParams(params: URLSearchParams): McpDeepLinkTarget | null {
  if (params.get('section') !== 'mcp') {
    return null
  }
  const rawTab = params.get('tab')
  const tab = (TAB_IDS as readonly string[]).includes(rawTab ?? '')
    ? (rawTab as McpTabId)
    : 'connect'
  const focusId = params.get('approval') ?? params.get('token') ?? params.get('app') ?? undefined
  return focusId ? { tab, focusId } : { tab }
}

/** Parses `/?section=mcp&tab=…&approval=…` (any pathname, incl. legacy `/settings`). */
export function parseMcpDeepLink(
  loc: Pick<Location, 'pathname' | 'search'>
): McpDeepLinkTarget | null {
  return parseMcpDeepLinkParams(new URLSearchParams(loc.search))
}
