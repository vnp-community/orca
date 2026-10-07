import * as path from 'path'
import { CodeIntelRequestContext } from './codeintel-method-table'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import { CodeIntelError } from './codeintel-errors'
import { buildGitNexusArgv } from './codeintel-command-whitelist'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { getCodeIntelConcurrencyGate } from './codeintel-concurrency-gate'

/**
 * Handles 'cycles' structural facts.
 * 
 * Lọc theo `pathPrefixes`: Vòng (cycle) sẽ được giữ lại nếu tệp đầu tiên (first file)
 * của vòng đó bắt đầu bằng một trong các `pathPrefixes` được cung cấp.
 */
export async function handleCycles(binding: CodeIntelRepoBinding, params: any, ctx: CodeIntelRequestContext): Promise<any> {
  const argv = buildGitNexusArgv({ verb: 'check-cycles' }, binding.gitNexusRegistryPath || binding.toplevel)

  const gate = getCodeIntelConcurrencyGate()
  const opts = {
    tool: 'gitnexus' as const,
    deadline: ctx.deadline,
    signal: ctx.signal
  }

  const tmpPerf: any[] = []
  
  const res = await runCodeIntelTool(
    argv,
    binding.toplevel,
    ctx.config,
    gate,
    opts,
    { perf: tmpPerf }
  )

  if (tmpPerf.length > 0) {
    ctx.perf.recordCli(tmpPerf[0])
  }

  let parsed: any
  try {
    parsed = JSON.parse(res.stdout)
  } catch (err) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'truncated_stdout', { reason: 'truncated_stdout' })
  }

  if (!parsed || typeof parsed.status !== 'string' || typeof parsed.cycleCount !== 'number' || !Array.isArray(parsed.cycles)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'unknown_shape', { reason: 'unknown_shape' })
  }

  if (parsed.status !== 'cycles_found' && parsed.status !== 'clean') {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'unknown_shape', { reason: 'unknown_shape' })
  }

  const pathPrefixes: string[] = params.pathPrefixes || []
  let filteredCycles = parsed.cycles

  if (pathPrefixes.length > 0) {
    filteredCycles = parsed.cycles.filter((c: any) => {
      if (!c.files || c.files.length === 0) return false
      const firstFile = c.files[0]
      return pathPrefixes.some(prefix => firstFile.startsWith(prefix))
    })
  }

  const isValidPath = (p: string) => {
    if (path.isAbsolute(p)) return false
    if (p.includes('..')) return false
    return true
  }

  const cleanCycles = filteredCycles.map((c: any) => {
    const validFiles = (c.files || [])
      .map((f: string) => f.replace(/\\/g, '/'))
      .filter(isValidPath)
    return { files: validFiles }
  }).filter((c: any) => c.files.length > 0)

  cleanCycles.sort((a: any, b: any) => {
    const aStr = a.files.join('\n')
    const bStr = b.files.join('\n')
    if (aStr < bStr) return -1
    if (aStr > bStr) return 1
    return 0
  })

  return {
    kind: 'cycles',
    cycleCount: parsed.cycleCount,
    status: parsed.status,
    cycles: cleanCycles
  }
}
