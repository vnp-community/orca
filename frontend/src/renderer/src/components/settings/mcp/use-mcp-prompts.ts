import { useCallback } from 'react'
import type { McpPrompt } from '../../../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useMcpQuery, type McpQuery } from '@/hooks/useMcpQuery'

const NO_PROMPTS: McpPrompt[] = []

export function sortMcpPrompts(prompts: readonly McpPrompt[]): McpPrompt[] {
  return [...prompts].sort(
    (a, b) => Number(b.builtin) - Number(a.builtin) || a.name.localeCompare(b.name)
  )
}

export type McpPromptsApi = McpQuery<McpPrompt[]> & {
  save: (draft: McpPrompt) => Promise<McpPrompt>
  remove: (id: string) => Promise<void>
  /** Re-reads the list, patches it, and returns the stored copy of one prompt. */
  fetchLatest: (id: string) => Promise<McpPrompt | undefined>
}

/** Tab-local prompt list: save/remove patch the list in place; NOT_FOUND triggers a reload. */
export function useMcpPrompts(): McpPromptsApi {
  const query = useMcpQuery('mcp.admin.prompt.list', undefined, NO_PROMPTS, {
    refetchOnFocus: true,
    quietReload: true
  })
  const { setData, reload } = query

  const save = useCallback(
    async (draft: McpPrompt): Promise<McpPrompt> => {
      try {
        const saved = await mcpClient.call('mcp.admin.prompt.upsert', draft)
        setData((prev) => [...prev.filter((p) => p.id !== saved.id), saved])
        return saved
      } catch (e) {
        if (e instanceof McpRpcError && e.code === 'MCP_NOT_FOUND') {
          reload()
        }
        throw e
      }
    },
    [setData, reload]
  )

  const remove = useCallback(
    async (id: string): Promise<void> => {
      try {
        await mcpClient.call('mcp.admin.prompt.delete', { promptId: id })
        setData((prev) => prev.filter((p) => p.id !== id))
      } catch (e) {
        if (e instanceof McpRpcError && e.code === 'MCP_NOT_FOUND') {
          reload()
        }
        throw e
      }
    },
    [setData, reload]
  )

  const fetchLatest = useCallback(
    async (id: string): Promise<McpPrompt | undefined> => {
      const list = await mcpClient.call('mcp.admin.prompt.list')
      setData(() => list)
      return list.find((p) => p.id === id)
    },
    [setData]
  )

  return { ...query, save, remove, fetchLatest }
}
