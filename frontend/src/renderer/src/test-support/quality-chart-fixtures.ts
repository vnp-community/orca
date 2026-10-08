// Deterministic synthetic data at the performance-budget ceiling for the quality chart primitives.
// Independent of backend contract types on purpose: the feature layer maps real data separately.

export function seededRandom(seed: number): () => number {
  let state = seed >>> 0
  return () => {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0
    return state / 4294967296
  }
}

export type FixtureTreemapItem = { id: string; label: string; size: number; intensity: number }

export function buildTreemapItems(count = 400, seed = 1): FixtureTreemapItem[] {
  const rnd = seededRandom(seed)
  return Array.from({ length: count }, (_, i) => ({
    id: `file-${i}`,
    label: `src/module-${i % 17}/file-${i}.ts`,
    size: 1 + Math.floor(rnd() * rnd() * 4000),
    intensity: Math.floor(rnd() * 100)
  }))
}

export type FixtureHeatmapRow = { id: string; label: string; values: (number | null)[] }

export function buildHeatmapRows(
  rows = 40,
  cols = 6,
  nullRate = 0.1,
  seed = 2
): FixtureHeatmapRow[] {
  const rnd = seededRandom(seed)
  return Array.from({ length: rows }, (_, r) => ({
    id: `row-${r}`,
    label: `src/area-${r % 9}/hotspot-${r}.ts`,
    values: Array.from({ length: cols }, () => (rnd() < nullRate ? null : Math.floor(rnd() * 100)))
  }))
}

export type FixtureGraph = {
  nodes: { id: string; label: string }[]
  edges: { from: string; to: string; weight: number }[]
}

/** DAG by construction (edges only go to higher indexes) plus `cycles` back edges. */
export function buildDependencyGraph(
  nodeCount = 60,
  density = 0.06,
  cycles = 3,
  seed = 3
): FixtureGraph {
  const rnd = seededRandom(seed)
  const nodes = Array.from({ length: nodeCount }, (_, i) => ({ id: `mod-${i}`, label: `mod-${i}` }))
  const edges: FixtureGraph['edges'] = []
  for (let i = 0; i < nodeCount; i++) {
    for (let j = i + 1; j < nodeCount; j++) {
      if (rnd() < density) {
        edges.push({ from: nodes[i].id, to: nodes[j].id, weight: 1 + Math.floor(rnd() * 9) })
      }
    }
  }
  for (let c = 0; c < cycles && nodeCount > 3; c++) {
    const from = 2 + Math.floor(rnd() * (nodeCount - 2))
    const to = Math.floor(rnd() * from)
    edges.push({ from: nodes[from].id, to: nodes[to].id, weight: 1 })
  }
  return { nodes, edges }
}

export type FixtureTrendSeries = {
  id: string
  label: string
  points: { label: string; value: number | null }[]
}

export function buildTrendSeries(
  points = 50,
  series = 4,
  nullRate = 0.05,
  seed = 4
): { series: FixtureTrendSeries[]; xLabels: string[] } {
  const rnd = seededRandom(seed)
  const ids = ['error', 'warning', 'info', 'extra']
  const xLabels = Array.from({ length: points }, (_, i) => `T${i + 1}`)
  return {
    xLabels,
    series: Array.from({ length: series }, (_, s) => ({
      id: ids[s] ?? `series-${s}`,
      label: ids[s] ?? `Series ${s}`,
      points: xLabels.map((label) => ({
        label,
        value: rnd() < nullRate ? null : Math.floor(rnd() * 30)
      }))
    }))
  }
}

export function buildSeverityCounts(seed = 5): { error: number; warning: number; info: number } {
  const rnd = seededRandom(seed)
  return {
    error: Math.floor(rnd() * 20),
    warning: Math.floor(rnd() * 40),
    info: Math.floor(rnd() * 80)
  }
}
