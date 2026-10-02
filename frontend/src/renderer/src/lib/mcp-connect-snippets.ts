import type { McpServerInfo } from '../../../shared/mcp-types'

// Why: a real token must never be interpolated into a copyable snippet (CONTRACT C6).
export const MCP_TOKEN_PLACEHOLDER = '<YOUR_ACCESS_TOKEN>'

export type McpClientKind = 'claude-code' | 'claude-desktop' | 'cursor'
export type McpSnippet = {
  kind: McpClientKind
  language: 'bash' | 'json'
  withOAuth: string
  withToken: string
  /** i18n sub-keys under auto.mcp.connect for per-client notes. */
  notes: string[]
}

const json = (v: unknown): string => JSON.stringify(v, null, 2)
const bearer = `Bearer ${MCP_TOKEN_PLACEHOLDER}`

// Third-party client syntax: unverified against current client docs, keep it isolated here.
export function buildMcpSnippet(
  kind: McpClientKind,
  info: Pick<McpServerInfo, 'resourceUrl'>
): McpSnippet {
  const url = info.resourceUrl
  switch (kind) {
    case 'claude-code':
      return {
        kind,
        language: 'bash',
        withOAuth: `claude mcp add --transport http orca ${url}`,
        withToken: `claude mcp add --transport http orca ${url} --header "Authorization: ${bearer}"`,
        notes: []
      }
    case 'cursor':
      return {
        kind,
        language: 'json',
        withOAuth: json({ mcpServers: { orca: { url } } }),
        withToken: json({
          mcpServers: { orca: { url, headers: { Authorization: bearer } } }
        }),
        notes: []
      }
    case 'claude-desktop':
      return {
        kind,
        language: 'json',
        withOAuth: json({
          mcpServers: {
            orca: { command: 'npx', args: ['-y', 'mcp-remote', url] }
          }
        }),
        withToken: json({
          mcpServers: {
            orca: {
              command: 'npx',
              args: ['-y', 'mcp-remote', url, '--header', `Authorization: ${bearer}`]
            }
          }
        }),
        notes: ['claudeDesktopConnectorsHint']
      }
  }
}

export function isInsecureMcpUrl(url: string): boolean {
  try {
    const u = new URL(url)
    return u.protocol !== 'https:' && !['localhost', '127.0.0.1', '[::1]'].includes(u.hostname)
  } catch {
    return true
  }
}

export type McpCliShell = 'posix' | 'powershell'

// Why: the snippet never contains the secret; users paste it into an env var themselves.
export function buildTokenCliSnippet(shell: McpCliShell, url: string): string {
  return shell === 'powershell'
    ? [
        '# Windows (PowerShell)',
        "$env:ORCA_MCP_TOKEN = '<paste token here>'",
        `claude mcp add --transport http orca ${url} --header "Authorization: Bearer $env:ORCA_MCP_TOKEN"`
      ].join('\n')
    : [
        '# macOS / Linux',
        "export ORCA_MCP_TOKEN='<paste token here>'",
        `claude mcp add --transport http orca ${url} --header "Authorization: Bearer $ORCA_MCP_TOKEN"`
      ].join('\n')
}
