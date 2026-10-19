// Surface actionable error khi pis_* tool chưa configured (CR-ORCA-001)
// Detect exact error string từ backend ExecuteLegacy() (SOL-ORCA-001 §2.3):
//   "pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID"

import type { FC } from 'react'

interface Props {
  toolName: string
  errorMessage: string
}

export const PISTooLNotConfiguredError: FC<Props> = ({ toolName, errorMessage }) => {
  // Detect exact backend error string (SOL-ORCA-001 §2.3)
  const isConfigError = errorMessage.includes('ORCA_BRIDGES_PRODUCTS_AGENT_ID')

  if (!isConfigError) return null

  return (
    <div className="pis-tool-config-error" role="alert" id={`pis-config-error-${toolName}`}>
      <span aria-hidden="true">⚙️</span>
      <strong>KG Integration Not Configured</strong>
      <p>
        Tool <code>{toolName}</code> requires PIKB Knowledge Graph integration.
      </p>
      <p className="pis-tool-config-error__fix">
        Set <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code> in the orca-agent deploy config to
        enable pis_* tools.
      </p>
    </div>
  )
}
