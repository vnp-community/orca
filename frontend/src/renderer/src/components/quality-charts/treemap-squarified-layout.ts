export type SquarifyBounds = { x: number; y: number; width: number; height: number }
export type SquarifyRect = SquarifyBounds & { id: string }
export type SquarifyResult = { rects: SquarifyRect[]; omitted: string[] }

function worstRatio(areas: number[], side: number): number {
  const sum = areas.reduce((a, b) => a + b, 0)
  const max = Math.max(...areas)
  const min = Math.min(...areas)
  const s2 = side * side
  return Math.max((s2 * max) / (sum * sum), (sum * sum) / (s2 * min))
}

/**
 * Squarified treemap (Bruls, Huizing, van Wijk). Why a second algorithm: the status-bar
 * treemap bisects and yields poor aspect ratios; this lives here so review lenses can reuse it.
 * Items with non-positive or non-finite size are returned in `omitted`.
 */
export function squarify(
  items: readonly { id: string; size: number }[],
  bounds: SquarifyBounds
): SquarifyResult {
  const omitted: string[] = []
  const valid = items.filter((item) => {
    const ok = Number.isFinite(item.size) && item.size > 0
    if (!ok) {
      omitted.push(item.id)
    }
    return ok
  })
  const usable =
    [bounds.x, bounds.y, bounds.width, bounds.height].every(Number.isFinite) &&
    bounds.width > 0 &&
    bounds.height > 0
  if (!usable || valid.length === 0) {
    return { rects: [], omitted: usable ? omitted : [...omitted, ...valid.map((v) => v.id)] }
  }
  const sorted = [...valid].sort(
    (a, b) => b.size - a.size || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
  )
  const total = sorted.reduce((sum, item) => sum + item.size, 0)
  const unit = (bounds.width * bounds.height) / total
  const scaled = sorted.map((item) => ({ id: item.id, area: item.size * unit }))

  const rects: SquarifyRect[] = []
  let { x, y, width, height } = bounds
  let row: typeof scaled = []

  const flushRow = (): void => {
    if (row.length === 0) {
      return
    }
    const rowArea = row.reduce((sum, r) => sum + r.area, 0)
    if (width >= height) {
      const colWidth = rowArea / height
      let cy = y
      for (const r of row) {
        const h = r.area / colWidth
        rects.push({ id: r.id, x, y: cy, width: colWidth, height: h })
        cy += h
      }
      x += colWidth
      width -= colWidth
    } else {
      const rowHeight = rowArea / width
      let cx = x
      for (const r of row) {
        const w = r.area / rowHeight
        rects.push({ id: r.id, x: cx, y, width: w, height: rowHeight })
        cx += w
      }
      y += rowHeight
      height -= rowHeight
    }
    row = []
  }

  for (const item of scaled) {
    const side = Math.min(width, height)
    if (row.length === 0 || side <= 0) {
      row.push(item)
      continue
    }
    const current = row.map((r) => r.area)
    if (worstRatio([...current, item.area], side) <= worstRatio(current, side)) {
      row.push(item)
    } else {
      flushRow()
      row.push(item)
    }
  }
  flushRow()
  return { rects, omitted }
}
