import { useCallback } from 'react'
import type {
  McpExternalServer,
  McpExternalServerUpsertInput,
  McpRpcResult
} from '../../../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { McpRpcError, parseMcpError } from '@/runtime/runtime-mcp-error'
import { useMcpQuery, type McpQuery } from '@/hooks/useMcpQuery'

const NO_SERVERS: McpExternalServer[] = []

export type McpProbeResult = McpRpcResult<'mcp.externalServer.probe'>

export type McpExternalServersApi = McpQuery<McpExternalServer[]> & {
  upsert: (input: McpExternalServerUpsertInput) => Promise<McpExternalServer>
  /** Plaintext is sent once and never retained here; errors are scrubbed of the value. */
  setSecret: (p: {
    serverId: string
    kind: 'env' | 'header'
    name: string
    value: string
  }) => Promise<void>
  remove: (serverId: string) => Promise<void>
  probe: (serverId: string) => Promise<McpProbeResult>
  review: (
    serverId: string,
    decision: 'approve' | 'reject',
    toolsDigest: string
  ) => Promise<McpExternalServer>
}

function replaceServer(list: McpExternalServer[], s: McpExternalServer): McpExternalServer[] {
  return list.some((x) => x.id === s.id) ? list.map((x) => (x.id === s.id ? s : x)) : [...list, s]
}

/** Tab-local registry list; the server computes status/digest/hasSecret, so mutations patch from its reply. */
export function useExternalServers(): McpExternalServersApi {
  const query = useMcpQuery('mcp.externalServer.list', {}, NO_SERVERS, {
    refetchOnFocus: true,
    quietReload: true
  })
  const { setData, reload } = query

  const guard = useCallback(
    async <T>(run: () => Promise<T>): Promise<T> => {
      try {
        return await run()
      } catch (e) {
        const err = parseMcpError(e)
        if (err.code === 'MCP_NOT_FOUND') {
          reload()
        }
        throw err
      }
    },
    [reload]
  )

  const upsert = useCallback(
    (input: McpExternalServerUpsertInput) =>
      guard(async () => {
        const saved = await mcpClient.call('mcp.externalServer.upsert', input)
        setData((prev) => replaceServer(prev, saved))
        return saved
      }),
    [guard, setData]
  )

  const setSecret = useCallback<McpExternalServersApi['setSecret']>(
    async (p) => {
      try {
        await mcpClient.call('mcp.externalServer.setSecret', p)
      } catch (e) {
        const err = parseMcpError(e)
        const detail = p.value ? err.detail.split(p.value).join('[redacted]') : err.detail
        throw new McpRpcError(err.code, detail)
      }
      // hasSecret is re-read from the server, never inferred locally.
      reload()
    },
    [reload]
  )

  const remove = useCallback(
    (serverId: string) =>
      guard(async () => {
        await mcpClient.call('mcp.externalServer.delete', { serverId })
        setData((prev) => prev.filter((s) => s.id !== serverId))
      }),
    [guard, setData]
  )

  const probe = useCallback(
    (serverId: string) => guard(() => mcpClient.call('mcp.externalServer.probe', { serverId })),
    [guard]
  )

  const review = useCallback(
    (serverId: string, decision: 'approve' | 'reject', toolsDigest: string) =>
      guard(async () => {
        const saved = await mcpClient.call('mcp.externalServer.review', {
          serverId,
          decision,
          toolsDigest
        })
        setData((prev) => replaceServer(prev, saved))
        return saved
      }),
    [guard, setData]
  )

  return { ...query, upsert, setSecret, remove, probe, review }
}
