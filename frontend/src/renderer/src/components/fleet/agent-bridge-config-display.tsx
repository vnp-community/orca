// Hiển thị Bridges config của orca-agent (CR-ORCA-004)
// Fields: Config.Bridges.ProductsAgentID + Config.Bridges.DocAgentID (SOL-ORCA-004 §2.1)

import type { FC } from 'react'

interface AgentBridgeConfig {
  productsAgentId?: string // ORCA_BRIDGES_PRODUCTS_AGENT_ID
  docAgentId?: string // ORCA_BRIDGES_DOC_AGENT_ID
}

interface Props {
  environmentId: string
  bridgeConfig?: AgentBridgeConfig
}

export const AgentBridgeConfigDisplay: FC<Props> = ({ environmentId, bridgeConfig }) => {
  if (bridgeConfig == null) return null

  const hasProductsAgent = Boolean(bridgeConfig.productsAgentId)
  const hasDocAgent = Boolean(bridgeConfig.docAgentId)

  return (
    <div className="agent-bridge-config" id={`agent-bridge-config-${environmentId}`}>
      <h4 className="agent-bridge-config__title">Agent Bridges</h4>

      <div className="agent-bridge-config__item">
        <span className="agent-bridge-config__label">Products Agent:</span>
        {hasProductsAgent ? (
          <code className="agent-bridge-config__value">{bridgeConfig.productsAgentId}</code>
        ) : (
          <span className="agent-bridge-config__empty">
            Not set — <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>
          </span>
        )}
      </div>

      <div className="agent-bridge-config__item">
        <span className="agent-bridge-config__label">Doc Agent:</span>
        {hasDocAgent ? (
          <code className="agent-bridge-config__value">{bridgeConfig.docAgentId}</code>
        ) : (
          <span className="agent-bridge-config__empty">
            Not set — <code>ORCA_BRIDGES_DOC_AGENT_ID</code>
          </span>
        )}
      </div>

      {!hasProductsAgent && (
        <div className="agent-bridge-config__hint">
          Without <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>, pis_* tools will not be exposed in
          the orca-agent MCP catalog.
        </div>
      )}
    </div>
  )
}
