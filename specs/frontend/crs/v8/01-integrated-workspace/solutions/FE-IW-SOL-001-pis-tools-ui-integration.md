# FE-IW-SOL-001 — Frontend: PIS Tools UI Integration

**Solution ID:** FE-IW-SOL-001  
**CR Reference:** CR-ORCA-001 (pis-tools-wiring)  
**Backend Solution:** SOL-ORCA-001  
**TDD Reference:** TDD-FE-03 Runtime Client | TDD-FE-07 Hooks & IPC  
**Status:** ✅ Done — implemented 2026-10-19  
**Priority:** P1  
**Effort:** 1–2 ngày  
**Depends On:** SOL-ORCA-005 → SOL-ORCA-004 → SOL-ORCA-001 (backend chain)

---

## Alignment với Backend Solution

Backend (SOL-ORCA-001) xác định:

| Backend detail | Frontend implication |
|----------------|----------------------|
| `PISToolCatalog()` trong `catalog_pis.go` — 5 tools, **chỉ expose khi `ProductsAgentID != ""`** | Frontend phải handle trường hợp catalog **không có** pis_* tools (hasPisTools=false là normal) |
| `PISTool.Dispatch()` trả về `json.RawMessage` | Frontend render raw JSON response — cần parse/display riêng |
| 5 tool names cố định: `pis_query_feature_context`, `pis_query_crs`, `pis_query_hld`, `pis_query_adr`, `pis_kg_search` | UI hardcode đúng 5 tên này cho health check và display |
| `ExecuteLegacy()` routing — error message: `"pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID"` | Frontend surface đúng error text này khi tool call fail |
| Execution order: CR-ORCA-005 → CR-ORCA-004 → CR-ORCA-001 | Frontend **không thể query pis_* tools** cho đến khi toàn bộ chain done |

---

## Giải pháp

### SOL-1A: `useAgentToolCapabilities` Hook

```typescript
// src/renderer/src/hooks/use-agent-tool-capabilities.ts

import { useCallback, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'

// Khớp với 5 tools trong PISToolCatalog() của backend SOL-ORCA-001
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

  // pis_* tools chỉ present khi backend ORCA_BRIDGES_PRODUCTS_AGENT_ID set (SOL-ORCA-001)
  const hasPisTools = tools.some((t) => PIS_TOOL_NAMES.includes(t.name as PISTool))

  return { tools, loading, fetchCapabilities, hasPisTools }
}
```

**Tại sao `PIS_TOOL_NAMES` là const**: Danh sách 5 tools này cố định trong backend `PISToolCatalog()` — không dynamic. Hardcode để detect availability chính xác.

---

### SOL-1B: `KGQueryResultRenderer` Component

Backend `PISTool.Dispatch()` trả về `json.RawMessage` — frontend nhận raw JSON từ AI agent conversation. Cần render structured thay vì plain JSON:

```typescript
// src/renderer/src/components/agent/kg-query-result-renderer.tsx

// Matches backend PIKB KG node structure (từ products-agent → pikb-catalog-service)
interface CRSNode {
  id: string
  status: 'pending' | 'in-progress' | 'done' | 'cancelled'
  layer: string
  feature_ref?: string
  description?: string
  relationships?: Array<{ type: string; target: string }>
}

interface FeatureNode {
  id: string
  description?: string
  hld_ref?: string
}

interface HLDNode {
  product_id: string
  version?: string
  level?: 'L1' | 'L2' | 'L3' | 'L4'
  description?: string
}

interface ADRNode {
  id: string
  status: string
  supersedes?: string[]
}

interface KGSearchResult {
  results: Array<{ node_type: string; id: string; description?: string }>
}

type KGResult = CRSNode | FeatureNode | HLDNode | ADRNode | KGSearchResult

interface Props {
  // toolName là 1 trong 5 PIS_TOOL_NAMES
  toolName: string
  // rawResult là json.RawMessage parsed từ backend
  rawResult: unknown
}

export const KGQueryResultRenderer: FC<Props> = ({ toolName, rawResult }) => {
  if (toolName === 'pis_kg_search') {
    const r = rawResult as KGSearchResult
    return (
      <div className="kg-search-results">
        {(r.results ?? []).map((item) => (
          <div key={item.id} className="kg-search-result-item">
            <KGNodeTypeBadge type={item.node_type} />
            <code>{item.id}</code>
            {item.description && <span>{item.description}</span>}
          </div>
        ))}
      </div>
    )
  }

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

  // pis_query_feature_context, pis_query_hld, pis_query_adr — generic node display
  return (
    <pre className="kg-raw-result">
      {JSON.stringify(rawResult, null, 2)}
    </pre>
  )
}
```

---

### SOL-1C: Agent Panel — PIKB Integration Badge

```typescript
// src/renderer/src/components/agent/agent-capability-badge.tsx

import { useEffect } from 'react'
import { useAgentToolCapabilities } from '../../hooks/use-agent-tool-capabilities'

interface Props {
  environmentId?: string
}

export const AgentCapabilityBadge: FC<Props> = ({ environmentId }) => {
  const { hasPisTools, fetchCapabilities } = useAgentToolCapabilities(environmentId)

  // Fetch on mount — catalog only available after ORCA_BRIDGES_PRODUCTS_AGENT_ID set
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

---

### SOL-1D: PIS Tools Not Configured — Error Surface

Khi AI agent call pis_* tools mà ORCA_BRIDGES_PRODUCTS_AGENT_ID chưa set, backend trả về error:
`"pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID"`

Frontend cần nhận diện error này và hiển thị actionable message:

```typescript
// src/renderer/src/components/agent/pis-tool-not-configured-error.tsx

interface Props {
  toolName: string
  errorMessage: string
}

export const PISTooLNotConfiguredError: FC<Props> = ({ toolName, errorMessage }) => {
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
        Set <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code> in the orca-agent deploy config
        to enable pis_* tools.
      </p>
    </div>
  )
}
```

---

### SOL-1E: Settings — PIKB Integration Status (SSH Only)

```typescript
// src/renderer/src/components/settings/pikb-integration-settings.tsx

export const PIKBIntegrationSettings: FC<{ environmentId: string }> = ({ environmentId }) => {
  const { hasPisTools, tools, fetchCapabilities } = useAgentToolCapabilities(environmentId)
  const pisTools = tools.filter((t) => PIS_TOOL_NAMES.includes(t.name as PISTool))

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
        <ul className="pis-tool-list">
          {pisTools.map((t) => (
            <li key={t.name}>
              <code>{t.name}</code>
              {t.description && <span> — {t.description}</span>}
            </li>
          ))}
        </ul>
      ) : (
        <div className="settings-section__hint">
          <p>No pis_* tools detected. To enable PIKB KG integration on the orca-agent server:</p>
          <ol>
            <li>
              Set <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod</code> in{' '}
              <code>deploy/orca-agent/.env</code>
            </li>
            <li>
              Set <code>ORCA_MCP_PROXY_URL</code> and <code>ORCA_MCP_PROXY_PEER_TOKEN</code>{' '}
              (required by CR-ORCA-005)
            </li>
            <li>Restart orca-agent and click Refresh above</li>
          </ol>
        </div>
      )}
    </div>
  )
}
```

---

## Files cần tạo/sửa

```
src/renderer/src/
├── hooks/
│   └── use-agent-tool-capabilities.ts          ← NEW (PIS_TOOL_NAMES const + hook)
└── components/
    ├── agent/
    │   ├── kg-query-result-renderer.tsx         ← NEW (5 tool cases)
    │   ├── agent-capability-badge.tsx           ← NEW
    │   └── pis-tool-not-configured-error.tsx    ← NEW
    └── settings/
        └── pikb-integration-settings.tsx        ← NEW (SSH mode only)
```

---

## Tests

```typescript
// hooks/__tests__/use-agent-tool-capabilities.test.ts
describe('useAgentToolCapabilities', () => {
  it('hasPisTools=true khi catalog chứa "pis_query_crs"')
  it('hasPisTools=false khi catalog không chứa pis_* tools')
  it('hasPisTools=false khi catalog rỗng (ProductsAgentID chưa set)')
})

// components/agent/__tests__/kg-query-result-renderer.test.tsx
describe('KGQueryResultRenderer', () => {
  it('render CRS node với status badge khi toolName="pis_query_crs"')
  it('render search result list khi toolName="pis_kg_search"')
  it('render JSON fallback cho pis_query_hld')
})

// components/agent/__tests__/pis-tool-not-configured-error.test.tsx
describe('PISTooLNotConfiguredError', () => {
  it('render error khi errorMessage chứa "ORCA_BRIDGES_PRODUCTS_AGENT_ID"')
  it('return null khi error không liên quan đến config')
})
```

---

## Acceptance Criteria (Frontend)

| AC | Mô tả | Khớp với Backend AC |
|----|-------|---------------------|
| AC-F1 | `hasPisTools=true` khi catalog có đúng 5 pis_* tools | SOL-ORCA-001 §2.2 — catalog chỉ expose khi configured |
| AC-F2 | KGQueryResultRenderer render CRS node (id, status, layer) | SOL-ORCA-001 — `pis_query_crs` trả về CRS node |
| AC-F3 | Error "ORCA_BRIDGES_PRODUCTS_AGENT_ID" surface thành config hint | SOL-ORCA-001 §2.3 error message |
| AC-F4 | Settings hint liệt kê đúng 3 env vars (BRIDGES + PROXY + TOKEN) | SOL-ORCA-004 §2.8 .env.example |
| AC-F5 | `hasPisTools=false` khi catalog rỗng (không có pis_*) | SOL-ORCA-001 — PISTool không wire nếu không configured |
