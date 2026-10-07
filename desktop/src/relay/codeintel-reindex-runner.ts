import { ReindexJob, ReindexOutcome } from './codeintel-reindex-job'
import { emitCodeIntelNotification } from './codeintel-notification-sink'
import { runToolCommand } from './agent-tool-registry'
import { resolveCodeIntelBinary } from './codeintel-tool-runner'
import { buildCodeIntelChildEnv } from './codeintel-child-env'
import { buildReindexArgv, ReindexCommand } from './codeintel-reindex-commands'
import { parseProgressLine, createProgressLimiter } from './codeintel-reindex-progress'
import { gitnexusIndexProbe } from './gitnexus-index-probe'
import { codegraphIndexProbe } from './codegraph-index-probe'
import { invalidateHeadCommit } from './codeintel-head-commit'
import { getHeavyJobGate } from './agent-heavy-job-gate'
import { ORCA_HEAVY_QUEUE_WAIT_MS } from './quality-limits'
import { probeIndexBasis } from './codeintel-index-basis-probe'
import { classifyIndexBasis } from './codeintel-index-basis'

const RUNNING_JOBS = new Map<string, { abort: AbortController, outputTail: string }>()

export async function runReindexJob(job: ReindexJob, config: any): Promise<void> {
  job.state = 'running'
  job.outcome = ''
  
  const ac = new AbortController()
  const entry = { abort: ac, outputTail: '' }
  RUNNING_JOBS.set(job.jobId, entry)

  const emitProgress = createProgressLimiter((msg, pct) => {
    emitCodeIntelNotification('codeintel.reindexProgress', {
      jobId: job.jobId,
      workspaceRoot: job.workspaceRoot,
      stage: job.state,
      message: msg,
      percent: pct
    })
  })

  let lineBuffer = ''
  const appendOutput = (chunk: Buffer) => {
    const text = chunk.toString()
    entry.outputTail += text
    if (entry.outputTail.length > 65536) {
      entry.outputTail = entry.outputTail.substring(entry.outputTail.length - 65536)
    }
    
    lineBuffer += text
    const lines = lineBuffer.split('\n')
    lineBuffer = lines.pop() || ''
    
    for (const line of lines) {
      if (line.trim()) {
        const parsed = parseProgressLine(line)
        emitProgress(parsed.message, parsed.percent)
      }
    }
  }

  try {
    emitProgress('Starting preflight...', null, true)
    const env = buildCodeIntelChildEnv(config)

    let releaseGate: (() => void) | undefined
    try {
      if (job.mode === 'full') {
        emitProgress('Waiting for heavy job queue...', null, true)
        releaseGate = await getHeavyJobGate().acquire(ac.signal, ORCA_HEAVY_QUEUE_WAIT_MS)
      }

      for (const tool of job.tools) {
      if (ac.signal.aborted) break

      const cmd: ReindexCommand = tool === 'gitnexus' 
        ? { tool, mode: job.mode as any, repoRoot: job.repoRoot }
        : { tool, mode: job.mode as any, projectPath: job.repoRoot }
      
      const argv = buildReindexArgv(cmd)
      const binary = resolveCodeIntelBinary(tool, config.toolPath || '')

      emitProgress(`Starting ${tool}...`, null, true)

      const res = await runToolCommand(binary, argv, {
        cwd: job.repoRoot,
        timeout: 45 * 60 * 1000,
        env,
        killGraceMs: 10000,
        signal: ac.signal,
        detached: true,
        onStdout: appendOutput,
        onStderr: appendOutput
      })

      if (lineBuffer.trim()) {
        const parsed = parseProgressLine(lineBuffer)
        emitProgress(parsed.message, parsed.percent)
        lineBuffer = ''
      }

      if (res.exitCode !== 0 && !ac.signal.aborted) {
        throw new Error(`${tool} failed with exit code ${res.exitCode}`)
      }
    }

    } catch (err: any) {
      if (err.name === 'HeavyGateTimeoutError') {
        job.state = 'completed'
        job.outcome = 'queue_full'
        emitProgress('Heavy job queue timeout', null, true)
        return
      }
      throw err
    } finally {
      if (releaseGate) releaseGate()
    }

    if (ac.signal.aborted) {
      job.state = 'cancelled'
      emitProgress('Job cancelled', null, true)
    } else {
      emitProgress('Verifying index...', null, true)
      
      let outcome: ReindexOutcome = ''
      
      if (job.tools.includes('gitnexus')) {
         try { 
           const p = await gitnexusIndexProbe(job.workspaceRoot, { config, log: { log: () => {}, info: () => {}, warn: () => {}, error: () => {} } } as any)
           if (p.state === 'stale' || p.state === 'missing') outcome = 'index_not_updated'
         } catch {
           outcome = 'index_not_updated'
         }
      }
      
      if (outcome !== 'index_not_updated' && job.tools.includes('codegraph')) {
         try { 
           const p = await codegraphIndexProbe(job.workspaceRoot, { config, log: { log: () => {}, info: () => {}, warn: () => {}, error: () => {} } } as any)
           if (p.state === 'stale' || p.state === 'missing') outcome = 'index_not_updated'
         } catch {
           outcome = 'index_not_updated'
         }
      }

      job.state = 'completed'
      job.outcome = outcome
      emitProgress('Job completed', 100, true)
      
      let indexScope = 'unknown';
      let mergeBase = null;
      try {
        const probeRes = await probeIndexBasis(job.workspaceRoot, { baseRef: 'origin/HEAD' });
        const basis = classifyIndexBasis({
          tool: job.tools.includes('gitnexus') ? 'gitnexus' : 'codegraph',
          toolUsable: true,
          indexExists: true,
          rootMatches: true,
          indexedCommit: probeRes.headCommit,
          indexedAtMs: Date.now(),
          headCommit: probeRes.headCommit,
          headCommitTimeMs: probeRes.headCommitTimeMs,
          mergeBase: probeRes.mergeBase,
          mergeBaseCommitTimeMs: probeRes.mergeBaseCommitTimeMs,
          dirtySinceIndex: false,
          pendingChanges: null
        });
        indexScope = basis.indexScope;
        mergeBase = probeRes.mergeBase;
      } catch {
        // Ignore probe error to keep stdio clean
      }
      
      const payload: any = { reason: 'reindex', workspaceRoot: job.workspaceRoot, indexScope };
      if (mergeBase) payload.mergeBase = mergeBase;
      if (job.trigger) payload.trigger = job.trigger;
      
      emitCodeIntelNotification('codeintel.indexChanged', payload)
      invalidateHeadCommit(job.workspaceRoot)
    }
  } catch (err: any) {
    job.state = 'failed'
    entry.outputTail += '\n' + err.message
    emitProgress(`Job failed: ${err.message}`, null, true)
  } finally {
    RUNNING_JOBS.delete(job.jobId)
  }
}

export function cancelReindex(jobId: string) {
  const entry = RUNNING_JOBS.get(jobId)
  if (entry) {
    entry.abort.abort()
  }
}

export function getRunningJobTail(jobId: string): string | null {
  const entry = RUNNING_JOBS.get(jobId)
  return entry ? entry.outputTail : null
}
