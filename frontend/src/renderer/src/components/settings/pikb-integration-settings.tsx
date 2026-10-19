// Settings section hiển thị PIKB KG status (SSH environments only)
// Chỉ render khi environmentId present (server-side config)

import type { FC } from 'react'
import { useEffect } from 'react'
import {
  useAgentToolCapabilities,
  PIS_TOOL_NAMES,
} from '../../hooks/use-agent-tool-capabilities'
import type { PISTool } from '../../hooks/use-agent-tool-capabilities'

interface StatusIndicatorProps {
  active: boolean
  label: string
}

const StatusIndicator: FC<StatusIndicatorProps> = ({ active, label }) => (
  <span className={`status-indicator status-indicator--${active ? 'active' : 'inactive'}`}>
    <span aria-hidden="true">{active ? '🟢' : '⚪'}</span> {label}
  </span>
)

interface Props {
  environmentId: string
}

export const PIKBIntegrationSettings: FC<Props> = ({ environmentId }) => {
  const { hasPisTools, tools, fetchCapabilities } = useAgentToolCapabilities(environmentId)
  const pisTools = tools.filter((t) => PIS_TOOL_NAMES.includes(t.name as PISTool))

  useEffect(() => {
    fetchCapabilities()
  }, [fetchCapabilities])

  return (
    <div className="settings-section" id="pikb-integration-settings">
      <h3>PIKB Knowledge Graph Integration</h3>
      <div className="settings-section__status">
        <StatusIndicator active={hasPisTools} label="KG Tools" />
        <button
          onClick={fetchCapabilities}
          className="settings-section__refresh-btn"
          id="pikb-integration-refresh"
          title="Refresh tool catalog"
        >
          ↺ Refresh
        </button>
      </div>

      {hasPisTools ? (
        <ul className="pis-tool-list" aria-label="Active pis_* tools">
          {pisTools.map((t) => (
            <li key={t.name}>
              <code>{t.name}</code>
              {t.description.length > 0 && <span> — {t.description}</span>}
            </li>
          ))}
        </ul>
      ) : (
        <div className="settings-section__hint">
          <p>Không có pis_* tools. Để enable PIKB KG integration:</p>
          <ol>
            <li>
              Set <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod</code>
            </li>
            <li>
              Set <code>ORCA_MCP_PROXY_URL</code> và <code>ORCA_MCP_PROXY_PEER_TOKEN</code>
            </li>
            <li>Restart orca-agent và click Refresh</li>
          </ol>
        </div>
      )}
    </div>
  )
}
