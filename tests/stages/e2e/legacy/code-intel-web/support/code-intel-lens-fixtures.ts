// Lens data for the fake code-intel backend. setHandler() bypasses the fake's §2.2 envelope, so
// handlers for envelope channels wrap their payload with `envelopeOf`.
import {
  CHANGE_OVERLAY_SMALL,
  C4_COMPONENT_VIEW,
  CONTAINER_REFS,
  DATA_FLOW_PARTIAL,
  IMPACT_GRAPH_NO_EDGES,
  SEQUENCE_MODEL,
  SYMBOL_DETAIL_WITH_SOURCE
} from '../../../../frontend/src/renderer/src/test-support/code-intel-fixtures'
import {
  CONTRACT_DIFF_MIXED,
  FINDINGS_MIXED
} from '../../../../frontend/src/renderer/src/test-support/contract-findings-fixtures'
import { sampleErdModel } from '../../../../../../frontend/src/renderer/src/components/review-map/erd/erd-model.fixture'
import { sampleStorageMap } from '../../../../../../frontend/src/renderer/src/components/review-map/storage/storage-map.fixture'
import type { FakeCodeIntelBackend } from './mock-code-intel-ws'

/** DSN-looking string the storage fixture carries in a binding; must never reach the DOM. */
export const DSN_CANARY_SECRET = 'p4ss'

let etag = 0
export const envelopeOf = (method: string, data: unknown): unknown => ({
  worktreeId: '',
  view: method.replace('codeIntel.', ''),
  sources: [{ tool: 'gitnexus', version: '1.0.0', indexedAt: null, commit: null }],
  headCommit: null,
  stale: false,
  truncated: false,
  totalCount: Array.isArray(data) ? data.length : 0,
  etag: `lens-etag-${++etag}`,
  fromCache: false,
  generatedAt: new Date().toISOString(),
  data
})

/** Seeds every Review lens and the symbol drawer with golden fixtures. */
export function seedReviewLenses(backend: FakeCodeIntelBackend): void {
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  backend.setChannelData('codeIntel.symbol', SYMBOL_DETAIL_WITH_SOURCE)
  backend.setChannelData('codeIntel.impact', IMPACT_GRAPH_NO_EDGES)
  const erd = sampleErdModel()
  // `erd` without `service` lists services; with one it returns that service's model.
  backend.setHandler('codeIntel.erd', (p) =>
    envelopeOf(
      'codeIntel.erd',
      p.service
        ? erd
        : {
            services: [
              { name: erd.service, dialects: ['postgres'], tableCount: 3 },
              // Owner of the ghost `tenants` table, so "open its ERD" has a target.
              { name: 'auth', dialects: ['postgres'], tableCount: 1 }
            ]
          }
    )
  )
  backend.setStorage(sampleStorageMap())
  backend.setContractDiff(CONTRACT_DIFF_MIXED)
  backend.setFindings({ findings: FINDINGS_MIXED, dismissedCount: 0 })
  backend.setHandler('codeIntel.architecture', (p) =>
    envelopeOf('codeIntel.architecture', {
      containers: CONTAINER_REFS,
      view: p.container === 'frontend' ? null : C4_COMPONENT_VIEW
    })
  )
  backend.setChannelData('codeIntel.dataFlows', {
    flows: [
      {
        id: DATA_FLOW_PARTIAL.id,
        label: DATA_FLOW_PARTIAL.label,
        trigger: DATA_FLOW_PARTIAL.trigger,
        entryService: 'order-service',
        entryRpc: 'order.create',
        serviceHops: 1,
        completeness: 'partial'
      }
    ]
  })
  backend.setChannelData('codeIntel.dataFlow', {
    flow: DATA_FLOW_PARTIAL,
    sequence: SEQUENCE_MODEL
  })
}
