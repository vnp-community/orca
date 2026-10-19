# Phase 2 — PIS Tool Hook & Fleet/Settings UI

**Phase:** 2/6  
**Priority:** P1  
**Effort:** ~1–1.5 ngày  
**Depends on:** Phase 1 (mcp-proxy-types.ts, use-mcp-proxy-status.ts)  
**Unlocks:** Phase 3  
**Source solutions:** FE-IW-SOL-001, FE-IW-SOL-004

---

## Mục tiêu Phase

Tạo foundation cho pis_* tool awareness: hook phát hiện PIS tools trong catalog, KG renderer, badge, và toàn bộ Fleet/Onboarding UI để quản lý deploy config. Các task trong phase này **có thể chạy song song** (T05–T10 không phụ thuộc nhau).

---

## T05 — Tạo `use-agent-tool-capabilities.ts`

```
Action:    NEW
File:      src/renderer/src/hooks/use-agent-tool-capabilities.ts
Solution:  FE-IW-SOL-001 §SOL-1A
```

**Code cần tạo:**

```typescript
// src/renderer/src/hooks/use-agent-tool-capabilities.ts
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
      const target = environmentId
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
```

**Verify:** `PIS_TOOL_NAMES` export đúng 5 elements. `hasPisTools` computed từ array.

---

## T06 — Tạo `kg-query-result-renderer.tsx`

```
Action:    NEW
File:      src/renderer/src/components/agent/kg-query-result-renderer.tsx
Solution:  FE-IW-SOL-001 §SOL-1B
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/agent/kg-query-result-renderer.tsx
// Render structured KG node results từ pis_* tool calls (CR-ORCA-001)
// rawResult là json.RawMessage parsed từ backend PISTool.Dispatch()

import type { FC } from 'react'
import type { PISTool } from '../../hooks/use-agent-tool-capabilities'

// CRS status values từ backend validation (SOL-ORCA-003 §2.2)
type CRSStatus = 'pending' | 'in-progress' | 'done' | 'cancelled'

interface CRSNode {
  id: string
  status: CRSStatus
  layer?: string
  feature_ref?: string
  relationships?: Array<{ type: string; target: string }>
}

interface KGSearchResult {
  results: Array<{ node_type: string; id: string; description?: string }>
}

interface Props {
  toolName: string  // 1 trong 5 PIS_TOOL_NAMES
  rawResult: unknown // json.RawMessage parsed
}

// KGNodeTypeBadge — badge màu theo loại node
const KGNodeTypeBadge: FC<{ type: string }> = ({ type }) => (
  <span className={`kg-badge kg-badge--${type}`}>{type.toUpperCase()}</span>
)

// KGStatusBadge — badge status với màu semantic
const KGStatusBadge: FC<{ status: string }> = ({ status }) => (
  <span className={`kg-status-badge kg-status-badge--${status}`}>{status}</span>
)

// KGRelationshipChip — chip hiển thị relationship edge
const KGRelationshipChip: FC<{ type: string; target: string }> = ({ type, target }) => (
  <span className="kg-relationship-chip">
    <span className="kg-relationship-chip__type">{type}</span>
    <span className="kg-relationship-chip__target">→ {target}</span>
  </span>
)

export const KGQueryResultRenderer: FC<Props> = ({ toolName, rawResult }) => {
  // pis_kg_search — render list
  if (toolName === 'pis_kg_search') {
    const r = rawResult as KGSearchResult
    return (
      <div className="kg-search-results">
        {(r.results ?? []).map((item) => (
          <div key={item.id} className="kg-search-result-item">
            <KGNodeTypeBadge type={item.node_type} />
            <code>{item.id}</code>
            {item.description && <span className="kg-description">{item.description}</span>}
          </div>
        ))}
        {(r.results ?? []).length === 0 && (
          <span className="kg-search-empty">No results found.</span>
        )}
      </div>
    )
  }

  // pis_query_crs — structured CRS node
  if (toolName === 'pis_query_crs') {
    const r = rawResult as CRSNode
    return (
      <div className="kg-node-card kg-node-card--crs">
        <div className="kg-node-header">
          <KGNodeTypeBadge type="crs" />
          <code className="kg-node-id">{r.id}</code>
          <KGStatusBadge status={r.status} />
        </div>
        {r.layer && <span className="kg-meta">Layer: {r.layer}</span>}
        {r.feature_ref && <span className="kg-meta">Feature: {r.feature_ref}</span>}
        {r.relationships?.map((rel) => (
          <KGRelationshipChip key={`${rel.type}-${rel.target}`} {...rel} />
        ))}
      </div>
    )
  }

  // pis_query_feature_context, pis_query_hld, pis_query_adr — generic JSON display
  return (
    <pre className="kg-raw-result">
      {JSON.stringify(rawResult, null, 2)}
    </pre>
  )
}
```

**Verify:** Component render mà không crash với `rawResult = null`.

---

## T07 — Tạo `agent-capability-badge.tsx`

```
Action:    NEW
File:      src/renderer/src/components/agent/agent-capability-badge.tsx
Solution:  FE-IW-SOL-001 §SOL-1C
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/agent/agent-capability-badge.tsx
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
```

**Verify:** Badge render null khi `hasPisTools=false`. Render "KG" khi `hasPisTools=true`.

---

## T08 — Tạo `pis-tool-not-configured-error.tsx`

```
Action:    NEW
File:      src/renderer/src/components/agent/pis-tool-not-configured-error.tsx
Solution:  FE-IW-SOL-001 §SOL-1D
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/agent/pis-tool-not-configured-error.tsx
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
    <div
      className="pis-tool-config-error"
      role="alert"
      id={`pis-config-error-${toolName}`}
    >
      <span aria-hidden="true">⚙️</span>
      <strong>KG Integration Not Configured</strong>
      <p>
        Tool <code>{toolName}</code> requires PIKB Knowledge Graph integration.
      </p>
      <p className="pis-tool-config-error__fix">
        Set <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code> in the orca-agent deploy config
        to enable pis_* tools.
      </p>
    </div>
  )
}
```

**Verify:** Render null khi error không chứa "ORCA_BRIDGES_PRODUCTS_AGENT_ID".

---

## T09 — Tạo `pikb-integration-settings.tsx`

```
Action:    NEW
File:      src/renderer/src/components/settings/pikb-integration-settings.tsx
Solution:  FE-IW-SOL-001 §SOL-1E
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/settings/pikb-integration-settings.tsx
// Settings section hiển thị PIKB KG status (SSH environments only)
// Chỉ render khi environmentId present (server-side config)

import type { FC } from 'react'
import { useEffect } from 'react'
import { useAgentToolCapabilities, PIS_TOOL_NAMES } from '../../hooks/use-agent-tool-capabilities'
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
              {t.description && <span> — {t.description}</span>}
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
```

**Verify:** Render danh sách tools khi `hasPisTools=true`. Render hint khi `hasPisTools=false`.

---

## T10 — Tạo Fleet UI components (3 files)

### T10a — `agent-bridge-config-display.tsx`

```
Action:    NEW
File:      src/renderer/src/components/fleet/agent-bridge-config-display.tsx
Solution:  FE-IW-SOL-004 §SOL-4A
```

```typescript
// src/renderer/src/components/fleet/agent-bridge-config-display.tsx
// Hiển thị Bridges config của orca-agent (CR-ORCA-004)
// Fields: Config.Bridges.ProductsAgentID + Config.Bridges.DocAgentID (SOL-ORCA-004 §2.1)

import type { FC } from 'react'

interface AgentBridgeConfig {
  productsAgentId?: string  // ORCA_BRIDGES_PRODUCTS_AGENT_ID
  docAgentId?: string       // ORCA_BRIDGES_DOC_AGENT_ID
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
          Without <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>, pis_* tools will not be
          exposed in the orca-agent MCP catalog.
        </div>
      )}
    </div>
  )
}
```

### T10b — `agent-pis-health-check.tsx`

```
Action:    NEW
File:      src/renderer/src/components/fleet/agent-pis-health-check.tsx
Solution:  FE-IW-SOL-004 §SOL-4B
```

```typescript
// src/renderer/src/components/fleet/agent-pis-health-check.tsx
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

### T10c — `agent-env-config-display.tsx`

```
Action:    NEW
File:      src/renderer/src/components/fleet/agent-env-config-display.tsx
Solution:  FE-IW-SOL-004 §SOL-4C
```

```typescript
// src/renderer/src/components/fleet/agent-env-config-display.tsx
// Bảng 7 env vars từ .env.example (SOL-ORCA-004 §2.8)
// Sensitive vars masked với ●●●●●●

import type { FC } from 'react'

// 7 env vars theo đúng thứ tự .env.example (SOL-ORCA-004 §2.8)
const ORCA_AGENT_ENV_VARS = [
  { key: 'ORCA_BOOTSTRAP_TOKEN',            label: 'Bootstrap Token',   required: true,  sensitive: true,  cr: null },
  { key: 'ORCA_API_URL',                    label: 'Orca API URL',      required: true,  sensitive: false, cr: null },
  { key: 'ORCA_API_KEY',                    label: 'Orca API Key',      required: true,  sensitive: true,  cr: null },
  { key: 'ORCA_BRIDGES_PRODUCTS_AGENT_ID',  label: 'Products Agent ID', required: false, sensitive: false, cr: 'CR-ORCA-004' },
  { key: 'ORCA_BRIDGES_DOC_AGENT_ID',       label: 'Doc Agent ID',      required: false, sensitive: false, cr: 'CR-ORCA-004' },
  { key: 'ORCA_MCP_PROXY_URL',              label: 'MCP Proxy URL',     required: false, sensitive: false, cr: 'CR-ORCA-005' },
  { key: 'ORCA_MCP_PROXY_PEER_TOKEN',       label: 'Peer Token',        required: false, sensitive: true,  cr: 'CR-ORCA-005' },
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
          <tr key={key}>
            <td>
              <code>{key}</code>
              <br />
              <span className="env-label">{label}</span>
            </td>
            <td>
              {value ? (
                sensitive ? (
                  <span className="env-value--sensitive" aria-label="hidden value">●●●●●●</span>
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

### T10d — `pikb-bridge-step.tsx`

```
Action:    NEW
File:      src/renderer/src/components/onboarding/pikb-bridge-step.tsx
Solution:  FE-IW-SOL-004 §SOL-4D
```

```typescript
// src/renderer/src/components/onboarding/pikb-bridge-step.tsx
// Onboarding step để configure PIKB bridge (SSH environments, CR-ORCA-004)
// Default mcpProxyUrl từ docker-compose default (SOL-ORCA-004 §2.7)

import type { FC } from 'react'
import { useState } from 'react'

interface PIKBBridgeConfig {
  productsAgentId: string   // → ORCA_BRIDGES_PRODUCTS_AGENT_ID
  docAgentId?: string       // → ORCA_BRIDGES_DOC_AGENT_ID
  mcpProxyUrl: string       // → ORCA_MCP_PROXY_URL (required — CR-ORCA-005 dep)
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
        Enable pis_* tools cho AI agent query KG. Cần orca-agent server được cấu hình
        bridge env vars.
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
            onChange={(e) =>
              setConfig((c) => ({ ...c, mcpProxyPeerToken: e.target.value }))
            }
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

**Verify T10:** 4 files tồn tại. `agent-pis-health-check` import `PIS_TOOL_NAMES` từ T05. TypeScript clean.

---

## Checklist Phase 2

- [ ] T05: `use-agent-tool-capabilities.ts` — `PIS_TOOL_NAMES` export 5 elements
- [ ] T06: `kg-query-result-renderer.tsx` — render 3 cases (crs, search, fallback)
- [ ] T07: `agent-capability-badge.tsx` — null khi !hasPisTools
- [ ] T08: `pis-tool-not-configured-error.tsx` — null khi error không match
- [ ] T09: `pikb-integration-settings.tsx` — refresh button id="pikb-integration-refresh"
- [ ] T10a–T10d: 4 fleet/onboarding files tồn tại
- [ ] TypeScript compile: `npx tsc --noEmit` — 0 errors từ Phase 2 files
