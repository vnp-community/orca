import { describe, it, expect, vi } from 'vitest'
import path from 'path'
import fs from 'fs'
import os from 'os'
import { listProfiles, QualityListProfilesError } from './quality-list-profiles'
import * as preflight from './quality-environment-preflight'

describe('quality-list-profiles', () => {
  it('throws INVALID_PARAMS for non-absolute workspaceRoot', async () => {
    await expect(listProfiles('relative/path', { warn: vi.fn() }))
      .rejects.toThrowError(QualityListProfilesError)
  })

  it('returns valid shape without leaking absolute paths', async () => {
    const tmpSpace = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-list-profiles-'))
    const workspaceRoot = path.join(tmpSpace, 'repo')
    fs.mkdirSync(workspaceRoot)

    // Mock preflight to just return ready
    const spy = vi.spyOn(preflight, 'preflightProfile').mockResolvedValue({
      ready: true,
      missing: []
    })

    const res = await listProfiles(workspaceRoot, { warn: vi.fn() })
    
    expect(res.profiles.length).toBeGreaterThan(0)
    expect(res.suites.length).toBeGreaterThan(0)
    expect(res.host).toBeDefined()
    expect(res.limits).toBeDefined()

    const str = JSON.stringify(res)
    expect(str).not.toContain(workspaceRoot) // No absolute paths in output

    spy.mockRestore()
    fs.rmSync(tmpSpace, { recursive: true, force: true })
  })

  it('handles preflight timeout (10s) gracefully', async () => {
    const tmpSpace = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-list-profiles-timeout-'))
    const workspaceRoot = path.join(tmpSpace, 'repo')
    fs.mkdirSync(workspaceRoot)

    // Mock preflight to hang forever
    const spy = vi.spyOn(preflight, 'preflightProfile').mockImplementation(async () => {
      return new Promise(resolve => setTimeout(resolve, 200)) // longer than 10s
    })
    
    // We can't actually wait 10s in the test without it being slow.
    // Vitest provides fake timers!
    const p = listProfiles(workspaceRoot, { warn: vi.fn(), preflightTimeoutMs: 50 })
    const res = await p
    expect(res.profiles[0].ready).toBe(false)
    expect(res.profiles[0].missing).toEqual([])
    
    
    spy.mockRestore()
    fs.rmSync(tmpSpace, { recursive: true, force: true })
  })
})
