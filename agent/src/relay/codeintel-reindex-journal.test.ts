import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import {
  writeJobStart,
  writeJobFinish,
  markRunningJobsInterrupted,
  pruneJournal
} from './codeintel-reindex-journal'

describe('codeintel-reindex-journal', () => {
  let tmpHome: string

  beforeEach(() => {
    tmpHome = fs.mkdtempSync(path.join(process.cwd(), 'tmp-home-'))
    process.env.HOME = tmpHome
  })

  afterEach(() => {
    try {
      fs.rmSync(tmpHome, { recursive: true, force: true })
    } catch {}
  })

  it('writeJobStart and writeJobFinish', () => {
    writeJobStart({
      jobId: 'j1',
      tool: 'gitnexus',
      mode: 'full',
      workspaceRoot: '/repo',
      startedAt: 1000
    })

    const p = path.join(tmpHome, '.orca/codeintel/jobs/j1.json')
    expect(fs.existsSync(p)).toBe(true)
    
    let content = JSON.parse(fs.readFileSync(p, 'utf8'))
    expect(content.status).toBe('running')
    expect(content.finishedAt).toBe(null)

    writeJobFinish('j1', 'completed')
    content = JSON.parse(fs.readFileSync(p, 'utf8'))
    expect(content.status).toBe('completed')
    expect(content.finishedAt).not.toBe(null)
  })

  it('markRunningJobsInterrupted', () => {
    writeJobStart({ jobId: 'r1', tool: 'gitnexus', mode: 'full', workspaceRoot: '/repo', startedAt: 1000 })
    writeJobStart({ jobId: 'r2', tool: 'gitnexus', mode: 'full', workspaceRoot: '/repo', startedAt: 1000 })
    writeJobFinish('r2', 'completed')

    markRunningJobsInterrupted()

    const c1 = JSON.parse(fs.readFileSync(path.join(tmpHome, '.orca/codeintel/jobs/r1.json'), 'utf8'))
    const c2 = JSON.parse(fs.readFileSync(path.join(tmpHome, '.orca/codeintel/jobs/r2.json'), 'utf8'))

    expect(c1.status).toBe('interrupted')
    expect(c2.status).toBe('completed')
  })

  it('pruneJournal', () => {
    for (let i = 0; i < 55; i++) {
      const p = path.join(tmpHome, '.orca/codeintel/jobs', `j${i}.json`)
      fs.mkdirSync(path.dirname(p), { recursive: true })
      fs.writeFileSync(p, JSON.stringify({ jobId: `j${i}` }))
      // manually alter mtime to sort
      fs.utimesSync(p, new Date(Date.now() - i * 1000), new Date(Date.now() - i * 1000))
    }

    pruneJournal(50)
    const files = fs.readdirSync(path.join(tmpHome, '.orca/codeintel/jobs'))
    expect(files.length).toBe(50)
  })

  it('ignores read-only HOME without throwing', () => {
    const roHome = path.join(tmpHome, 'ro')
    fs.mkdirSync(roHome, { mode: 0o500 }) // read/execute only
    process.env.HOME = roHome

    expect(() => {
      writeJobStart({ jobId: 'j1', tool: 'gitnexus', mode: 'full', workspaceRoot: '/repo', startedAt: 1000 })
    }).not.toThrow()
  })
})
