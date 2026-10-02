import type { McpEvent, McpRpcArgs, McpRpcMethod, McpRpcResult } from '../../../shared/mcp-types'
import { parseMcpError } from './runtime-mcp-error'

// Single entry point for all MCP UI code: errors are always parsed into McpRpcError.
export const mcpClient = {
  call: async <M extends McpRpcMethod>(
    method: M,
    ...params: McpRpcArgs<M>
  ): Promise<McpRpcResult<M>> => {
    try {
      return await window.api.mcp.call(method, ...params)
    } catch (e) {
      throw parseMcpError(e)
    }
  },
  subscribeEvents: (onEvent: (e: McpEvent) => void, onClose?: () => void): (() => void) =>
    window.api.mcp.subscribeEvents(onEvent, onClose),
  isBridgeAvailable: (): boolean =>
    typeof window !== 'undefined' && typeof window.api?.mcp === 'object' && window.api.mcp !== null
}
