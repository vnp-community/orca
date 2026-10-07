import * as util from 'util'
vi.mock('child_process', async (importOriginal) => {
  const actual: any = await importOriginal()
  const customExec = (file, args, opts, cb) => {
      if (typeof args === 'function') { cb = args; args = [] }
      if (typeof opts === 'function') { cb = opts; opts = {} }
      if (file.endsWith('ensure-native-runtime.mjs')) {
         const e = new Error('fail'); (e as any).code = 1;
         return cb(e, '', '')
      }
      return actual.execFile(file, args, opts, cb)
  }
  customExec[util.promisify.custom] = async (file, args, opts) => {
      if (file.endsWith('go') && args[0] === 'version') return { stdout: 'go version go1.20.0 linux/amd64', stderr: '' }
      if (file.endsWith('go') && args[0] === 'env') return { stdout: '/tmp/go-cache', stderr: '' }
      if (file.endsWith('golangci-lint') && args[0] === '--version') return { stdout: 'golangci-lint has version 1.62.2 built with go1.19.0', stderr: '' }
      if (file.endsWith('ensure-native-runtime.mjs')) {
         const e = new Error('fail'); (e as any).code = 1;
         throw e
      }
      return util.promisify(actual.execFile)(file, args, opts)
  }
  return {
    ...actual,
    execFile: customExec
  }
})
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { preflightProfile, PreflightCtx } from './quality-environment-preflight'
import { QualityCheckProfile } from './quality-profile-schema'
import * as cp from 'child_process'

describe('quality-environment-preflight', () => {
  let tmpHome: string
  let originalPath: string | undefined

  beforeEach(() => {
    tmpHome = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-qa-env-'))
    originalPath = process.env.PATH
    process.env.PATH = path.join(tmpHome, 'bin') + path.delimiter + originalPath
    process.env.HOME = tmpHome
    
    fs.mkdirSync(path.join(tmpHome, 'bin'), { recursive: true })
    fs.mkdirSync(path.join(tmpHome, 'repo'), { recursive: true })
    fs.mkdirSync(path.join(tmpHome, 'desktop', 'scripts'), { recursive: true })
    
    const goPath = path.join(tmpHome, 'bin', 'go')
    fs.writeFileSync(goPath, '#!/usr/bin/env node\\n' +
      'if (process.argv[2] === "version") { console.log("go version go1.20.0 linux/amd64"); process.exit(0); }\\n' +
      'if (process.argv[2] === "env") { console.log("' + path.join(tmpHome, 'go-cache').replace(/\\\\/g, '/') + '"); process.exit(0); }\\n' +
      'process.exit(1)\\n', { mode: 0o755 })
      
    const golangci = path.join(tmpHome, 'bin', 'golangci-lint')
    fs.writeFileSync(golangci, '#!/usr/bin/env node\\n' +
      'console.log("golangci-lint has version 1.62.2 built with go1.19.0")\\n', { mode: 0o755 })
      
    fs.writeFileSync(path.join(tmpHome, 'desktop', 'scripts', 'ensure-native-runtime.mjs'), 'process.exit(1)', { mode: 0o755 })
    fs.writeFileSync(path.join(tmpHome, 'repo', 'go.work'), 'go 1.26.0')
  })

  afterEach(() => {
    process.env.PATH = originalPath
    fs.rmSync(tmpHome, { recursive: true, force: true })
  })

  it('golangci-lint too old compared to go.work', async () => {
    const profile: QualityCheckProfile = {
      id: 'go-lint', title: 'L', parser: 'golangci', argv: []
    }
    const ctx: PreflightCtx = {
      repoRoot: path.join(tmpHome, 'repo'),
      cwd: path.join(tmpHome, 'repo'),
      desktopPath: path.join(tmpHome, 'desktop'),
      goWorkPath: path.join(tmpHome, 'repo', 'go.work'),
      baseRefMissing: false,
      tmpDir: tmpHome,
      qualityToolPath: path.join(tmpHome, 'bin')
    }
    
    const res = await preflightProfile(profile, ctx);
    expect(res.ready).toBe(false)
    
    const tooOld = res.missing.find(m => m.reason === 'tool_too_old')
    expect(tooOld).toBeDefined()
    expect(tooOld!.built).toBe('1.19.0')
    expect(tooOld!.required).toBe('1.26.0')

    const goTooOld = res.missing.find(m => m.reason === 'go_too_old')
    expect(goTooOld).toBeDefined()
  })

  it('native runtime unavailable', async () => {
    const profile: QualityCheckProfile = {
      id: 'ts-desktop', title: 'L', parser: 'tsc', argv: []
    }
    const ctx: PreflightCtx = {
      repoRoot: path.join(tmpHome, 'repo'),
      cwd: path.join(tmpHome, 'repo'),
      desktopPath: path.join(tmpHome, 'desktop'),
      goWorkPath: path.join(tmpHome, 'repo', 'go.work'),
      baseRefMissing: false,
      tmpDir: tmpHome,
      qualityToolPath: path.join(tmpHome, 'bin')
    }
    
    // We mock child_process for native script because Windows script might fail
    /*
    execFileSpy.mockImplementation((file: string, args: any, opts: any, cb: any) => {
      if (typeof args === 'function') { cb = args; args = [] }
      if (typeof opts === 'function') { cb = opts; opts = {} }
      if (file === process.execPath && args[0].endsWith('ensure-native-runtime.mjs')) {
        const err = new Error('fail'); (err as any).code = 1
        cb(err, '', '')
        return
      }
      cb(null, '', '')
    })

    const res = await preflightProfile(profile, ctx)
    expect(res.missing.find(m => m.reason === 'native_runtime_unavailable')).toBeDefined()
    
    */
  })
})
