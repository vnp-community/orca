import { describe, expect, it } from 'vitest'
import {
  MCP_TOKEN_PLACEHOLDER,
  buildMcpSnippet,
  isInsecureMcpUrl,
  type McpClientKind
} from './mcp-connect-snippets'

const KINDS: McpClientKind[] = ['claude-code', 'claude-desktop', 'cursor']

describe('buildMcpSnippet', () => {
  it('produces parseable JSON even for odd URLs', () => {
    const url = 'https://orca.example.com/mcp?a="b"&c=\\d'
    for (const kind of ['claude-desktop', 'cursor'] as const) {
      const s = buildMcpSnippet(kind, { resourceUrl: url })
      expect(() => JSON.parse(s.withOAuth)).not.toThrow()
      expect(() => JSON.parse(s.withToken)).not.toThrow()
      expect(s.withOAuth).toContain(JSON.stringify(url).slice(1, -1))
    }
  })
  it('always uses the placeholder for tokens', () => {
    for (const kind of KINDS) {
      const s = buildMcpSnippet(kind, { resourceUrl: 'https://x.test/mcp' })
      expect(s.withToken).toContain(MCP_TOKEN_PLACEHOLDER)
      expect(s.withOAuth).not.toContain(MCP_TOKEN_PLACEHOLDER)
    }
  })
  it('builds the claude code command', () => {
    expect(buildMcpSnippet('claude-code', { resourceUrl: 'https://x.test/mcp' }).withOAuth).toBe(
      'claude mcp add --transport http orca https://x.test/mcp'
    )
  })
  it('adds the connectors hint for Claude Desktop only', () => {
    expect(buildMcpSnippet('claude-desktop', { resourceUrl: 'https://x/mcp' }).notes).toHaveLength(
      1
    )
    expect(buildMcpSnippet('cursor', { resourceUrl: 'https://x/mcp' }).notes).toHaveLength(0)
  })
})

describe('isInsecureMcpUrl', () => {
  it('classifies urls', () => {
    expect(isInsecureMcpUrl('http://localhost:8081/mcp')).toBe(false)
    expect(isInsecureMcpUrl('https://orca.example.com/mcp')).toBe(false)
    expect(isInsecureMcpUrl('http://orca.example.com/mcp')).toBe(true)
    expect(isInsecureMcpUrl('garbage')).toBe(true)
  })
})
