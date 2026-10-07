import { QUALITY_METHODS, QUALITY_METHOD_SCHEMAS } from './quality-method-table'
import { CodeIntelError, toErrorPayload } from './codeintel-errors'
import { resolveWorktreeRoot } from './codeintel-repo-resolution'
import { isValidRunId } from './quality-run-id'
import { gateDisabled } from './codeintel/disabled-gate'
import { readRuntimeSwitches } from './codeintel/runtime-switches'

export async function dispatchQualityRpc(rpc: any, config: any, log: any, ws: any, state: any): Promise<any | null> {
  if (!rpc.method || typeof rpc.method !== 'string' || !rpc.method.startsWith('quality.')) {
    return null
  }

  const switches = readRuntimeSwitches(config?.toolEnv ?? process.env)
  const gated = gateDisabled(rpc, switches)
  if (gated) {
    return gated
  }

  function makeError(id: any, payload: any) {
    return { jsonrpc: '2.0', id: id ?? null, error: payload }
  }

  function validateString(val: any, field: string) {
    if (typeof val !== 'string') throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `${field} must be a string`, { field })
    if (val.length < 1 || val.length > 512) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `${field} length must be 1..512`, { field })
    // eslint-disable-next-line no-control-regex
    if (/[\x00-\x1F\x7F]/.test(val)) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `${field} contains control characters`, { field })
    const n = val.normalize('NFKC')
    if (n.startsWith('-') || n.startsWith('\uFF0D') || n.startsWith('\u2212')) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `${field} cannot start with dash`, { field })
    }
  }

  try {
    if (!QUALITY_METHODS.includes(rpc.method)) {
      return null // Will be handled by MethodNotFound
    }

    if (process.platform === 'win32') {
      throw new CodeIntelError('CODEINTEL_TOOL_UNAVAILABLE', 'Quality is unsupported on Windows', { reason: 'unsupported_platform' })
    }

    const params = rpc.params || {}
    if (typeof params !== 'object' || Array.isArray(params)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object')
    }

    const allowed = QUALITY_METHOD_SCHEMAS[rpc.method] || []
    for (const k of Object.keys(params)) {
      if (!allowed.includes(k)) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Unknown parameter: ${k}`, { field: k })
      }
    }

    if (!params.workspaceRoot) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing workspaceRoot', { field: 'workspaceRoot' })
    }
    validateString(params.workspaceRoot, 'workspaceRoot')

    let binding: any
    try {
      binding = await resolveWorktreeRoot(params.workspaceRoot)
    } catch (e: any) {
      throw new CodeIntelError('CODEINTEL_PATH_NOT_ALLOWED', e.message || 'Invalid workspaceRoot')
    }

    if (params.runId !== undefined) {
      validateString(params.runId, 'runId')
      if (!isValidRunId(params.runId)) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid runId format', { field: 'runId' })
      }
    }

    if (params.offset !== undefined) {
      if (typeof params.offset !== 'number' || params.offset < 0) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid offset', { field: 'offset' })
    }
    if (params.limit !== undefined) {
      if (typeof params.limit !== 'number' || params.limit < 1 || params.limit > 500) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid limit', { field: 'limit' })
    } else if (rpc.method === 'quality.results') {
      params.limit = 500
    }

    if (params.view !== undefined) {
      validateString(params.view, 'view')
      if (!['findings', 'steps', 'log'].includes(params.view)) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid view', { field: 'view' })
      if (params.view === 'log' && !params.stepId) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Missing stepId for log view', { field: 'stepId' })
    }
    if (params.stepId !== undefined) {
      validateString(params.stepId, 'stepId')
    }
    if (params.profile !== undefined) {
      validateString(params.profile, 'profile')
    }

    const timeoutMs = rpc.method === 'quality.listProfiles' ? 40000 : 10000

    const ac = new AbortController()
    const timer = setTimeout(() => {
      ac.abort(new CodeIntelError('CODEINTEL_TIMEOUT', `Method ${rpc.method} timed out`))
    }, timeoutMs)

    try {
      let result = {}
      if (rpc.method === 'quality.listProfiles') {
        const { handleListProfiles } = await import('./quality-list-profiles')
        result = await handleListProfiles(params, { config, log, signal: ac.signal })
      } else if (rpc.method === 'quality.run') {
        const { handleStartRun } = await import('./quality-run-start')
        result = await handleStartRun(params, { config, log, signal: ac.signal })
      } else if (rpc.method === 'quality.runStatus') {
        const manager = (await import('./quality-run-manager')).getQualityRunManager()
        const run = manager.getRun(params.runId)
        if (!run) throw new CodeIntelError('CODEINTEL_RUN_NOT_FOUND', `Run not found: ${params.runId}`, { runId: params.runId })
        result = { state: run.state }
      } else if (rpc.method === 'quality.cancel') {
        const manager = (await import('./quality-run-manager')).getQualityRunManager()
        await manager.cancelRun(params.runId)
        result = { status: 'cancelled' }
      } else if (rpc.method === 'quality.results') {
        const store = (await import('./quality-results-store')).getQualityResultsStore()
        result = store.getResults(params.runId, params.offset ?? 0, params.limit ?? 500, params.view, params.stepId)
      } else if (rpc.method === 'quality.coverage') {
        const { handleQualityCoverage } = await import('./quality-coverage-handler')
        result = await handleQualityCoverage(params, { config, log, signal: ac.signal })
      }

      if (ac.signal.aborted) {
        throw ac.signal.reason
      }

      return { jsonrpc: '2.0', id: rpc.id, result }
    } finally {
      clearTimeout(timer)
    }
  } catch (err: any) {
    return makeError(rpc.id, toErrorPayload(err))
  }
}
