// Hook query danh sách tools từ orca-agent catalog (CR-ORCA-001)
// PIS_TOOL_NAMES: 5 tools cố định từ PISToolCatalog() — SOL-ORCA-001 §2.2

import { useCallback, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'

// 5 tool names cố định từ backend PISToolCatalog() (SOL-ORCA-001 §2.2)
// Chỉ present trong catalog khi ORCA_BRIDGES_PRODUCTS_AGENT_ID được set
export const PIS_TOOL_NAMES = [
  'pis_query_feature_context',
  'pis_query_crs',
  'pis_query_hld',
  'pis_query_adr',
  'pis_kg_search',
] as const

export type PISTool = (typeof PIS_TOOL_NAMES)[number]

export interface AgentToolCapability {
  name: string
  description: string
}

export function useAgentToolCapabilities(environmentId?: string) {
  const [tools, setTools] = useState<AgentToolCapability[]>([])
  const [loading, setLoading] = useState(false)

  const fetchCapabilities = useCallback(async () => {
    setLoading(true)
    try {
      const target =
        environmentId != null
          ? { kind: 'environment' as const, environmentId }
          : { kind: 'local' as const }

      const result = await callRuntimeRpc<{ tools: AgentToolCapability[] }>(
        target,
        'agent.list_tools',
        {}
      )
      setTools(result.tools)
    } finally {
      setLoading(false)
    }
  }, [environmentId])

  // pis_* tools chỉ present khi ORCA_BRIDGES_PRODUCTS_AGENT_ID set (SOL-ORCA-001)
  const hasPisTools = tools.some((t) => PIS_TOOL_NAMES.includes(t.name as PISTool))

  return { tools, loading, fetchCapabilities, hasPisTools }
}
