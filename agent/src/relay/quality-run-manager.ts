import fs from 'fs'
import { generateRunId } from './quality-run-id'
import { writeJournal, trimJournal, JournalEntry, JournalStep } from './quality-run-journal'
import { PlannedStep, StepExecResult } from './quality-run-types'
import { ORCA_QUALITY_QUEUE_MAX, ORCA_QUALITY_RUN_TIMEOUT_MS } from './quality-limits'

export interface RunPlan {
  workspaceRoot: string
  steps: PlannedStep[]
}

export interface RunManagerDeps {
  executeStep: (step: PlannedStep, ctx: any) => Promise<StepExecResult>
  parse?: (parserId: string, stdoutPath: string, stderrPath: string) => Promise<any>
  heavyGate?: any
  journal: {
    write: (entry: JournalEntry) => void
    trim: () => void
  }
  emit: (event: string, payload: any) => void
  now: () => number
  fingerprintOf: (root: string) => Promise<string>
  tmpDir?: string
}

export interface ActiveRun {
  runId: string
  workspaceRoot: string
  realPath: string
  state: 'queued' | 'running' | 'cancelling' | 'completed' | 'failed' | 'timeout' | 'cancelled'
  startedAt: number
  plan: RunPlan
  ac: AbortController
  journal: JournalEntry
  runDir: string
  startFingerprint: string
  timer?: NodeJS.Timeout
}

export function createRunManager(deps: RunManagerDeps) {
  const activeRuns = new Map<string, ActiveRun>()
  const queue: ActiveRun[] = []
  
  function getRealPath(p: string) {
    try {
      return fs.realpathSync(p)
    } catch {
      return p
    }
  }

  function pump() {
    // maxConcurrentRuns = 1
    const running = Array.from(activeRuns.values()).filter(r => r.state === 'running' || r.state === 'cancelling')
    if (running.length > 0) return

    const next = queue.shift()
    if (next) {
      startRun(next).catch(console.error)
    }
  }

  async function startRun(run: ActiveRun) {
    run.state = 'running'
    run.journal.state = 'running'
    run.startedAt = deps.now()
    run.journal.startedAt = run.startedAt
    deps.journal.write(run.journal)

    let timeoutFired = false
    run.timer = setTimeout(() => {
      timeoutFired = true
      run.ac.abort()
    }, ORCA_QUALITY_RUN_TIMEOUT_MS)

    let failed = false
    
    for (const step of run.plan.steps) {
      const jStep: JournalStep = { id: step.id, tool: step.parser, state: 'running', startedAt: deps.now() }
      run.journal.steps.push(jStep)
      
      if (run.ac.signal.aborted || timeoutFired) {
        jStep.state = 'skipped'
        jStep.finishedAt = deps.now()
        continue
      }

      const ctx = {
        runDir: run.runDir,
        signal: run.ac.signal,
        heavyGate: deps.heavyGate
      }

      const res = await deps.executeStep(step, ctx)
      
      jStep.finishedAt = deps.now()
      jStep.durationMs = res.durationMs
      jStep.exitCode = res.exitCode

      if (res.kind === 'success') {
        jStep.state = 'passed'
        // Mock parsing
        if (deps.parse && step.parser) {
           // We might call parse here
        }
      } else if (res.kind === 'cancelled') {
        jStep.state = 'cancelled'
      } else if (res.kind === 'timeout') {
        jStep.state = 'timeout'
        failed = true
      } else {
        jStep.state = 'failed'
        failed = true
      }
    }

    if (run.timer) clearTimeout(run.timer)

    // endFingerprint unused

    if (timeoutFired) {
      run.state = 'timeout'
    } else if (run.ac.signal.aborted) {
      run.state = 'cancelled'
    } else if (failed) {
      run.state = 'failed'
    } else {
      run.state = 'completed'
    }

    run.journal.state = run.state
    run.journal.finishedAt = deps.now()
    
    deps.journal.write(run.journal)
    deps.journal.trim()
    activeRuns.delete(run.runId)
    
    pump()
  }

  return {
    async submit(plan: RunPlan) {
      const realPath = getRealPath(plan.workspaceRoot)
      
      const existing = Array.from(activeRuns.values()).find(r => r.realPath === realPath && ['queued', 'running', 'cancelling'].includes(r.state))
      if (existing) {
        const err = new Error('Worktree is busy')
        ;(err as any).code = 'worktree_busy'
        ;(err as any).runId = existing.runId
        throw err
      }

      if (queue.length >= ORCA_QUALITY_QUEUE_MAX) {
        const err = new Error('Queue is full')
        ;(err as any).code = 'queue_full'
        throw err
      }

      const startFingerprint = await deps.fingerprintOf(plan.workspaceRoot).catch(() => '')
      const runId = generateRunId()
      const runDir = deps.tmpDir ? `${deps.tmpDir}/${runId}` : `/tmp/orca-quality/${runId}`

      const run: ActiveRun = {
        runId,
        workspaceRoot: plan.workspaceRoot,
        realPath,
        state: 'queued',
        startedAt: 0,
        plan,
        ac: new AbortController(),
        runDir,
        startFingerprint,
        journal: {
          runId,
          workspaceRoot: plan.workspaceRoot,
          state: 'queued',
          startedAt: 0,
          steps: []
        }
      }

      activeRuns.set(runId, run)
      queue.push(run)
      
      deps.journal.write(run.journal)
      
      // Async pump
      setTimeout(pump, 0)

      return {
        runId,
        state: 'queued',
        queuePosition: queue.length,
        dirtyFingerprint: startFingerprint
      }
    },

    getStatus(workspaceRoot: string, runId: string) {
      const r = activeRuns.get(runId)
      if (!r || r.workspaceRoot !== workspaceRoot) return null
      return { state: r.state, queuePosition: queue.indexOf(r) + 1 }
    },

    cancel(workspaceRoot: string, runId: string) {
      const r = activeRuns.get(runId)
      if (!r || r.workspaceRoot !== workspaceRoot) return
      
      if (r.state === 'queued') {
        const idx = queue.indexOf(r)
        if (idx !== -1) queue.splice(idx, 1)
        r.state = 'cancelled'
        r.journal.state = 'cancelled'
        deps.journal.write(r.journal)
        activeRuns.delete(runId)
      } else if (r.state === 'running') {
        r.state = 'cancelling'
        r.ac.abort()
      }
    },

    getRun(runId: string) {
      return activeRuns.get(runId) || null
    },

    list() {
      return Array.from(activeRuns.values()).map(r => ({ runId: r.runId, state: r.state }))
    }
  }
}

let defaultRunManager: ReturnType<typeof createRunManager> | null = null

export function getQualityRunManager(): ReturnType<typeof createRunManager> {
  if (!defaultRunManager) {
    defaultRunManager = createRunManager({
      executeStep: async () => ({ exitCode: 0, durationMs: 0, stdoutPath: '', stderrPath: '', timedOut: false }),
      journal: { write: () => {}, trim: () => {} },
      emit: () => {},
      now: () => Date.now(),
      fingerprintOf: async () => 'fp_default'
    })
  }
  return defaultRunManager
}

export function setQualityRunManager(mgr: ReturnType<typeof createRunManager> | null): void {
  defaultRunManager = mgr
}
