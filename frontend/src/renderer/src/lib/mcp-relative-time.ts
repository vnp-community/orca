const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 31_536_000],
  ['month', 2_592_000],
  ['day', 86_400],
  ['hour', 3_600],
  ['minute', 60],
  ['second', 1]
]

export function formatMcpRelativeTime(iso: string | undefined, now = Date.now()): string {
  const ts = iso ? Date.parse(iso) : Number.NaN
  if (Number.isNaN(ts)) {
    return '—'
  }
  const diffSec = Math.round((ts - now) / 1000)
  const fmt = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
  for (const [unit, secs] of UNITS) {
    if (Math.abs(diffSec) >= secs || unit === 'second') {
      return fmt.format(Math.trunc(diffSec / secs), unit)
    }
  }
  return '—'
}
