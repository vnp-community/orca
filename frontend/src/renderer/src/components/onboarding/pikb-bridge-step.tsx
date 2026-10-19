// Onboarding step để configure PIKB bridge (SSH environments, CR-ORCA-004)
// Default mcpProxyUrl từ docker-compose default (SOL-ORCA-004 §2.7)

import type { FC } from 'react'
import { useState } from 'react'

interface PIKBBridgeConfig {
  productsAgentId: string // → ORCA_BRIDGES_PRODUCTS_AGENT_ID
  docAgentId?: string // → ORCA_BRIDGES_DOC_AGENT_ID
  mcpProxyUrl: string // → ORCA_MCP_PROXY_URL (required — CR-ORCA-005 dep)
  mcpProxyPeerToken: string // → ORCA_MCP_PROXY_PEER_TOKEN
}

interface Props {
  onSkip: () => void
  onComplete: (config: PIKBBridgeConfig) => void
}

export const PIKBBridgeStep: FC<Props> = ({ onSkip, onComplete }) => {
  const [config, setConfig] = useState<PIKBBridgeConfig>({
    productsAgentId: '',
    docAgentId: '',
    // Default từ docker-compose: ORCA_MCP_PROXY_URL:-http://mcp-proxy:3012 (SOL-ORCA-004 §2.7)
    mcpProxyUrl: 'http://mcp-proxy:3012',
    mcpProxyPeerToken: '',
  })

  // Cả hai required vì mcpProxyUrl là dep của CR-ORCA-005
  const isValid = config.productsAgentId.trim() !== '' && config.mcpProxyUrl.trim() !== ''

  return (
    <div className="onboarding-step" id="onboarding-pikb-bridge-step">
      <h3>PIKB Knowledge Graph (Optional)</h3>
      <p>
        Enable pis_* tools cho AI agent query KG. Cần orca-agent server được cấu hình bridge env
        vars.
      </p>

      <div className="onboarding-step__fields">
        <div className="form-field">
          <label htmlFor="onboarding-products-agent-id">
            Products Agent ID <span className="required" aria-hidden="true">*</span>
          </label>
          <input
            id="onboarding-products-agent-id"
            type="text"
            placeholder="products-agent-prod"
            value={config.productsAgentId}
            onChange={(e) => setConfig((c) => ({ ...c, productsAgentId: e.target.value }))}
          />
          <span className="form-field__hint">
            → env: <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>
          </span>
        </div>

        <div className="form-field">
          <label htmlFor="onboarding-doc-agent-id">Doc Agent ID</label>
          <input
            id="onboarding-doc-agent-id"
            type="text"
            placeholder="doc-agent-prod"
            value={config.docAgentId ?? ''}
            onChange={(e) =>
              setConfig((c) => ({ ...c, docAgentId: e.target.value || undefined }))
            }
          />
          <span className="form-field__hint">
            → env: <code>ORCA_BRIDGES_DOC_AGENT_ID</code>
          </span>
        </div>

        <div className="form-field">
          <label htmlFor="onboarding-mcp-proxy-url">
            MCP Proxy URL <span className="required" aria-hidden="true">*</span>
          </label>
          <input
            id="onboarding-mcp-proxy-url"
            type="text"
            placeholder="http://mcp-proxy:3012"
            value={config.mcpProxyUrl}
            onChange={(e) => setConfig((c) => ({ ...c, mcpProxyUrl: e.target.value }))}
          />
          <span className="form-field__hint">
            → env: <code>ORCA_MCP_PROXY_URL</code> (required cho peer-invoke CR-ORCA-005)
          </span>
        </div>

        <div className="form-field">
          <label htmlFor="onboarding-peer-token">Peer Token</label>
          <input
            id="onboarding-peer-token"
            type="password"
            placeholder="From workplace admin"
            value={config.mcpProxyPeerToken}
            onChange={(e) => setConfig((c) => ({ ...c, mcpProxyPeerToken: e.target.value }))}
          />
          <span className="form-field__hint">
            → env: <code>ORCA_MCP_PROXY_PEER_TOKEN</code>
          </span>
        </div>
      </div>

      <div className="onboarding-step__actions">
        <button className="onboarding-step__skip-btn" onClick={onSkip} id="onboarding-pikb-skip">
          Skip for now
        </button>
        <button
          className="onboarding-step__continue-btn"
          disabled={!isValid}
          onClick={() => onComplete(config)}
          id="onboarding-pikb-continue"
        >
          Configure
        </button>
      </div>
    </div>
  )
}
