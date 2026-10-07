import { cypherString } from './gitnexus-cypher-literal'
import { assertReadOnlyCypher } from './gitnexus-cypher-guard'
import { CodeIntelError } from './codeintel-errors'

export function buildLayerImportsCypher(fromSeg: string, toSeg: string, pathPrefixes: string[]): string {
  if (pathPrefixes.length > 20) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Too many pathPrefixes')
  }

  let prefixClause = pathPrefixes.map(p => `a.filePath STARTS WITH ${cypherString(p)}`).join(' OR ')
  if (prefixClause.length === 0) {
    prefixClause = 'true'
  }

  const query = `MATCH (a:File)-[r:CodeRelation {type:'IMPORTS'}]->(b:File) ` +
`WHERE a.filePath CONTAINS ${cypherString(fromSeg)} ` +
`AND b.filePath CONTAINS ${cypherString(toSeg)} ` +
`AND NOT a.filePath ENDS WITH '_test.go' ` +
`AND NOT a.filePath CONTAINS '/usecasetest/' ` +
`AND NOT a.filePath CONTAINS '/testutil/' ` +
`AND NOT a.filePath CONTAINS '/cmd/' ` +
`AND NOT a.filePath CONTAINS '/proto/gen/' ` +
`AND NOT a.filePath ENDS WITH '.pb.go' ` +
`AND (${prefixClause}) ` +
`RETURN a.filePath, b.filePath`

  return assertReadOnlyCypher(query)
}

export function buildImportInDegreeCypher(pathPrefixes: string[], limitN: number): string {
  if (pathPrefixes.length > 20) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Too many pathPrefixes')
  }

  let prefixClause = pathPrefixes.map(p => `b.filePath STARTS WITH ${cypherString(p)}`).join(' OR ')
  if (prefixClause.length === 0) {
    prefixClause = 'true'
  }

  const query = `MATCH (a:File)-[r:CodeRelation {type:'IMPORTS'}]->(b:File) ` +
`WHERE NOT b.filePath ENDS WITH '_test.go' ` +
`AND (${prefixClause}) ` +
`RETURN b.filePath, count(DISTINCT a) ` +
`ORDER BY count(DISTINCT a) DESC, b.filePath ASC ` +
`LIMIT ${limitN}`

  return assertReadOnlyCypher(query)
}

export function buildFileSizesCypher(label: string, pathPrefixes: string[]): string {
  if (label !== 'Function' && label !== 'Method') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid label')
  }

  if (pathPrefixes.length > 20) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Too many pathPrefixes')
  }

  let prefixClause = pathPrefixes.map(p => `f.filePath STARTS WITH ${cypherString(p)}`).join(' OR ')
  if (prefixClause.length === 0) {
    prefixClause = 'true'
  }

  const query = `MATCH (f:File)-[:CodeRelation {type:'DEFINES'}]->(s:${label}) ` +
`WHERE NOT f.filePath ENDS WITH '_test.go' ` +
`AND (${prefixClause}) ` +
`RETURN f.filePath, count(s), sum(s.endLine - s.startLine + 1), max(s.endLine - s.startLine + 1)`

  return assertReadOnlyCypher(query)
}

export function buildUnusedExportsCypher(pathPrefixes: string[]): string {
  if (pathPrefixes.length > 20) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Too many pathPrefixes')
  }

  let prefixClause = pathPrefixes.map(p => `f.filePath STARTS WITH ${cypherString(p)}`).join(' OR ')
  if (prefixClause.length === 0) {
    prefixClause = 'true'
  }

  const query = `MATCH (f:Function) ` +
`WHERE f.isExported = true ` +
`AND NOT f.filePath ENDS WITH '_test.go' ` +
`AND NOT f.filePath CONTAINS '/usecasetest/' ` +
`AND NOT f.filePath CONTAINS '/cmd/' ` +
`AND (${prefixClause}) ` +
`AND NOT EXISTS { MATCH (x)-[r:CodeRelation]->(f) WHERE r.type IN ['CALLS', 'ACCESSES', 'IMPORTS'] } ` +
`RETURN f.id, f.name, labels(f)[0] AS label, f.filePath, f.startLine, f.endLine ` +
`ORDER BY f.filePath ASC, f.startLine ASC, f.name ASC`

  return assertReadOnlyCypher(query)
}
