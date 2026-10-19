// Badge hiển thị khi orca-agent có pis_* tools active (CR-ORCA-001)

import type { FC } from 'react'
import { useEffect } from 'react'
import { useAgentToolCapabilities } from '../../hooks/use-agent-tool-capabilities'

interface Props {
  environmentId?: string
}

export const AgentCapabilityBadge: FC<Props> = ({ environmentId }) => {
  const { hasPisTools, fetchCapabilities } = useAgentToolCapabilities(environmentId)

  // Fetch catalog on mount — lazy, không poll
  useEffect(() => {
    fetchCapabilities()
  }, [fetchCapabilities])

  if (!hasPisTools) return null

  return (
    <span
      className="agent-capability-badge agent-capability-badge--kg"
      title="PIKB Knowledge Graph integration enabled (pis_* tools active)"
      id={`agent-kg-badge-${environmentId ?? 'local'}`}
    >
      KG
    </span>
  )
}
