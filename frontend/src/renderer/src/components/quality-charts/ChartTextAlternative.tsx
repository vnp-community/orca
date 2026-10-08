import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { cn } from '../../lib/utils'
import { NO_VALUE_DASH } from './quality-chart-copy'
import type { ChartTableData } from './chart-frame-types'

type ChartTextAlternativeProps = {
  data: ChartTableData
  visible: boolean
  id?: string
}

// Why: the table is the canonical accessible path, so it stays in the DOM (sr-only, never
// display:none) while the chart is shown. Cells render as plain text (backend strings are untrusted).
export function ChartTextAlternative({
  data,
  visible,
  id
}: ChartTextAlternativeProps): React.JSX.Element {
  return (
    <div
      id={id}
      data-chart-table
      data-visible={visible ? 'true' : 'false'}
      className={cn(!visible && 'sr-only')}
    >
      <Table>
        <caption className="px-3 py-2 text-left text-xs text-muted-foreground">
          {data.caption}
        </caption>
        <TableHeader>
          <TableRow>
            {data.columns.map((column) => (
              <TableHead
                key={column.key}
                scope="col"
                className={cn(column.align === 'end' && 'text-right')}
              >
                {column.label}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.rows.map((row, index) => (
            <TableRow key={index}>
              {data.columns.map((column) => {
                const cell = row[column.key]
                return (
                  <TableCell
                    key={column.key}
                    className={cn(column.align === 'end' && 'text-right tabular-nums')}
                  >
                    {cell === null || cell === undefined ? NO_VALUE_DASH : String(cell)}
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
