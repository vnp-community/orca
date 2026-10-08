export type SegmentPoint = { x: number; y: number; index: number }

/** Splits points at null values so a missing number breaks the line instead of drawing 0 or bridging. */
export function splitAtGaps(
  values: readonly (number | null)[],
  xOf: (index: number) => number,
  yOf: (value: number) => number
): SegmentPoint[][] {
  const segments: SegmentPoint[][] = []
  let current: SegmentPoint[] = []
  values.forEach((value, index) => {
    if (typeof value === 'number' && Number.isFinite(value)) {
      current.push({ x: xOf(index), y: yOf(value), index })
    } else if (current.length > 0) {
      segments.push(current)
      current = []
    }
  })
  if (current.length > 0) {
    segments.push(current)
  }
  return segments
}

export function toPolylinePoints(segment: readonly SegmentPoint[]): string {
  return segment.map((p) => `${p.x.toFixed(2)},${p.y.toFixed(2)}`).join(' ')
}
