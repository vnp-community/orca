import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { CodeIntelError } from './codeintel-errors'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { gitnexusIndexProbe } from './gitnexus-index-probe'
import { getHeadCommit } from './codeintel-head-commit'
import { detectCodeIntelTools } from './codeintel-tool-detection'

export function validateStructuralFacts(params: any): any {
  if (typeof params !== 'object' || params === null) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object', { field: 'params' })
  }

  const { workspaceRoot, kind, pair, pathPrefixes, limit, offset, _trace } = params

  if (typeof workspaceRoot !== 'string' || !workspaceRoot.trim()) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing or invalid workspaceRoot', { field: 'workspaceRoot' })
  }

  const validKinds = ['layerImports', 'cycles', 'importInDegree', 'fileSizes', 'unusedExports']
  if (typeof kind !== 'string' || !validKinds.includes(kind)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid or missing kind', { field: 'kind' })
  }

  if (pair !== undefined) {
    if (kind !== 'layerImports') {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'pair is only allowed when kind is layerImports', { field: 'pair' })
    }
    const validPairs = ['usecase->adapter', 'domain->usecase', 'domain->adapter', 'adapter->adapter']
    if (typeof pair !== 'string' || !validPairs.includes(pair)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid pair value', { field: 'pair' })
    }
  }

  const checkStr = (s: string) => {
    if (typeof s !== 'string') return false;
    if (s.length < 1 || s.length > 512) return false;
    if (/[\x00-\x1F]/.test(s)) return false; // no control chars
    if (s.includes('..') || s.includes('\\')) return false;
    const norm = s.normalize('NFKC')
    if (norm.startsWith('-') || norm.startsWith('\uFF0D') || norm.startsWith('\u2212')) return false;
    return true;
  }

  if (pathPrefixes !== undefined) {
    if (!Array.isArray(pathPrefixes) || pathPrefixes.length > 20) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'pathPrefixes must be an array of max length 20', { field: 'pathPrefixes' })
    }
    for (const p of pathPrefixes) {
      if (!checkStr(p) || !p.endsWith('/')) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid pathPrefix', { field: 'pathPrefixes' })
      }
    }
  }

  // limit and offset
  const l = typeof limit === 'number' ? Math.min(5000, Math.max(1, limit)) : 5000;
  const o = typeof offset === 'number' ? Math.max(0, offset) : 0;

  // check for unknown keys
  const allowedKeys = ['workspaceRoot', 'kind', 'pair', 'pathPrefixes', 'limit', 'offset', '_trace']
  for (const k of Object.keys(params)) {
    if (!allowedKeys.includes(k)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Unknown parameter: ${k}`, { field: k })
    }
  }

  return {
    workspaceRoot,
    kind,
    pair,
    pathPrefixes: pathPrefixes || ['backend-go/services/'],
    limit: l,
    offset: o,
    _trace
  }
}

export async function handleStructuralFacts(params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  const probe = await gitnexusIndexProbe(params.workspaceRoot, ctx)
  
  if (probe.state === 'missing') {
    throw new CodeIntelError('CODEINTEL_INDEX_MISSING', 'GitNexus index is missing for this repository')
  }
  if (probe.state === 'building') {
    throw new CodeIntelError('CODEINTEL_REINDEX_IN_PROGRESS', 'GitNexus is currently reindexing')
  }
  
  const tools = await detectCodeIntelTools(ctx.config)
  if (!tools.gitnexus.available) {
    throw new CodeIntelError('CODEINTEL_TOOL_UNAVAILABLE', 'GitNexus is not available')
  }

  let handlerData: any;
  let totalCount = 0;

  switch (params.kind) {
    case 'layerImports': {
      const module = await import('./codeintel-structural-layer-imports').catch(() => null)
      if (module && module.handleLayerImports) {
        handlerData = await module.handleLayerImports(binding, params, ctx)
      } else {
        handlerData = { kind: 'layerImports', rows: [] }
      }
      break;
    }
    case 'cycles': {
      const module = await import('./codeintel-structural-cycles').catch(() => null)
      if (module && module.handleCycles) {
        handlerData = await module.handleCycles(binding, params, ctx)
      } else {
        handlerData = { kind: 'cycles', cycleCount: 0, status: 'clean', cycles: [] }
      }
      break;
    }
    case 'importInDegree':
    case 'fileSizes': {
      const module = await import('./codeintel-structural-file-metrics').catch(() => null)
      if (module && module.handleFileMetrics) {
        handlerData = await module.handleFileMetrics(binding, params, ctx)
      } else {
        handlerData = { kind: params.kind, rows: [] }
      }
      break;
    }
    case 'unusedExports': {
      const module = await import('./codeintel-structural-unused-exports').catch(() => null)
      if (module && module.handleUnusedExports) {
        handlerData = await module.handleUnusedExports(binding, params, ctx)
      } else {
        handlerData = { kind: 'unusedExports', rows: [] }
      }
      break;
    }
  }

  // Apply limit and offset generically if there is a rows array (for layerImports, importInDegree, fileSizes, unusedExports)
  // For 'cycles', it has 'cycles' array instead of 'rows'
  let truncated = false;
  if (handlerData.rows) {
    totalCount = handlerData.rows.length;
    if (params.offset > 0 || handlerData.rows.length > params.limit) {
      handlerData.rows = handlerData.rows.slice(params.offset, params.offset + params.limit);
      if (params.offset + params.limit < totalCount) {
        truncated = true;
      }
    }
  } else if (handlerData.cycles) {
    totalCount = handlerData.cycleCount; // Wait, cycleCount is total or just length of cycles?
    // According to solution: "cycles phân trang theo vòng"
    if (params.offset > 0 || handlerData.cycles.length > params.limit) {
      handlerData.cycles = handlerData.cycles.slice(params.offset, params.offset + params.limit);
      if (params.offset + params.limit < totalCount) {
        truncated = true;
      }
    }
  }

  const headCommit = await getHeadCommit(params.workspaceRoot)
  const stale = computeStale([{ tool: 'gitnexus', version: tools.gitnexus.version, indexedAt: null, commit: probe.indexedCommit, lineBase: 1 }], headCommit, { worktreeMismatch: binding.worktreeMismatch })

  const resultParams: any = {
    sources: [{ tool: 'gitnexus', version: tools.gitnexus.version, indexedAt: null, commit: stale ? null : headCommit, lineBase: 1 }],
    headCommit,
    stale,
    truncated,
    totalCount,
    perf: ctx.perf.build(),
    data: handlerData
  }

  if (handlerData.warnings) {
    resultParams.warnings = handlerData.warnings;
    delete handlerData.warnings;
  }

  return buildCodeIntelResult(resultParams)
}
