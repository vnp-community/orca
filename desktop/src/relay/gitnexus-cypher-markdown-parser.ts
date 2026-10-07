import { CodeIntelError } from './codeintel-errors'
import { CypherTemplate } from './gitnexus-cypher-templates'

export type CypherRows = {
  rows: Array<Record<string, string | number | string[] | null>>
  rowCount: number
  skippedRows: number
  warnings: string[]
}

export function assertNoFreeTextColumns(template: CypherTemplate) {
  for (const c of template.columns) {
    const lower = c.header.toLowerCase()
    if (lower === 'content' || lower === 'description' || lower === 'docstring') {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Template ${template.id} has a forbidden free text column ${c.header}`)
    }
  }
}

export function parseCypherOutput(stdoutText: string, template: CypherTemplate): CypherRows {
  const text = stdoutText.trim()
  if (!text) {
    return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
  }

  if (text.startsWith('{') && text.includes('"error"')) {
    try {
      const parsed = JSON.parse(text)
      if (parsed.error) {
        const errMsg = String(parsed.error)
        if (/write operation/i.test(errMsg)) {
          throw new CodeIntelError('CODEINTEL_TOOL_FAILED', `GitNexus error: ${errMsg}`, { reason: 'write_blocked' })
        }
        if (/not found|does not exist/i.test(errMsg)) {
          throw new CodeIntelError('CODEINTEL_SYMBOL_NOT_FOUND', `GitNexus error: ${errMsg}`)
        }
        throw new CodeIntelError('CODEINTEL_TOOL_FAILED', `GitNexus error: ${errMsg}`, { reason: 'unknown_shape' })
      }
    } catch (e: any) {
      if (e instanceof CodeIntelError) throw e
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'truncated_stdout')
    }
  }

  if (text.startsWith('[')) {
    try {
      const parsed = JSON.parse(text)
      if (Array.isArray(parsed) && parsed.length === 0) {
        return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
      }
    } catch {
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'truncated_stdout')
    }
  }

  const lines = text.split(/\r?\n/).map(l => l.trim())
  if (lines.length < 2) {
    return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
  }

  const warnings: string[] = []
  let rowCount = 0
  
  let tableLines = lines
  const lastLine = lines[lines.length - 1]
  const rowCountMatch = lastLine.match(/\((\d+) rows?\)/i)
  if (rowCountMatch) {
    rowCount = parseInt(rowCountMatch[1], 10)
    tableLines = lines.slice(0, lines.length - 1)
  }

  if (tableLines.length < 2) {
    return { rows: [], rowCount: 0, skippedRows: 0, warnings: [] }
  }

  const parseRow = (line: string, isFreeText: boolean) => {
    let s = line.trim()
    if (s.startsWith('|')) s = s.substring(1)
    if (s.endsWith('|')) s = s.substring(0, s.length - 1)
    
    if (isFreeText) {
      const parts = s.split('|').map(p => p.trim())
      const numCols = template.columns.length
      if (parts.length >= numCols) {
        const firstCols = parts.slice(0, numCols - 1)
        const lastCol = parts.slice(numCols - 1).join(' | ')
        return [...firstCols, lastCol]
      }
      return parts
    } else {
      return s.split('|').map(p => p.trim())
    }
  }

  const headers = parseRow(tableLines[0], false)
  if (headers.length !== template.columns.length) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', `unexpected_columns: Expected ${template.columns.length}, got ${headers.length}`)
  }
  for (let i = 0; i < headers.length; i++) {
    if (headers[i] !== template.columns[i].header) {
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', `unexpected_columns: Expected header ${template.columns[i].header}, got ${headers[i]}`)
    }
  }

  const rows: Array<Record<string, any>> = []
  let skippedRows = 0

  for (let i = 2; i < tableLines.length; i++) {
    const line = tableLines[i]
    if (!line) continue
    
    const parts = parseRow(line, !!template.freeTextColumn)
    if (parts.length !== template.columns.length) {
      skippedRows++
      continue
    }

    const rowObj: Record<string, any> = {}
    for (let j = 0; j < template.columns.length; j++) {
      const colDef = template.columns[j]
      let val = parts[j]
      
      if (val === 'null' || val === '') {
        rowObj[colDef.header] = null
        continue
      }

      if (colDef.type === 'int') {
        const num = parseInt(val, 10)
        rowObj[colDef.header] = isNaN(num) ? null : num
      } else if (colDef.type === 'float') {
        const num = parseFloat(val)
        rowObj[colDef.header] = isNaN(num) ? null : num
      } else if (colDef.type === 'jsonArray') {
        try {
          const parsedArray = JSON.parse(val)
          if (Array.isArray(parsedArray)) {
            rowObj[colDef.header] = parsedArray.map(item => {
              if (typeof item === 'string' && item.startsWith("'") && item.endsWith("'")) {
                return item.slice(1, -1)
              }
              return item
            })
          } else {
            rowObj[colDef.header] = parsedArray
          }
        } catch {
          rowObj[colDef.header] = null
        }
      } else {
        rowObj[colDef.header] = val
      }
    }
    rows.push(rowObj)
  }

  if (skippedRows > 0) {
    warnings.push(`gitnexus: skipped ${skippedRows} rows with unparsable columns`)
  }

  if (rowCount > 0 && rows.length !== rowCount) {
    warnings.push('row_count_mismatch')
  }

  if (rowCount === 0 && rows.length > 0) {
    rowCount = rows.length + skippedRows
  }

  return { rows, rowCount, skippedRows, warnings }
}
