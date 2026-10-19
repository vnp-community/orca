// Bảng 7 env vars từ .env.example (SOL-ORCA-004 §2.8)
// Sensitive vars masked với ●●●●●●

import type { FC } from 'react'

// 7 env vars theo đúng thứ tự .env.example (SOL-ORCA-004 §2.8)
const ORCA_AGENT_ENV_VARS = [
  {
    key: 'ORCA_BOOTSTRAP_TOKEN',
    label: 'Bootstrap Token',
    required: true,
    sensitive: true,
    cr: null,
  },
  { key: 'ORCA_API_URL', label: 'Orca API URL', required: true, sensitive: false, cr: null },
  { key: 'ORCA_API_KEY', label: 'Orca API Key', required: true, sensitive: true, cr: null },
  {
    key: 'ORCA_BRIDGES_PRODUCTS_AGENT_ID',
    label: 'Products Agent ID',
    required: false,
    sensitive: false,
    cr: 'CR-ORCA-004',
  },
  {
    key: 'ORCA_BRIDGES_DOC_AGENT_ID',
    label: 'Doc Agent ID',
    required: false,
    sensitive: false,
    cr: 'CR-ORCA-004',
  },
  {
    key: 'ORCA_MCP_PROXY_URL',
    label: 'MCP Proxy URL',
    required: false,
    sensitive: false,
    cr: 'CR-ORCA-005',
  },
  {
    key: 'ORCA_MCP_PROXY_PEER_TOKEN',
    label: 'Peer Token',
    required: false,
    sensitive: true,
    cr: 'CR-ORCA-005',
  },
] as const

type EnvKey = (typeof ORCA_AGENT_ENV_VARS)[number]['key']

interface AgentEnvSnapshot {
  [key: string]: string | undefined
}

interface Props {
  environmentId: string
  envSnapshot: AgentEnvSnapshot
}

export const AgentEnvConfigDisplay: FC<Props> = ({ environmentId, envSnapshot }) => (
  <table
    className="agent-env-config-table"
    id={`agent-env-config-${environmentId}`}
    aria-label="orca-agent environment variables"
  >
    <thead>
      <tr>
        <th scope="col">Variable</th>
        <th scope="col">Value</th>
        <th scope="col">Required</th>
        <th scope="col">CR</th>
      </tr>
    </thead>
    <tbody>
      {ORCA_AGENT_ENV_VARS.map(({ key, label, required, sensitive, cr }) => {
        const value = envSnapshot[key as EnvKey]
        return (
          <tr key={key}>
            <td>
              <code>{key}</code>
              <br />
              <span className="env-label">{label}</span>
            </td>
            <td>
              {value != null ? (
                sensitive ? (
                  <span className="env-value--sensitive" aria-label="hidden value">
                    ●●●●●●
                  </span>
                ) : (
                  <code>{value}</code>
                )
              ) : (
                <span className={`env-value--missing ${required ? 'env-value--required' : ''}`}>
                  {required ? '⚠ Not set' : 'Not set'}
                </span>
              )}
            </td>
            <td>{required ? '✓' : 'Optional'}</td>
            <td>{cr != null ? <span className="env-cr-ref">{cr}</span> : '—'}</td>
          </tr>
        )
      })}
    </tbody>
  </table>
)
