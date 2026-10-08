export type MatrixOrdering = { order: string[]; blocks: string[][] }

/**
 * Orders matrix rows/columns so dependencies point one way on a DAG; cycles collapse into
 * blocks (strongly connected components). Iterative Tarjan, then a stable Kahn pass on the
 * condensation so equal-rank components keep input order.
 */
export function orderByStrongComponents(
  nodes: readonly string[],
  edges: readonly { from: string; to: string }[]
): MatrixOrdering {
  const ids: string[] = []
  const index = new Map<string, number>()
  for (const id of nodes) {
    if (!index.has(id)) {
      index.set(id, ids.length)
      ids.push(id)
    }
  }
  const n = ids.length
  const adjacency: number[][] = Array.from({ length: n }, () => [])
  const selfLoop = Array.from<boolean>({ length: n }).fill(false)
  for (const e of edges) {
    const a = index.get(e.from)
    const b = index.get(e.to)
    if (a === undefined || b === undefined) {
      continue
    }
    if (a === b) {
      selfLoop[a] = true
    } else {
      adjacency[a].push(b)
    }
  }

  const disc = Array.from<number>({ length: n }).fill(-1)
  const low = Array.from<number>({ length: n }).fill(0)
  const onStack = Array.from<boolean>({ length: n }).fill(false)
  const component = Array.from<number>({ length: n }).fill(-1)
  const stack: number[] = []
  const members: number[][] = []
  let counter = 0

  for (let root = 0; root < n; root++) {
    if (disc[root] !== -1) {
      continue
    }
    const work: { v: number; next: number }[] = [{ v: root, next: 0 }]
    disc[root] = low[root] = counter++
    stack.push(root)
    onStack[root] = true
    while (work.length > 0) {
      const frame = work.at(-1)!
      const v = frame.v
      if (frame.next < adjacency[v].length) {
        const w = adjacency[v][frame.next++]
        if (disc[w] === -1) {
          disc[w] = low[w] = counter++
          stack.push(w)
          onStack[w] = true
          work.push({ v: w, next: 0 })
        } else if (onStack[w]) {
          low[v] = Math.min(low[v], disc[w])
        }
      } else {
        work.pop()
        if (work.length > 0) {
          const parent = work.at(-1)!.v
          low[parent] = Math.min(low[parent], low[v])
        }
        if (low[v] === disc[v]) {
          const group: number[] = []
          let w: number
          do {
            w = stack.pop()!
            onStack[w] = false
            component[w] = members.length
            group.push(w)
          } while (w !== v)
          members.push(group.sort((a, b) => a - b))
        }
      }
    }
  }

  const count = members.length
  const outgoing: Set<number>[] = Array.from({ length: count }, () => new Set())
  const indegree = Array.from<number>({ length: count }).fill(0)
  for (let v = 0; v < n; v++) {
    for (const w of adjacency[v]) {
      const cv = component[v]
      const cw = component[w]
      if (cv !== cw && !outgoing[cv].has(cw)) {
        outgoing[cv].add(cw)
        indegree[cw]++
      }
    }
  }
  const ready: number[] = []
  for (let c = 0; c < count; c++) {
    if (indegree[c] === 0) {
      ready.push(c)
    }
  }
  const order: string[] = []
  const blocks: string[][] = []
  while (ready.length > 0) {
    let best = 0
    for (let i = 1; i < ready.length; i++) {
      if (members[ready[i]][0] < members[ready[best]][0]) {
        best = i
      }
    }
    const c = ready.splice(best, 1)[0]
    const group = members[c]
    order.push(...group.map((i) => ids[i]))
    if (group.length > 1 || selfLoop[group[0]]) {
      blocks.push(group.map((i) => ids[i]))
    }
    for (const next of outgoing[c]) {
      if (--indegree[next] === 0) {
        ready.push(next)
      }
    }
  }
  return { order, blocks }
}
