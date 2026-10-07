import fs from 'fs'
import path from 'path'
import os from 'os'
import { isValidRunId } from './quality-run-id'

export interface JournalStep {
  id: string
  tool: string
  state: string
  startedAt?: number
  finishedAt?: number
  durationMs?: number
  exitCode?: number | null
}

export interface JournalEntry {
  runId: string
  workspaceRoot: string
  state: string
  startedAt: number
  finishedAt?: number
  pgid?: number
  steps: JournalStep[]
}

function getJournalDir(): string {
  return path.join(os.homedir(), '.orca', 'quality', 'runs')
}

export function writeJournal(entry: JournalEntry): void {
  const dir = getJournalDir()
  try {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true, mode: 0o700 })
    }
  } catch (err) {
    console.warn('Failed to create journal dir', err)
    return
  }

  const file = path.join(dir, `${entry.runId}.json`)
  const tmpFile = file + '.tmp'
  try {
    fs.writeFileSync(tmpFile, JSON.stringify(entry, null, 2), { mode: 0o600 })
    fs.renameSync(tmpFile, file)
  } catch (err) {
    console.warn(`Failed to write journal ${entry.runId}`, err)
  }
}

export function readJournal(runId: string): JournalEntry | null {
  if (!isValidRunId(runId)) return null
  const file = path.join(getJournalDir(), `${runId}.json`)
  try {
    const data = fs.readFileSync(file, 'utf8')
    return JSON.parse(data)
  } catch {
    return null
  }
}

export function trimJournal(keepCount = 50): void {
  const dir = getJournalDir()
  if (!fs.existsSync(dir)) return
  try {
    const files = fs.readdirSync(dir).filter(f => f.endsWith('.json'))
    if (files.length <= keepCount) return
    
    const withMtime = files.map(f => {
      const p = path.join(dir, f)
      return { file: p, mtime: fs.statSync(p).mtimeMs }
    })
    withMtime.sort((a, b) => b.mtime - a.mtime) // newest first
    
    for (let i = keepCount; i < withMtime.length; i++) {
      fs.unlinkSync(withMtime[i].file)
    }
  } catch (err) {
    console.warn('Failed to trim journal', err)
  }
}

export function recoverInterruptedRuns(): void {
  const dir = getJournalDir()
  if (!fs.existsSync(dir)) return
  try {
    const files = fs.readdirSync(dir).filter(f => f.endsWith('.json'))
    for (const file of files) {
      const p = path.join(dir, file)
      try {
        const data = fs.readFileSync(p, 'utf8')
        const entry = JSON.parse(data) as JournalEntry
        if (['queued', 'running', 'cancelling'].includes(entry.state)) {
          entry.state = 'interrupted'
          entry.finishedAt = Date.now()
          fs.writeFileSync(p, JSON.stringify(entry, null, 2))
          console.warn(`Run ${entry.runId} was interrupted`)
        }
      } catch (err) {
        // ignore invalid files
      }
    }
  } catch (err) {
    console.warn('Failed to recover interrupted runs', err)
  }
}
