import { runCypherTemplate } from './gitnexus-cypher-runner'

export interface AffectedFlowEntry {
  id: string
  label?: string
  stepCount: number
  earliestChangedStep: number
  changedSymbols: string[]
}

export interface AffectedClusterEntry {
  id: string
  label?: string
  changedSymbols: string[]
}

export interface FindAffectedFlowsAndClustersOptions {
  repoRoot: string
  withClusters?: boolean
  runCypher?: typeof runCypherTemplate
}

const FLOWS_BATCH_SIZE = 300
const MAX_AFFECTED_FLOWS = 200

export async function findAffectedFlows(
  symbolIds: string[],
  options: FindAffectedFlowsAndClustersOptions
): Promise<AffectedFlowEntry[]> {
  if (symbolIds.length === 0) return []

  const { repoRoot, runCypher = runCypherTemplate } = options
  const flowsMap = new Map<
    string,
    {
      id: string
      label?: string
      stepCount: number
      earliestChangedStep: number
      changedSymbolsSet: Set<string>
    }
  >()

  const batches: string[][] = []
  for (let i = 0; i < symbolIds.length; i += FLOWS_BATCH_SIZE) {
    batches.push(symbolIds.slice(i, i + FLOWS_BATCH_SIZE))
  }

  for (const batch of batches) {
    try {
      const res = await runCypher('SYMBOL_FLOWS', { ids: batch }, { repoRoot })
      for (const row of res.rows) {
        const symbolId = String(row['s.id'] ?? '')
        const flowId = String(row['p.id'] ?? '')
        const stepCount = Number(row['p.stepCount'] ?? 0)
        const step = Number(row['r.step'] ?? 0)
        const label = row['p.label'] ? String(row['p.label']) : undefined

        let existing = flowsMap.get(flowId)
        if (!existing) {
          existing = {
            id: flowId,
            label,
            stepCount,
            earliestChangedStep: step,
            changedSymbolsSet: new Set()
          }
          flowsMap.set(flowId, existing)
        } else {
          if (step < existing.earliestChangedStep) {
            existing.earliestChangedStep = step
          }
        }
        if (symbolId) {
          existing.changedSymbolsSet.add(symbolId)
        }
      }
    } catch {
      // ignore individual batch errors
    }
  }

  const allFlows: AffectedFlowEntry[] = Array.from(flowsMap.values()).map(f => ({
    id: f.id,
    label: f.label,
    stepCount: f.stepCount,
    earliestChangedStep: f.earliestChangedStep,
    changedSymbols: Array.from(f.changedSymbolsSet)
  }))

  // Sort by number of changed symbols descending, then earliestChangedStep ascending
  allFlows.sort((a, b) => {
    const diff = b.changedSymbols.length - a.changedSymbols.length
    if (diff !== 0) return diff
    return a.earliestChangedStep - b.earliestChangedStep
  })

  return allFlows.slice(0, MAX_AFFECTED_FLOWS)
}

export async function findAffectedClusters(
  symbolIds: string[],
  options: FindAffectedFlowsAndClustersOptions
): Promise<AffectedClusterEntry[]> {
  if (symbolIds.length === 0 || !options.withClusters) return []

  const { repoRoot, runCypher = runCypherTemplate } = options
  const clustersMap = new Map<string, { id: string; changedSymbolsSet: Set<string> }>()

  const batches: string[][] = []
  for (let i = 0; i < symbolIds.length; i += FLOWS_BATCH_SIZE) {
    batches.push(symbolIds.slice(i, i + FLOWS_BATCH_SIZE))
  }

  for (const batch of batches) {
    try {
      const res = await runCypher('MEMBER_CLUSTER', { ids: batch }, { repoRoot })
      for (const row of res.rows) {
        const symbolId = String(row['s.id'] ?? '')
        const clusterId = String(row['c.id'] ?? '')
        if (!clusterId) continue

        let existing = clustersMap.get(clusterId)
        if (!existing) {
          existing = {
            id: clusterId,
            changedSymbolsSet: new Set()
          }
          clustersMap.set(clusterId, existing)
        }
        if (symbolId) {
          existing.changedSymbolsSet.add(symbolId)
        }
      }
    } catch {
      // ignore
    }
  }

  return Array.from(clustersMap.values()).map(c => ({
    id: c.id,
    changedSymbols: Array.from(c.changedSymbolsSet)
  }))
}
