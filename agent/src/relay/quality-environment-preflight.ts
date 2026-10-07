import fs from 'fs'
import path from 'path'
import os from 'os'
import { execFile } from 'child_process'
import { promisify } from 'util'
import { QualityCheckProfile } from './quality-profile-schema'
import { QualityEnvMissing } from './quality-run-types'
import { resolveQualityBinary } from './quality-tool-resolution'

const exec = promisify(execFile)

export interface PreflightCtx {
  repoRoot: string
  cwd: string
  desktopPath: string
  goWorkPath: string
  baseRefMissing: boolean
  tmpDir: string
  qualityToolPath: string
}

export interface PreflightResult {
  ready: boolean
  missing: QualityEnvMissing[]
}

const cache = new Map<string, { result: PreflightResult; time: number }>()

export async function preflightProfile(
  profile: QualityCheckProfile,
  ctx: PreflightCtx
): Promise<PreflightResult> {
  const cacheKey = `${ctx.repoRoot}:${profile.id}`
  const now = Date.now()
  const cached = cache.get(cacheKey)
  if (cached && now - cached.time < 60000) {
    return cached.result
  }

  const missing: QualityEnvMissing[] = []

  // 1. HOME missing
  if (!process.env.HOME) {
    missing.push({ reason: 'home_missing' })
  }

  // 2. tmp_space_low (requires Node 18+ statfs)
  try {
    const st = await fs.promises.statfs(ctx.tmpDir)
    const free = st.bavail * st.bsize
    if (free < 256 * 1024 * 1024) {
      missing.push({ reason: 'tmp_space_low' })
    }
  } catch (e) { console.log('GO EXEC ERROR', e);
    // ignore
  }

  // 3. network_policy
  if (profile.network && process.env.ORCA_QUALITY_NETWORK === 'deny') {
    missing.push({ reason: 'network_policy' })
  }

  // 4. base_ref_missing
  if (ctx.baseRefMissing && profile.argv.includes('{base}')) {
    missing.push({ reason: 'base_ref_missing' })
  }

  // 5. Node dependencies
  if (profile.parser === 'oxlint' || profile.parser === 'tsc' || profile.parser === 'vitest' || profile.id.includes('ts-')) {
    const modYaml = path.join(ctx.cwd, 'node_modules', '.modules.yaml')
    const modYamlRepo = path.join(ctx.repoRoot, 'node_modules', '.modules.yaml')
    let found = false
    try { await fs.promises.access(modYaml); found = true } catch (e) { console.log('GO EXEC ERROR', e);}
    if (!found) {
      try { await fs.promises.access(modYamlRepo); found = true } catch {}
    }
    if (!found) {
      missing.push({ reason: 'node_modules_missing' })
    }
  }

  // 6. Native runtime
  if (profile.id.includes('desktop') || profile.id.includes('agent')) {
    try {
      const script = path.join(ctx.desktopPath, 'scripts', 'ensure-native-runtime.mjs')
      if (fs.existsSync(script)) {
        await exec(process.execPath, [script, '--check-only'], { cwd: ctx.desktopPath, timeout: 15000 })
      }
    } catch (e: any) {
      if (e.code !== 'ENOENT') {
        missing.push({ reason: 'native_runtime_unavailable' })
      }
    }
  }

  // 7. Go checks
  if (profile.parser === 'govet' || profile.parser === 'golangci' || profile.parser === 'gotest' || profile.id.includes('go-')) {
    let requiredGo = '1.20'
    try {
      const goWork = await fs.promises.readFile(ctx.goWorkPath, 'utf8')
      const m = /^go\s+([0-9.]+)/m.exec(goWork)
      if (m) requiredGo = m[1]
    } catch {}

    let goVer = ''
    try {
      const { stdout } = await exec(await resolveQualityBinary('go', ctx), ['version'], { timeout: 5000 })
      const m = /go([0-9.]+)/.exec(stdout)
      if (m) goVer = m[1]
    } catch {
      missing.push({ reason: 'go_missing' })
    }

    if (goVer && goVer < requiredGo) {
      missing.push({ reason: 'go_too_old', required: requiredGo, built: goVer })
    }

    try {
      const { stdout } = await exec(await resolveQualityBinary('go', ctx), ['env', 'GOMODCACHE'], { timeout: 5000 })
      const cacheDir = stdout.trim()
      if (!cacheDir) throw new Error()
      const files = await fs.promises.readdir(cacheDir)
      if (files.length === 0) throw new Error()
    } catch {
      missing.push({ reason: 'go_modcache_empty' })
    }
  }

  // 8. golangci-lint
  if (profile.parser === 'golangci') {
    try {
      const bin = await resolveQualityBinary('golangci-lint', ctx)
      const { stdout } = await exec(bin, ['--version'], { timeout: 5000 })
      const m = /version\s+([0-9.]+).*built with go([0-9.]+)/.exec(stdout)
      if (m) {
        const ver = m[1]
        const built = m[2]
        if (!ver.startsWith('1.')) {
          missing.push({ reason: 'tool_incompatible' })
        }
        // we reuse requiredGo from earlier
        let requiredGo = '1.20'
        try {
          const goWork = await fs.promises.readFile(ctx.goWorkPath, 'utf8')
          const gm = /^go\s+([0-9.]+)/m.exec(goWork)
          if (gm) requiredGo = gm[1]
        } catch {}
        if (built < requiredGo) {
          missing.push({ reason: 'tool_too_old', built, required: requiredGo })
        }
      }
    } catch (e: any) {
      if (e.message === 'binary_missing') {
        missing.push({ reason: 'binary_missing', hint: 'golangci-lint' })
      }
    }
  }

  // buf / opa
  if (profile.parser === 'buf') {
    try {
      await resolveQualityBinary('buf', ctx)
    } catch {
      missing.push({ reason: 'binary_missing', hint: 'buf' })
    }
  }
  if (profile.parser === 'opa') {
    try {
      await resolveQualityBinary('opa', ctx)
    } catch {
      missing.push({ reason: 'binary_missing', hint: 'opa' })
    }
  }

  const result: PreflightResult = { ready: missing.length === 0, missing }
  cache.set(cacheKey, { result, time: now })
  return result
}
