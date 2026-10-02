import { describe, expect, it } from 'vitest'
import {
  McpRpcError,
  isMcpDisabledError,
  isMcpNotImplementedError,
  parseMcpError
} from './runtime-mcp-error'

describe('parseMcpError', () => {
  it.each([
    ['MCP_NOT_FOUND: x', 'MCP_NOT_FOUND', 'x'],
    ['rpc error: code = NotFound desc = MCP_NOT_FOUND: x', 'MCP_NOT_FOUND', 'x'],
    ['MCP_PROMPT_INVALID: name: too long', 'MCP_PROMPT_INVALID', 'name: too long'],
    ['MCP_BRAND_NEW: hi', null, 'hi'],
    ['something else', null, 'something else']
  ])('%s', (message, code, detail) => {
    const e = parseMcpError(new Error(message))
    expect(e).toBeInstanceOf(McpRpcError)
    expect(e.code).toBe(code)
    expect(e.detail).toBe(detail)
  })

  it('accepts non-Error values and is idempotent', () => {
    expect(parseMcpError('MCP_DISABLED: off').code).toBe('MCP_DISABLED')
    const e = parseMcpError(new Error('MCP_DISABLED: off'))
    expect(parseMcpError(e)).toBe(e)
  })

  it('classifies disabled / not implemented', () => {
    expect(isMcpDisabledError(parseMcpError(new Error('MCP_DISABLED: off')))).toBe(true)
    expect(isMcpDisabledError(new Error('MCP_DISABLED: off'))).toBe(false)
    expect(
      isMcpNotImplementedError(new Error('channel "mcp.server.info" is not yet implemented'))
    ).toBe(true)
  })
})
