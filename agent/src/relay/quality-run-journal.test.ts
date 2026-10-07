import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { writeJournal, readJournal, trimJournal, recoverInterruptedRuns, JournalEntry } from './quality-run-journal'
import fs from 'fs'
import path from 'path'
import os from 'os'

const { mockHomedir } = vi.hoisted(() => ({
  mockHomedir: vi.fn()
}))

vi.mock('os', () => ({
  default: { homedir: mockHomedir, tmpdir: () => '/tmp' },
  homedir: mockHomedir,
  tmpdir: () => '/tmp'
}))

vi.mock('node:os', () => ({
  default: { homedir: mockHomedir, tmpdir: () => '/tmp' },
  homedir: mockHomedir,
  tmpdir: () => '/tmp'
}))

describe('quality-run-journal', () => {
  let tmpHome: string

  beforeEach(() => {
    tmpHome = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-journal-test-'))
    mockHomedir.mockReturnValue(tmpHome)
  })

  afterEach(() => {
    fs.rmSync(tmpHome, { recursive: true, force: true })
    vi.clearAllMocks()
  })

  it('writes and reads journal atomically', () => {
    const entry: JournalEntry = {
      runId: 'qr_abc12345',
      workspaceRoot: '/repo',
      state: 'running',
      startedAt: 1234,
      steps: []
    }
    
    writeJournal(entry)
    
    const read = readJournal('qr_abc12345')
    expect(read).toBeDefined()
    expect(read?.state).toBe('running')
    
    // Invalid run ID
    expect(readJournal('invalid')).toBeNull()
  })

  it('trims to max count', () => {
    for (let i = 0; i < 55; i++) {
      const pad = String(i).padStart(6, '0')
      writeJournal({
        runId: `qr_10${pad}`,
        workspaceRoot: '/repo',
        state: 'completed',
        startedAt: 1000 + i,
        steps: []
      })
    }
    
    trimJournal(50)
    
    const dir = path.join(tmpHome, '.orca', 'quality', 'runs')
    const files = fs.readdirSync(dir)
    expect(files.length).toBe(50)
  })

  it('recovers interrupted runs', () => {
    writeJournal({ runId: 'qr_11111111', workspaceRoot: '/r', state: 'queued', startedAt: 1, steps: [] })
    writeJournal({ runId: 'qr_22222222', workspaceRoot: '/r', state: 'running', startedAt: 1, steps: [] })
    writeJournal({ runId: 'qr_33333333', workspaceRoot: '/r', state: 'completed', startedAt: 1, steps: [] })
    
    recoverInterruptedRuns()
    
    expect(readJournal('qr_11111111')?.state).toBe('interrupted')
    expect(readJournal('qr_22222222')?.state).toBe('interrupted')
    expect(readJournal('qr_33333333')?.state).toBe('completed') // unchanged
  })
})
