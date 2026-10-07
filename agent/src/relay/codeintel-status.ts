import os from 'os'
import { CodeIntelRequestContext, getIndexProbes } from './codeintel-method-table'
import { CodeIntelError } from './codeintel-errors'
import { detectCodeIntelTools } from './codeintel-tool-detection'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { getHeadCommit } from './codeintel-head-commit'
import { buildCodeIntelResult, computeStale } from './codeintel-result-envelope'
import { readCodeIntelLimits } from './codeintel-limits'

export async function handleStatus(
  params: { workspaceRoot: string; baseRef?: string },
  ctx: CodeIntelRequestContext
) {
  if (process.platform === 'win32') {
    throw new CodeIntelError('CODEINTEL_TOOL_UNAVAILABLE', 'Code Intel is not supported on Windows', { reason: 'unsupported_platform' })
  }

  let binding: any = null
  try {
    binding = await resolveCodeIntelRepo(params.workspaceRoot, ctx.config, ctx.log)
  } catch (err: any) {
    if (err instanceof CodeIntelError && err.code === 'CODEINTEL_REPO_NOT_REGISTERED') {
      binding = null
    } else if (err instanceof CodeIntelError && err.code === 'CODEINTEL_PATH_NOT_ALLOWED') {
      // Contract says codeintel.status always succeeds when agent is running.
      // So if path is invalid or not a git worktree, we also return success but no sources.
      binding = null
    } else {
      throw err
    }
  }

  const tools = await detectCodeIntelTools(ctx.config)
  const probes = getIndexProbes()
  const sources: any[] = []
  const warnings: string[] = []
  const flags: any = {}

  if (binding) {
    if (binding.worktreeMismatch) flags.worktreeMismatch = true
    
    const results = await Promise.allSettled(probes.map(p => p.probe(params.workspaceRoot, ctx)))
    results.forEach((res, i) => {
      const tool = probes[i].tool
      if (res.status === 'fulfilled') {
        sources.push({ tool, ...res.value })
      } else {
        warnings.push(`Probe for ${tool} failed: ${res.reason.message}`)
        sources.push({ tool, state: 'unknown', version: null, indexedAt: null, commit: null, lineBase: 1 })
      }
    })
  }

  // Ensure default sources exist if probe wasn't registered but tool exists?
  // If we don't have a probe for a tool but it is available, add a generic 'unknown' source?
  // Contract: "probe mặc định {state:'unknown'}."
  if (!sources.some(s => s.tool === 'gitnexus')) {
    sources.push({ tool: 'gitnexus', state: 'unknown', version: tools.gitnexus.version, indexedAt: null, commit: null, lineBase: 1 })
  }
  if (!sources.some(s => s.tool === 'codegraph')) {
    sources.push({ tool: 'codegraph', state: 'unknown', version: tools.codegraph.version, indexedAt: null, commit: null, lineBase: 1 })
  }

  const host = {
    platform: os.platform(),
    cpus: os.cpus().length,
    loadavg: os.loadavg()[0],
    freemem: os.freemem()
  }

  const headCommit = await getHeadCommit(params.workspaceRoot)
  const stale = computeStale(sources, headCommit, flags)

  return buildCodeIntelResult({
    sources,
    headCommit,
    stale,
    truncated: false,
    totalCount: null,
    warnings,
    perf: ctx.perf.build(),
    data: {
      host,
      limits: readCodeIntelLimits(process.env, ctx.log),
      tools: {
        gitnexus: tools.gitnexus,
        codegraph: tools.codegraph
      }
    }
  })
}
