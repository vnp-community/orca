import { CodeIntelRequestContext, registerIndexProbe } from './codeintel-method-table'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { CodeIntelConcurrencyGate } from './codeintel-concurrency-gate'
import { parseCodeGraphStatus } from './codegraph-cli-output'
import { CodeIntelError } from './codeintel-errors'

const probeGate = new CodeIntelConcurrencyGate()

export async function codegraphIndexProbe(workspaceRoot: string, ctx: CodeIntelRequestContext): Promise<any> {
  let binding
  try {
    binding = await resolveCodeIntelRepo(workspaceRoot, ctx.config, ctx.log)
  } catch (err) {
    return { state: 'missing', lineBase: 1 }
  }
  
  let res
  try {
    res = await runCodeIntelTool(
      ['status', binding.toplevel, '-j'],
      binding.toplevel,
      ctx.config,
      probeGate,
      { deadline: ctx.deadline, tool: 'codegraph', signal: ctx.signal },
      ctx
    )
  } catch (err: any) {
    if (err instanceof CodeIntelError && err.code === 'CODEINTEL_INDEX_MISSING') {
      return { state: 'missing', lineBase: 1 }
    }
    return { state: 'missing', lineBase: 1 }
  }

  let status: any
  try {
    status = parseCodeGraphStatus(res.stdout)
  } catch {
    return { state: 'missing', lineBase: 1 }
  }
  
  let state = status.state || 'ready'
  
  let pendingChanges = status.pendingChanges !== undefined ? status.pendingChanges : null
  if (status.worktreeMismatch) {
    pendingChanges = null
  }

  const warnings: string[] = []
  if (status.pendingChanges !== null && status.pendingChanges > 0) {
    state = 'stale'
    if (status.commit === null || status.commit === undefined) {
      warnings.push('codegraph_has_no_commit')
    }
  } else if (status.worktreeMismatch) {
    state = 'stale'
  }

  if (state === 'uninitialized') {
    state = 'missing'
  }

  return {
    state,
    indexedAt: status.indexedAt || null,
    stats: status.stats || {},
    pendingChanges,
    backend: status.backend,
    journalMode: status.journalMode,
    dbSizeBytes: status.dbSizeBytes,
    extractionVersion: status.extractionVersion,
    reindexRecommended: status.reindexRecommended,
    rootMismatch: status.worktreeMismatch ? { worktreeRoot: binding.toplevel, indexRoot: status.indexRoot || '' } : null,
    lineBase: 1,
    ...(warnings.length > 0 && { warnings })
  }
}

registerIndexProbe('codegraph', codegraphIndexProbe)
