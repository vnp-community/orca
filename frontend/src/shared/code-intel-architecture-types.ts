/**
 * code-intel-architecture-types.ts — CONTRACT-codeintel-ui-api §4.4 C4, data flow, ERD, storage
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

import type { WithUnknown } from './code-intel-enum-fallback'
import type { SourceRef, SymbolRef } from './code-intel-index-types'

// ---------------------------------------------------------------------------
// §4.4  C4, data flow, ERD, storage
// ---------------------------------------------------------------------------

export type ContainerRef = {
  id: string
  name: string
  path: string
  kind: WithUnknown<'service' | 'agent' | 'frontend' | 'desktop' | 'other'>
}

export type C4Component = {
  id: string
  name: string
  kind: WithUnknown<
    'domain' | 'usecase' | 'adapter' | 'grpc-server' | 'grpc-client' | 'config' | 'other'
  >
  path: string
  description?: string
  descriptionSource: WithUnknown<'c4.yaml' | 'package-doc' | 'none'>
  symbolCount: number
  techHint?: string
  origin: WithUnknown<'derived' | 'merged' | 'declared'>
  packagePaths: string[]
  hidden: boolean
}

export type C4Relation = {
  from: string
  to: string
  kind: WithUnknown<
    'uses' | 'implements' | 'calls-rpc' | 'reads' | 'writes' | 'publishes' | 'subscribes'
  >
  evidence: SymbolRef[]
  count: number
  origin: WithUnknown<'derived' | 'declared'>
  confidence: number
  violatesLayering: boolean
  label?: string
}

export type C4External = {
  id: string
  name: string
  kind: WithUnknown<'service' | 'database' | 'queue' | 'vault' | 'external-api'>
  description?: string
  origin: WithUnknown<'derived' | 'declared'>
}

export type C4ComponentView = {
  container: ContainerRef
  components: C4Component[]
  relations: C4Relation[]
  externals: C4External[]
  warnings: { code: string; message: string }[]
  overridesVersion: string
  hasOverrides: boolean
}

export type ComponentRef = {
  container: string
  componentId: string
  name: string
  kind: WithUnknown<'component' | 'external' | 'ui'>
}
export type StoreRef = { id: string; kind: string; name: string; schema?: string }

export type DataFlowStep = {
  n: number
  from: ComponentRef
  to: ComponentRef
  kind: WithUnknown<'call' | 'rpc' | 'event' | 'db-read' | 'db-write' | 'ws-push'>
  method?: string
  symbol?: SymbolRef
  sync: boolean
  confidence: number
  origin: WithUnknown<'static-fieldtype' | 'static-name' | 'process' | 'declared'>
  evidence: SymbolRef[]
  requestType?: string
  responseType?: string
  unimplemented?: boolean
}

export type DataFlowTrigger = {
  kind: WithUnknown<'ws-channel' | 'http' | 'grpc' | 'event' | 'cron'>
  name: string
}

export type DataFlow = {
  id: string
  label: string
  trigger: DataFlowTrigger
  steps: DataFlowStep[]
  stores: { step: number; store: StoreRef; table?: string; op: 'read' | 'write'; confidence: number }[]
  completeness: WithUnknown<'complete' | 'partial'>
  gaps: { afterStep: number; code: string; message: string }[]
  services: string[]
  relatedProcesses: {
    processId: string
    label: string
    stepCount: number
    relation: 'contains-symbol'
  }[]
}

export type DataFlowSummary = {
  id: string
  label: string
  trigger: DataFlowTrigger
  entryService: string
  entryRpc: string
  serviceHops: number
  completeness: WithUnknown<'complete' | 'partial'>
}

export type SequenceModel = {
  participants: { id: string; label: string; kind: WithUnknown<'ui' | 'component' | 'external'>; group?: string }[]
  messages: {
    n: number
    from: string
    to: string
    label: string
    kind: string
    sync: boolean
    dashedReturn: boolean
    note?: string
    confidence: number
  }[]
}

export type DfdModel = {
  nodes: {
    id: string
    label: string
    kind: WithUnknown<'ui' | 'gateway' | 'service' | 'store' | 'queue' | 'external'>
    group?: string
  }[]
  edges: { from: string; to: string; label: string; kind: string; data: string[]; count: number }[]
}

export type ErdColumn = {
  name: string
  type: string
  canonicalType: string
  nullable: boolean
  defaultExpr?: string
  isPk: boolean
  isFk: boolean
  comment?: string
  generated: boolean
}

export type ErdTable = {
  name: string
  schema: string
  columns: ErdColumn[]
  pk: string[]
  indexes: {
    name: string
    columns: string[]
    unique: boolean
    partial?: string
    method?: string
    emulatesPartialUnique: boolean
  }[]
  checks: { name: string; expr: string }[]
  rls: { name: string; command: string; usingExpr?: string; withCheckExpr?: string }[]
  rlsState: WithUnknown<'enabled' | 'forced' | 'none'>
  comment?: string
  tenantScoped: boolean
  degraded: boolean
  firstMigration: string
  lastMigration: string
  accessedBy: {
    symbol: SymbolRef
    op: WithUnknown<'read' | 'write' | 'readwrite'>
    confidence: number
  }[]
}

export type ErdEndpoint = { service?: string; table: string; columns: string[] }

export type ErdRelation = {
  kind: WithUnknown<'fk' | 'logical'>
  from: ErdEndpoint
  to: ErdEndpoint
  cardinality?: 'one-to-one' | 'many-to-one'
  crossService: boolean
  source: WithUnknown<'ddl' | 'declared' | 'comment' | 'naming'>
  confidence: number
  onDelete?: string
  note?: string
}

export type ErdChange = {
  table: string
  column?: string
  kind: WithUnknown<'added' | 'removed' | 'modified'>
  before?: { type: string; nullable: boolean; default?: string }
  after?: { type: string; nullable: boolean; default?: string }
  migrationFile: string
  line?: number
}

export type ErdModel = {
  service: string
  dialect: 'postgres' | 'mysql'
  schema: string
  asOfMigration: string
  tables: ErdTable[]
  relations: ErdRelation[]
  externalRefs: { service: string; table: string }[]
  changes: ErdChange[]
  warnings: { file: string; line?: number; code: string; message: string }[]
}

export type ErdServiceInfo = { name: string; dialects: ('postgres' | 'mysql')[]; tableCount: number }

export type StoreConfidence = WithUnknown<'declared' | 'derived' | 'inferred'>
export type StoreChange = 'added' | 'removed' | 'modified'

export type Store = {
  id: string
  kind: WithUnknown<'postgres' | 'mysql' | 'redis' | 'object' | 'volume' | 'vault' | 'queue' | 'other'>
  name: string
  env: WithUnknown<'dev' | 'prod' | 'legacy'>
  owner?: { name: string }
  schemas?: string[]
  deployed: boolean
  supportedByCode: boolean
  external: boolean
  evidence: SourceRef[]
  confidence: StoreConfidence
  change?: StoreChange
}

export type StorageMap = {
  stores: Store[]
  bindings: {
    service: string
    store: string
    access: WithUnknown<'rw' | 'ro'>
    via: string
    configKey?: string
    evidence: SourceRef[]
    confidence: StoreConfidence
    change?: StoreChange
  }[]
  topics: {
    name: string
    stream?: string
    publishers: string[]
    subscribers: string[]
    delivery: WithUnknown<'durable' | 'ephemeral'>
    payload?: string
    evidence: SourceRef[]
    confidence: StoreConfidence
  }[]
  sources: SourceRef[]
  redactedCount: number
  asOfCommit: string
  warnings: string[]
}
