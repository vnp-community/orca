import path from 'path'
import os from 'os'
import fs from 'fs'
import { execFile } from 'child_process'
import { promisify } from 'util'

const exec = promisify(execFile)

let goPathCache: { path: string; time: number } | null = null

export interface BuildQualityToolPathDeps {
  home: string
}

export async function buildQualityToolPath(
  config: { toolPath?: string },
  deps: BuildQualityToolPathDeps
): Promise<string> {
  const paths: string[] = []

  if (config.toolPath) paths.push(...config.toolPath.split(path.delimiter))

  paths.push(path.join(deps.home, 'go', 'bin'))

  const now = Date.now()
  let goPath = ''
  if (goPathCache && now - goPathCache.time < 60000) {
    goPath = goPathCache.path
  } else {
    try {
      const { stdout } = await exec('go', ['env', 'GOPATH'], { timeout: 5000 })
      goPath = stdout.trim()
      goPathCache = { path: goPath, time: now }
    } catch {
      // ignore
    }
  }

  if (goPath) {
    paths.push(path.join(goPath, 'bin'))
  }

  paths.push('/usr/local/go/bin')
  paths.push(path.join(deps.home, '.local', 'share', 'pnpm'))

  if (process.env.PATH) {
    paths.push(...process.env.PATH.split(path.delimiter))
  }

  // Deduplicate and resolve
  const unique = new Set<string>()
  for (const p of paths) {
    if (p) unique.add(path.resolve(p))
  }

  return Array.from(unique).join(path.delimiter)
}

export interface ResolveQualityBinaryCtx {
  cwd: string
  repoRoot: string
  qualityToolPath: string
}

export class QualityBinaryError extends Error {
  constructor(public reason: string) {
    super(reason)
  }
}

export async function resolveQualityBinary(
  name: string,
  ctx: ResolveQualityBinaryCtx
): Promise<string> {
  const candidates = [
    path.join(ctx.cwd, 'node_modules', '.bin', name),
    path.join(ctx.repoRoot, 'node_modules', '.bin', name)
  ]

  const sysPaths = ctx.qualityToolPath.split(path.delimiter)
  for (const sp of sysPaths) {
    candidates.push(path.join(sp, name))
    if (process.platform === 'win32') {
      candidates.push(path.join(sp, name + '.cmd'))
      candidates.push(path.join(sp, name + '.exe'))
    }
  }

  for (const c of candidates) {
    try {
      await fs.promises.access(c, fs.constants.X_OK)
      return c
    } catch {
      // continue
    }
  }

  throw new QualityBinaryError('binary_missing')
}
