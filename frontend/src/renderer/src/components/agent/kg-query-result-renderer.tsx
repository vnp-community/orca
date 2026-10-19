// Render structured KG node results từ pis_* tool calls (CR-ORCA-001)
// rawResult là json.RawMessage parsed từ backend PISTool.Dispatch()

import type { FC } from 'react'

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
  toolName: string // 1 trong 5 PIS_TOOL_NAMES
  rawResult: unknown // json.RawMessage parsed
}

const KGNodeTypeBadge: FC<{ type: string }> = ({ type }) => (
  <span className={`kg-badge kg-badge--${type}`}>{type.toUpperCase()}</span>
)

const KGStatusBadge: FC<{ status: string }> = ({ status }) => (
  <span className={`kg-status-badge kg-status-badge--${status}`} title={`Status: ${status}`}>
    {status}
  </span>
)

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
            {item.description != null && (
              <span className="kg-description">{item.description}</span>
            )}
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
        {r.layer != null && <span className="kg-meta">Layer: {r.layer}</span>}
        {r.feature_ref != null && <span className="kg-meta">Feature: {r.feature_ref}</span>}
        {r.relationships?.map((rel) => (
          <KGRelationshipChip key={`${rel.type}-${rel.target}`} {...rel} />
        ))}
      </div>
    )
  }

  // pis_query_feature_context, pis_query_hld, pis_query_adr — generic JSON display
  return <pre className="kg-raw-result">{JSON.stringify(rawResult, null, 2)}</pre>
}
