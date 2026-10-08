export type LinearScale = {
  valid: boolean
  domain: [number, number]
  scale: (value: number) => number
  invert: (pixel: number) => number
  ticks: (count?: number) => number[]
}

function niceStep(rawStep: number): number {
  const exponent = Math.floor(Math.log10(rawStep))
  const base = Math.pow(10, exponent)
  const fraction = rawStep / base
  const nice = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10
  return nice * base
}

// Why: multiply an integer index by the step instead of accumulating (0.1 + 0.2 drift).
function ticksBetween(min: number, max: number, count: number): number[] {
  if (!Number.isFinite(min) || !Number.isFinite(max) || max <= min || count < 1) {
    return []
  }
  const step = niceStep((max - min) / count)
  const first = Math.ceil(min / step - 1e-9)
  const last = Math.floor(max / step + 1e-9)
  const out: number[] = []
  for (let i = first; i <= last; i++) {
    out.push(Number((i * step).toPrecision(12)))
  }
  return out
}

export function niceDomain(min: number, max: number, count = 5): [number, number] {
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    return [0, 1]
  }
  const lo = Math.min(min, max)
  let hi = Math.max(min, max)
  if (hi === lo) {
    hi = lo + 1
  }
  const step = niceStep((hi - lo) / count)
  return [
    Number((Math.floor(lo / step) * step).toPrecision(12)),
    Number((Math.ceil(hi / step) * step).toPrecision(12))
  ]
}

export function createLinearScale(domain: [number, number], range: [number, number]): LinearScale {
  const finite = [...domain, ...range].every((v) => Number.isFinite(v))
  if (!finite) {
    return { valid: false, domain: [0, 1], scale: () => 0, invert: () => 0, ticks: () => [] }
  }
  let [d0, d1] = domain
  if (d0 === d1) {
    d1 = d0 + 1
  }
  const [r0, r1] = range
  const scale = (value: number): number =>
    Number.isFinite(value) ? r0 + ((value - d0) / (d1 - d0)) * (r1 - r0) : r0
  const invert = (pixel: number): number =>
    r1 === r0 ? d0 : d0 + ((pixel - r0) / (r1 - r0)) * (d1 - d0)
  return {
    valid: true,
    domain: [d0, d1],
    scale,
    invert,
    ticks: (count = 5) => ticksBetween(Math.min(d0, d1), Math.max(d0, d1), count)
  }
}
