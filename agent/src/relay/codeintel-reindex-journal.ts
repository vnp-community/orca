import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

export type JobStatus = 'running' | 'completed' | 'failed' | 'interrupted' | 'cancelled'

export interface JobState {
  jobId: string
  tool: string
  mode: string
  workspaceRoot: string
  status: JobStatus
  startedAt: number
  finishedAt: number | null
}

function getJobsDir(): string | null {
  const home = process.env.HOME || os.homedir()
  if (!home) return null
  return path.join(home, '.orca', 'codeintel', 'jobs')
}

function ensureJobsDir(dir: string): boolean {
  try {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 })
    return true
  } catch (err: any) {
    if (err.code === 'EEXIST') return true
    return false
  }
}

export function writeJobStart(job: Omit<JobState, 'finishedAt' | 'status'> & { status?: JobStatus }) {
  const dir = getJobsDir()
  if (!dir || !ensureJobsDir(dir)) return

  const state: JobState = {
    ...job,
    status: job.status || 'running',
    finishedAt: null
  }

  const file = path.join(dir, `${job.jobId}.json`)
  const tmpFile = `${file}.${Date.now()}.tmp`

  try {
    fs.writeFileSync(tmpFile, JSON.stringify(state), { mode: 0o600 })
    fs.renameSync(tmpFile, file)
  } catch {
    if (fs.existsSync(tmpFile)) {
      try { fs.unlinkSync(tmpFile) } catch {}
    }
  }
}

export function writeJobFinish(jobId: string, status: JobStatus) {
  const dir = getJobsDir()
  if (!dir) return
  const file = path.join(dir, `${jobId}.json`)
  try {
    if (!fs.existsSync(file)) return
    const content = fs.readFileSync(file, 'utf8')
    const state = JSON.parse(content) as JobState
    state.status = status
    state.finishedAt = Date.now()

    const tmpFile = `${file}.${Date.now()}.tmp`
    fs.writeFileSync(tmpFile, JSON.stringify(state), { mode: 0o600 })
    fs.renameSync(tmpFile, file)
  } catch {
    // ignore
  }
}

export function markRunningJobsInterrupted() {
  const dir = getJobsDir()
  if (!dir) return
  try {
    if (!fs.existsSync(dir)) return
    const files = fs.readdirSync(dir).filter(f => f.endsWith('.json'))
    for (const f of files) {
      const p = path.join(dir, f)
      try {
        const content = fs.readFileSync(p, 'utf8')
        const state = JSON.parse(content) as JobState
        if (state.status === 'running') {
          state.status = 'interrupted'
          state.finishedAt = Date.now()
          const tmpFile = `${p}.${Date.now()}.tmp`
          fs.writeFileSync(tmpFile, JSON.stringify(state), { mode: 0o600 })
          fs.renameSync(tmpFile, p)
        }
      } catch {}
    }
  } catch {}
}

export function pruneJournal(keepCount: number = 50) {
  const dir = getJobsDir()
  if (!dir) return
  try {
    if (!fs.existsSync(dir)) return
    const files = fs.readdirSync(dir)
      .filter(f => f.endsWith('.json'))
      .map(f => {
        const p = path.join(dir, f)
        const stat = fs.statSync(p)
        return { p, mtime: stat.mtimeMs }
      })
      .sort((a, b) => b.mtime - a.mtime)

    for (let i = keepCount; i < files.length; i++) {
      try { fs.unlinkSync(files[i].p) } catch {}
    }
  } catch {}
}

markRunningJobsInterrupted()
