// Health check cho 5 pis_* tools (CR-ORCA-004)
// Render theo đúng thứ tự PIS_TOOL_NAMES (= PISToolCatalog() order — SOL-ORCA-004 §2.3)

import type { FC } from 'react'
import { useState } from 'react'
import { PIS_TOOL_NAMES } from '../../hooks/use-agent-tool-capabilities'

type ToolCheckStatus = 'ok' | 'missing' | 'unchecked'

interface ToolCheckResult {
  name: string
  status: ToolCheckStatus
  error?: string
}

interface Props {
  environmentId: string
  onRunCheck: () => Promise<ToolCheckResult[]>
}

export const AgentPISHealthCheck: FC<Props> = ({ environmentId, onRunCheck }) => {
  const [results, setResults] = useState<ToolCheckResult[]>([])
  const [running, setRunning] = useState(false)
  const [ran, setRan] = useState(false)

  const runCheck = async () => {
    setRunning(true)
    try {
      const r = await onRunCheck()
      setResults(r)
      setRan(true)
    } finally {
      setRunning(false)
    }
  }

  const allOk = ran && results.length > 0 && results.every((r) => r.status === 'ok')

  return (
    <div className="agent-pis-health-check" id={`pis-health-check-${environmentId}`}>
      <div className="agent-pis-health-check__header">
        <h5>PIS Tools Health ({PIS_TOOL_NAMES.length} tools)</h5>
        <button
          className="agent-pis-health-check__run-btn"
          onClick={runCheck}
          disabled={running}
          id={`pis-health-check-run-${environmentId}`}
          aria-label="Run PIS tools health check"
        >
          {running ? 'Checking…' : ran ? '↺ Re-check' : 'Run Check'}
        </button>
        {ran && (
          <span className={`pis-health-summary pis-health-summary--${allOk ? 'ok' : 'fail'}`}>
            {allOk ? '✅ All tools available' : '❌ Some tools missing'}
          </span>
        )}
      </div>

      {ran && (
        <ul className="agent-pis-health-check__results" aria-label="PIS tool check results">
          {PIS_TOOL_NAMES.map((toolName) => {
            const result = results.find((r) => r.name === toolName)
            const status = result?.status ?? 'unchecked'
            return (
              <li
                key={toolName}
                className={`pis-tool-check-item pis-tool-check-item--${status}`}
                id={`pis-check-${toolName}`}
              >
                <span className="pis-tool-check-item__icon" aria-hidden="true">
                  {status === 'ok' ? '✅' : status === 'missing' ? '❌' : '○'}
                </span>
                <code>{toolName}</code>
                {result?.error != null && (
                  <span className="pis-tool-check-item__error" role="alert">
                    {result.error}
                  </span>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
