export type ChartTableCell = string | number | null

export type ChartTableData = {
  caption: string
  columns: { key: string; label: string; align?: 'start' | 'end' }[]
  rows: Record<string, ChartTableCell>[]
}

export type ChartSize = { width: number; height: number }

export type ChartStatus = 'loading' | 'ready' | 'empty' | 'error' | 'stale'
