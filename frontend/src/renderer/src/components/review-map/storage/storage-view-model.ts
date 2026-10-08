/**
 * storage-view-model.ts — FE-CV-TASK-058-01
 *
 * StorageMap -> nodes/edges in four lanes (service | store | topic | secret). Pure.
 * Secret nodes carry only a key name and a Vault path: the contract has no value field,
 * and `via`/`payload` are deliberately never copied onto them.
 */

import type {
  StorageMap,
  Store,
  StoreChange,
  StoreConfidence
} from '../../../../../shared/code-intel-architecture-types'
import { maskSensitiveText } from '../sensitive-text-masking'

export type StorageLane = 'service' | 'store' | 'topic' | 'secret'
export const STORAGE_LANES: readonly StorageLane[] = ['service', 'store', 'topic', 'secret']

export type StorageNode = {
  id: string
  lane: StorageLane
  /** Display name (masked). For secrets this is the config key or the Vault path. */
  name: string
  kind?: string
  env?: string
  owner?: string
  schemas?: string[]
  deployed?: boolean
  supportedByCode?: boolean
  external?: boolean
  confidence?: StoreConfidence | 'unknown'
  /** Vault path of a secret node (never a value). */
  vaultPath?: string
  stream?: string
  delivery?: string
  payload?: string
  evidencePaths: string[]
  backendChange?: StoreChange
  masked: boolean
}

export type StorageEdgeKind = 'binding' | 'publish' | 'subscribe' | 'config-key'

export type StorageEdge = {
  id: string
  kind: StorageEdgeKind
  from: string
  to: string
  access?: 'rw' | 'ro' | 'unknown'
  via?: string
  confidence: StoreConfidence | 'unknown'
  evidencePaths: string[]
  backendChange?: StoreChange
  masked: boolean
}

export type StorageViewModel = {
  nodes: StorageNode[]
  edges: StorageEdge[]
  warnings: { text: string; masked: boolean }[]
  redactedCount: number
}

export const serviceNodeId = (name: string): string => `service:${name}`
export const storeNodeId = (id: string): string => `store:${id}`
export const topicNodeId = (name: string): string => `topic:${name}`
export const secretNodeId = (key: string): string => `secret:${key}`

const CONFIDENCES = new Set(['declared', 'derived', 'inferred'])
function confidenceOf(value: string): StoreConfidence | 'unknown' {
  return CONFIDENCES.has(value) ? (value as StoreConfidence) : 'unknown'
}
function accessOf(value: string): 'rw' | 'ro' | 'unknown' {
  return value === 'rw' || value === 'ro' ? value : 'unknown'
}
function changeOf(value: string | undefined): StoreChange | undefined {
  return value === 'added' || value === 'removed' || value === 'modified' ? value : undefined
}
function evidencePaths(evidence: readonly { path: string }[] | undefined): string[] {
  return (evidence ?? []).map((e) => e.path)
}

function mask(text: string | undefined): { text: string; masked: boolean } {
  return maskSensitiveText(text ?? '')
}

function storeNode(store: Store): StorageNode {
  const name = mask(store.name)
  return {
    id: storeNodeId(store.id),
    lane: 'store',
    name: name.text,
    kind: store.kind,
    env: store.env,
    owner: store.owner?.name,
    schemas: store.schemas,
    deployed: store.deployed,
    supportedByCode: store.supportedByCode,
    external: store.external,
    confidence: confidenceOf(store.confidence),
    evidencePaths: evidencePaths(store.evidence),
    backendChange: changeOf(store.change),
    masked: name.masked
  }
}

export function buildStorageViewModel(map: StorageMap): StorageViewModel {
  const nodes = new Map<string, StorageNode>()
  const edges: StorageEdge[] = []
  const stores = new Map((map.stores ?? []).map((s) => [s.id, s]))

  const ensureService = (name: string): string => {
    const id = serviceNodeId(name)
    if (!nodes.has(id)) {
      const m = mask(name)
      nodes.set(id, { id, lane: 'service', name: m.text, evidencePaths: [], masked: m.masked })
    }
    return id
  }

  for (const store of stores.values()) {
    if (store.kind !== 'vault') {
      nodes.set(storeNodeId(store.id), storeNode(store))
    }
  }

  for (const [i, b] of (map.bindings ?? []).entries()) {
    const from = ensureService(b.service)
    const store = stores.get(b.store)
    const via = mask(b.via)
    let to: string
    let kind: StorageEdgeKind = 'binding'
    if (store?.kind === 'vault') {
      // Why: a Vault binding is shown as a secret reference, keyed by its config key.
      const key = b.configKey ?? store.name
      const keyMasked = mask(key)
      const path = mask(store.name)
      to = secretNodeId(key)
      kind = 'config-key'
      if (!nodes.has(to)) {
        nodes.set(to, {
          id: to,
          lane: 'secret',
          name: keyMasked.text,
          vaultPath: path.text,
          confidence: confidenceOf(store.confidence),
          evidencePaths: evidencePaths(store.evidence),
          backendChange: changeOf(store.change),
          masked: keyMasked.masked || path.masked
        })
      }
    } else {
      to = storeNodeId(b.store)
      if (!nodes.has(to)) {
        const m = mask(b.store)
        nodes.set(to, {
          id: to,
          lane: 'store',
          name: m.text,
          kind: 'other',
          confidence: 'unknown',
          evidencePaths: [],
          masked: m.masked
        })
      }
    }
    edges.push({
      id: `edge:${kind}:${i}:${from}->${to}`,
      kind,
      from,
      to,
      access: accessOf(b.access),
      via: kind === 'config-key' ? undefined : via.text,
      confidence: confidenceOf(b.confidence),
      evidencePaths: evidencePaths(b.evidence),
      backendChange: changeOf(b.change),
      masked: via.masked || (kind === 'config-key' && nodes.get(to)?.masked === true)
    })
  }

  for (const t of map.topics ?? []) {
    const id = topicNodeId(t.name)
    const name = mask(t.name)
    const stream = mask(t.stream)
    const payload = mask(t.payload)
    nodes.set(id, {
      id,
      lane: 'topic',
      name: name.text,
      stream: t.stream ? stream.text : undefined,
      delivery: t.delivery,
      payload: t.payload ? payload.text : undefined,
      confidence: confidenceOf(t.confidence),
      evidencePaths: evidencePaths(t.evidence),
      masked: name.masked || stream.masked || payload.masked
    })
    for (const p of t.publishers ?? []) {
      const from = ensureService(p)
      edges.push({
        id: `edge:publish:${from}->${id}`,
        kind: 'publish',
        from,
        to: id,
        confidence: confidenceOf(t.confidence),
        evidencePaths: evidencePaths(t.evidence),
        masked: false
      })
    }
    for (const s of t.subscribers ?? []) {
      const to = ensureService(s)
      edges.push({
        id: `edge:subscribe:${id}->${to}`,
        kind: 'subscribe',
        from: id,
        to,
        confidence: confidenceOf(t.confidence),
        evidencePaths: evidencePaths(t.evidence),
        masked: false
      })
    }
  }

  return {
    nodes: [...nodes.values()],
    edges,
    warnings: (map.warnings ?? []).map((w) => {
      const m = mask(w)
      return { text: m.text, masked: m.masked }
    }),
    redactedCount: map.redactedCount ?? 0
  }
}
