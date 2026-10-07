import { CodeIntelRequestContext } from './codeintel-method-table'
import { SymbolRef } from './codeintel-symbol-ref'
import { buildSymbolRefFromCodeGraph, CodeGraphSymbolRef } from './codeintel-symbol-ref-codegraph'
import { openCodeGraphDb, closeCodeGraphDb, findNodes, callersById, calleesById } from './codegraph-sqlite-reader'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { CodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { parseCodeGraphQuery } from './codegraph-cli-output'
import { getAffectedTests } from './codegraph-affected-tests'

const enrichmentGate = new CodeIntelConcurrencyGate()

export async function enrichSymbol(
  symbol: SymbolRef,
  binding: { toplevel: string },
  ctx: CodeIntelRequestContext,
  warnings: string[]
): Promise<CodeGraphSymbolRef> {
  const db = openCodeGraphDb(binding.toplevel)
  let cgNodes: any[] = []
  let usedCli = false

  if (db) {
    cgNodes = findNodes(db, symbol.name, 100)
  } else {
    try {
      const res = await runCodeIntelTool(
        ['query', binding.toplevel, symbol.name, '-j'],
        binding.toplevel,
        ctx.config,
        enrichmentGate,
        { deadline: ctx.deadline, tool: 'codegraph', signal: ctx.signal },
        ctx
      )
      cgNodes = parseCodeGraphQuery(res.stdout)
      usedCli = true
      warnings.push('codegraph_name_based_resolution')
    } catch {
      return symbol
    }
  }

  // Khớp theo file_path, name, kind
  const matches = cgNodes.filter(n => {
    const r = buildSymbolRefFromCodeGraph(n)
    if (!r) return false
    if (r.name !== symbol.name) return false
    // Tạm chấp nhận so khớp lỏng hơn nếu thiếu filePath, nhưng ưu tiên chính xác
    if (symbol.filePath && r.filePath && r.filePath !== symbol.filePath) return false
    // Kind matches
    if (r.kind !== symbol.kind) {
       // Relax for value vs method or function
       if (!(['function', 'method'].includes(r.kind) && ['function', 'method'].includes(symbol.kind)) && 
           !(symbol.kind === 'value' || r.kind === 'value')) {
         return false
       }
    }
    return true
  })

  if (matches.length > 1) {
    warnings.push('AMBIGUOUS_SYMBOL')
  }

  const match = matches[0]
  if (!match) return symbol

  const cgRef = buildSymbolRefFromCodeGraph(match)
  if (!cgRef) return symbol

  const enriched = { ...symbol }
  if (cgRef.codegraphId) (enriched as any).codegraphId = cgRef.codegraphId
  if (cgRef.signature) (enriched as any).signature = cgRef.signature
  if (cgRef.docstring) (enriched as any).docstring = cgRef.docstring
  if (cgRef.isExported !== undefined) (enriched as any).isExported = cgRef.isExported

  // includeTrail thử nghiệm
  if ((symbol as any).includeTrail && cgRef.codegraphId) {
    try {
      const nodeRes = await runCodeIntelTool(
        ['node', binding.toplevel, cgRef.codegraphId],
        binding.toplevel,
        ctx.config,
        enrichmentGate,
        { deadline: ctx.deadline, tool: 'codegraph', signal: ctx.signal },
        ctx
      )
      ;(enriched as any).trail = nodeRes.stdout.substring(0, 2048)
      warnings.push('experimental:true')
    } catch {
      // lỗi bỏ im lặng + cảnh báo
      warnings.push('trail_fetch_failed')
    }
  }

  return enriched
}

export async function enrichSubgraph(
  subgraph: any,
  binding: { toplevel: string },
  ctx: CodeIntelRequestContext,
  warnings: string[]
) {
  const db = openCodeGraphDb(binding.toplevel)
  if (!db) return subgraph

  subgraph.source = 'auto'

  const cgEdgesMap = new Map<string, any>()
  for (const node of subgraph.nodes) {
    if ((node as any).codegraphId) {
       const callers = callersById(db, (node as any).codegraphId, 500)
       for (const caller of callers) {
          const cRef = buildSymbolRefFromCodeGraph(caller)
          if (cRef) {
             const key = `${cRef.uid}::${node.uid}::CALLS`
             cgEdgesMap.set(key, { from: cRef.uid, to: node.uid, kind: 'CALLS', sources: ['codegraph'] })
          }
       }
       const callees = calleesById(db, (node as any).codegraphId, 500)
       for (const callee of callees) {
          const cRef = buildSymbolRefFromCodeGraph(callee)
          if (cRef) {
             const key = `${node.uid}::${cRef.uid}::CALLS`
             cgEdgesMap.set(key, { from: node.uid, to: cRef.uid, kind: 'CALLS', sources: ['codegraph'] })
          }
       }
    }
  }

  // Hợp nhất cạnh
  for (const edge of subgraph.edges) {
     const key = `${edge.from}::${edge.to}::${edge.kind}`
     edge.sources = ['gitnexus']
     if (cgEdgesMap.has(key)) {
        edge.sources.push('codegraph')
        cgEdgesMap.delete(key)
     }
  }

  // Thêm cạnh mới từ codegraph
  for (const edge of cgEdgesMap.values()) {
     subgraph.edges.push(edge)
  }

  return subgraph
}

export async function enrichImpact(
  impactData: any,
  binding: { toplevel: string },
  ctx: CodeIntelRequestContext,
  warnings: string[],
  includeTests: boolean
) {
  if (includeTests) {
    try {
       const files = Array.from(new Set(impactData.nodes.map((n: any) => n.filePath).filter(Boolean))) as string[]
       if (files.length > 0) {
         const { affectedTests } = await getAffectedTests(binding, files, ctx)
         impactData.testsCovering = affectedTests
       }
    } catch {
       warnings.push('codegraph_affected_tests_failed')
    }
  }
  return impactData
}
