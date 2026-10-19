# FE-IW-SOL-004 — Frontend: Agent Executor Config & Deploy Settings

**Solution ID:** FE-IW-SOL-004  
**CR Reference:** CR-ORCA-004 (orca-agent-executor-deploy-kit)  
**Backend Solution:** SOL-ORCA-004  
**TDD Reference:** TDD-FE-09 Onboarding & DevServer | TDD-FE-10 Fleet Management  
**Status:** ✅ Done — implemented 2026-10-19  
**Priority:** P1  
**Effort:** 1 ngày  
**Depends On:** SOL-ORCA-004 backend (config env override done)

---

## Alignment với Backend Solution

| Backend detail (SOL-ORCA-004) | Frontend implication |
|-------------------------------|----------------------|
| `Config.Bridges.ProductsAgentID` + `Config.Bridges.DocAgentID` — YAML + env override | Frontend display 2 bridge IDs, hint env var names |
| `Config.MCPProxy.URL` + `Config.MCPProxy.PeerToken` — YAML + env override | Frontend display URL (visible), mask PeerToken |
| Env var names: `ORCA_BRIDGES_PRODUCTS_AGENT_ID`, `ORCA_BRIDGES_DOC_AGENT_ID`, `ORCA_MCP_PROXY_URL`, `ORCA_MCP_PROXY_PEER_TOKEN`, `ORCA_API_URL` | UI hint text phải dùng đúng 4 tên này |
| `.env.example` có 6 entries total (BOOTSTRAP_TOKEN, API_URL, API_KEY, BRIDGES_PRODUCTS, BRIDGES_DOC, MCP_PROXY_URL, PEER_TOKEN) | Onboarding checklist show đủ 6 env vars |
| `PISToolCatalog()` trong `catalog_pis.go` — hàm helper `pisInputSchema(requiredField)` | Frontend health check verify đúng 5 tool names |
| `ExecuteLegacy()` routing: `pis_*` → `e.pisTool.Dispatch()` | Frontend health check test bằng cách call tool qua RPC |
| `SetPeerConfig(mcpProxyURL, peerToken)` — setter, không pass qua `ws.Config` | Frontend không cần biết init order này — backend detail |
| Compile verify: `GOWORK=off go build ./...` | Frontend verify: check `hasPisTools` sau restart |

---

## Giải pháp

### SOL-4A: Agent Bridge Config Display

```typescript
// src/renderer/src/components/fleet/agent-bridge-config-display.tsx

// Khớp với Config.Bridges struct (SOL-ORCA-004 §2.1)
interface AgentBridgeConfig {
  productsAgentId?: string  // Config.Bridges.ProductsAgentID
  docAgentId?: string       // Config.Bridges.DocAgentID
}

interface Props {
  environmentId: string
  bridgeConfig?: AgentBridgeConfig
}

export const AgentBridgeConfigDisplay: FC<Props> = ({ environmentId, bridgeConfig }) => {
  if (!bridgeConfig) return null

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
          <span className="agent-bridge-config__empty text-muted">
            Not set — <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>
          </span>
        )}
      </div>

      <div className="agent-bridge-config__item">
        <span className="agent-bridge-config__label">Doc Agent:</span>
        {hasDocAgent ? (
          <code className="agent-bridge-config__value">{bridgeConfig.docAgentId}</code>
        ) : (
          <span className="agent-bridge-config__empty text-muted">
            Not set — <code>ORCA_BRIDGES_DOC_AGENT_ID</code>
          </span>
        )}
      </div>

      {!hasProductsAgent && (
        <div className="agent-bridge-config__hint">
          Without <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>, pis_* tools will not be
          exposed in the orca-agent MCP catalog.
        </div>
      )}
    </div>
  )
}
```

---

### SOL-4B: PIS Health Check — Verify 5 Tools

```typescript
// src/renderer/src/components/fleet/agent-pis-health-check.tsx

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

  const allOk = ran && results.every((r) => r.status === 'ok')

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
          {/* Render in fixed order (PISToolCatalog() order — SOL-ORCA-004 §2.3) */}
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
                {result?.error && (
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
```

---

### SOL-4C: Deploy Env Config Table

```typescript
// src/renderer/src/components/fleet/agent-env-config-display.tsx

// Env vars từ SOL-ORCA-004 §2.8 .env.example — đúng thứ tự và tên
const ORCA_AGENT_ENV_VARS = [
  {
    key: 'ORCA_BOOTSTRAP_TOKEN',
    label: 'Bootstrap Token',
    required: true,
    sensitive: true,
    cr: null,
  },
  {
    key: 'ORCA_API_URL',
    label: 'Orca API URL',
    required: true,
    sensitive: false,
    cr: null,
  },
  {
    key: 'ORCA_API_KEY',
    label: 'Orca API Key',
    required: true,
    sensitive: true,
    cr: null,
  },
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
        const value = envSnapshot[key]
        return (
          <tr key={key} className={!required && !value ? 'env-row--optional-unset' : ''}>
            <td>
              <code>{key}</code>
              <br />
              <span className="env-label">{label}</span>
            </td>
            <td>
              {value ? (
                sensitive ? (
                  <span className="env-value--sensitive" aria-label="hidden">
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
            <td>{cr ? <span className="env-cr-ref">{cr}</span> : '—'}</td>
          </tr>
        )
      })}
    </tbody>
  </table>
)
```

---

### SOL-4D: Onboarding — PIKB Bridge Step (SSH Environments)

```typescript
// src/renderer/src/components/onboarding/pikb-bridge-step.tsx

interface PIKBBridgeConfig {
  productsAgentId: string        // → ORCA_BRIDGES_PRODUCTS_AGENT_ID
  docAgentId?: string            // → ORCA_BRIDGES_DOC_AGENT_ID (optional)
  mcpProxyUrl: string            // → ORCA_MCP_PROXY_URL (CR-ORCA-005 required)
  mcpProxyPeerToken: string      // → ORCA_MCP_PROXY_PEER_TOKEN (sensitive)
}

interface Props {
  onSkip: () => void
  onComplete: (config: PIKBBridgeConfig) => void
}

export const PIKBBridgeStep: FC<Props> = ({ onSkip, onComplete }) => {
  const [config, setConfig] = useState<PIKBBridgeConfig>({
    productsAgentId: '',
    docAgentId: '',
    mcpProxyUrl: 'http://mcp-proxy:3012',  // default từ docker-compose.yml (SOL-ORCA-004 §2.7)
    mcpProxyPeerToken: '',
  })

  const isValid = config.productsAgentId.trim() !== '' && config.mcpProxyUrl.trim() !== ''

  return (
    <div className="onboarding-step" id="onboarding-pikb-bridge-step">
      <h3>PIKB Knowledge Graph (Optional)</h3>
      <p>
        Enable pis_* tools for AI-powered KG queries. Requires orca-agent to be configured with
        bridge env vars.
      </p>

      <div className="onboarding-step__fields">
        <div className="form-field">
          <label htmlFor="onboarding-products-agent-id">
            Products Agent ID <span className="required">*</span>
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
            onChange={(e) => setConfig((c) => ({ ...c, docAgentId: e.target.value || undefined }))}
          />
          <span className="form-field__hint">
            → env: <code>ORCA_BRIDGES_DOC_AGENT_ID</code>
          </span>
        </div>

        <div className="form-field">
          <label htmlFor="onboarding-mcp-proxy-url">
            MCP Proxy URL <span className="required">*</span>
          </label>
          <input
            id="onboarding-mcp-proxy-url"
            type="text"
            placeholder="http://mcp-proxy:3012"
            value={config.mcpProxyUrl}
            onChange={(e) => setConfig((c) => ({ ...c, mcpProxyUrl: e.target.value }))}
          />
          <span className="form-field__hint">
            → env: <code>ORCA_MCP_PROXY_URL</code> (required for CR-ORCA-005 peer-invoke)
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
        <button
          className="onboarding-step__skip-btn"
          onClick={onSkip}
          id="onboarding-pikb-skip"
        >
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
```

---

## Files cần tạo/sửa

```
src/renderer/src/
└── components/
    ├── fleet/
    │   ├── agent-bridge-config-display.tsx     ← NEW
    │   ├── agent-pis-health-check.tsx           ← NEW
    │   └── agent-env-config-display.tsx         ← NEW (7 env vars từ .env.example)
    └── onboarding/
        └── pikb-bridge-step.tsx                 ← NEW (4 fields, 2 required)
```

---

## Tests

```typescript
// components/fleet/__tests__/agent-bridge-config-display.test.tsx
describe('AgentBridgeConfigDisplay', () => {
  it('render productsAgentId value khi configured')
  it('render env var hint "ORCA_BRIDGES_PRODUCTS_AGENT_ID" khi absent')
  it('render warning hint khi productsAgentId absent (pis_* sẽ không available)')
})

// components/fleet/__tests__/agent-pis-health-check.test.tsx
describe('AgentPISHealthCheck', () => {
  it('render đúng 5 tools theo thứ tự PIS_TOOL_NAMES')
  it('✅ icon khi status=ok')
  it('❌ icon khi status=missing')
  it('Run Check button disabled khi running=true')
  it('"All tools available" summary khi tất cả ok')
})

// components/fleet/__tests__/agent-env-config-display.test.tsx
describe('AgentEnvConfigDisplay', () => {
  it('render đúng 7 env vars')
  it('mask sensitive values bằng ●●●●●●')
  it('"Not set" với warning khi required var absent')
  it('CR column hiển thị "CR-ORCA-004" cho BRIDGES vars')
})

// components/onboarding/__tests__/pikb-bridge-step.test.tsx
describe('PIKBBridgeStep', () => {
  it('Continue disabled khi productsAgentId empty')
  it('Continue disabled khi mcpProxyUrl empty')
  it('Continue enabled khi cả hai có giá trị')
  it('gọi onSkip khi click Skip')
  it('gọi onComplete với full config khi click Configure')
  it('4 inputs có unique IDs')
  it('peer token field có type=password')
})
```

---

## Acceptance Criteria (Frontend)

| AC | Mô tả | Khớp với Backend AC |
|----|-------|---------------------|
| AC-F1 | Hint text dùng đúng env var names: `ORCA_BRIDGES_PRODUCTS_AGENT_ID`, `ORCA_BRIDGES_DOC_AGENT_ID` | SOL-ORCA-004 §2.1 Config struct |
| AC-F2 | Health check verify đúng 5 tool names từ `PIS_TOOL_NAMES` | SOL-ORCA-004 §2.3 `PISToolCatalog()` |
| AC-F3 | Env table có đủ 7 rows từ `.env.example` | SOL-ORCA-004 §2.8 `.env.example` |
| AC-F4 | Sensitive vars (BOOTSTRAP_TOKEN, API_KEY, PEER_TOKEN) masked | SOL-ORCA-004 §2.8 — tokens không log |
| AC-F5 | Onboarding form require cả `productsAgentId` lẫn `mcpProxyUrl` | SOL-ORCA-005 — mcpProxyUrl blocking dep |
| AC-F6 | Default `mcpProxyUrl = "http://mcp-proxy:3012"` | SOL-ORCA-004 §2.7 docker-compose default |
