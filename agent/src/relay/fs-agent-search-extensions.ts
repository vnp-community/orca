// src/relay/fs-agent-search-extensions.ts
// FS/preflight search RPC handlers for Orca Dev Agent v5.0. Split out of
// fs-agent-extensions.ts (max-lines ratchet) — pure code move, no behavior
// change. Covers: fs.grep, preflight.check.

import { spawn } from 'node:child_process'
import { join, isAbsolute } from 'node:path'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { checkRgAvailable } from './fs-handler-utils'
import { createTracer } from '../shared/trace'

const fsTracer = createTracer('agent:fs')
// Distinct from `agent:fs` (used for fs.* RPC methods) — preflight.check is a
// separate concern (binary/tool availability probing).
const preflightTracer = createTracer('agent:preflight')

// ─── fs.grep ─────────────────────────────────────────────────────────────────

type GrepMatch = {
  file: string
  line: number
  text: string
  source?: 'stderr'
}

export async function handleFsGrep(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const root = typeof params.root === 'string' && params.root ? params.root : config.workDir
  const pattern = typeof params.pattern === 'string' ? params.pattern : ''
  const maxResults = typeof params.maxResults === 'number' ? Math.min(params.maxResults, 200) : 50
  const span = fsTracer.start({ method: 'fs.grep', pattern: pattern || '(empty)', root })

  if (!pattern) {
    span.fail('missing param: pattern', { method: 'fs.grep' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: pattern' }
    }
  }

  const absRoot = isAbsolute(root) ? root : join(config.workDir, root)
  const rgAvailable = await checkRgAvailable()

  try {
    const matches = rgAvailable
      ? await grepWithRg(pattern, absRoot, maxResults, config)
      : await grepFallback(pattern, absRoot, maxResults, config)

    span.ok({
      pattern,
      matches: matches.length,
      truncated: matches.length >= maxResults,
      tool: rgAvailable ? 'rg' : 'grep'
    })
    return {
      jsonrpc: '2.0',
      id,
      result: { matches, total: matches.length, truncated: matches.length >= maxResults }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { pattern, root: absRoot })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

function grepWithRg(
  pattern: string,
  root: string,
  maxResults: number,
  config: AgentConfig
): Promise<GrepMatch[]> {
  return new Promise((resolve, reject) => {
    const child = spawn(
      'rg',
      ['--json', '--ignore-case', '--max-count', String(maxResults), pattern, root],
      { env: config.toolEnv, stdio: ['pipe', 'pipe', 'pipe'], shell: false }
    )
    let output = ''
    child.stdout?.on('data', (c: Buffer) => {
      output += c.toString()
    })
    child.on('close', (code) => {
      // rg exits 0 on match, 1 on no match — both are success
      if (code !== 0 && code !== 1) {
        reject(new Error(`rg exited with code ${code}`))
        return
      }
      const matches: GrepMatch[] = []
      for (const line of output.split('\n')) {
        if (!line.trim()) {
          continue
        }
        try {
          const obj = JSON.parse(line) as {
            type: string
            data: { path: { text: string }; line_number: number; lines: { text: string } }
          }
          if (obj.type === 'match') {
            matches.push({
              file: obj.data.path.text,
              line: obj.data.line_number,
              text: obj.data.lines.text.trimEnd()
            })
            if (matches.length >= maxResults) {
              break
            }
          }
        } catch {
          /* skip non-JSON lines */
        }
      }
      resolve(matches)
    })
    child.on('error', reject)
    child.stdin?.end()
  })
}

function grepFallback(
  pattern: string,
  root: string,
  maxResults: number,
  config: AgentConfig
): Promise<GrepMatch[]> {
  return new Promise((resolve, reject) => {
    const child = spawn(
      'grep',
      [
        '-r',
        '-n',
        '-i',
        '--include=*.ts',
        '--include=*.tsx',
        '--include=*.js',
        '--include=*.go',
        '--include=*.py',
        '--include=*.md',
        pattern,
        root
      ],
      { env: config.toolEnv, stdio: ['pipe', 'pipe', 'pipe'], shell: false }
    )
    let output = ''
    child.stdout?.on('data', (c: Buffer) => {
      output += c.toString()
    })
    child.on('close', (code) => {
      if (code !== 0 && code !== 1) {
        reject(new Error(`grep exited with code ${code}`))
        return
      }
      const matches: GrepMatch[] = []
      for (const raw of output.split('\n')) {
        const m = raw.match(/^(.+?):(\d+):(.*)$/)
        if (m) {
          matches.push({ file: m[1], line: Number.parseInt(m[2], 10), text: m[3] })
          if (matches.length >= maxResults) {
            break
          }
        }
      }
      resolve(matches)
    })
    child.on('error', reject)
    child.stdin?.end()
  })
}

// ─── preflight.check ─────────────────────────────────────────────────────────

export async function handlePreflightCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const services = Array.isArray(params.services) ? params.services.map(String) : []
  const results: Record<string, boolean> = {}
  const span = preflightTracer.start({ services: services.join(',') || '(empty)' })

  await Promise.all(
    services.map(async (service) => {
      try {
        switch (service) {
          case 'github-cli':
            results[service] = await checkBinaryAvailable('gh', config)
            break
          case 'ripgrep':
            results[service] = await checkRgAvailable()
            break
          case 'docker':
            results[service] = await checkBinaryAvailable('docker', config)
            break
          case 'claude':
            results[service] = await checkBinaryAvailable('claude', config)
            break
          default:
            results[service] = false
        }
      } catch {
        results[service] = false
      }
    })
  )

  // Business-level fail (some service unavailable) is distinguished from an
  // exception path, per CR-TRACE-014.
  const failedServices = Object.entries(results)
    .filter(([, ok]) => !ok)
    .map(([svc]) => svc)
  if (failedServices.length > 0) {
    span.fail(`unavailable: ${failedServices.join(',')}`, { failedCount: failedServices.length })
  } else {
    span.ok({ checkedCount: services.length })
  }

  return { jsonrpc: '2.0', id, result: results }
}

function checkBinaryAvailable(binary: string, config: AgentConfig): Promise<boolean> {
  return new Promise((resolve) => {
    const child = spawn(binary, ['--version'], {
      env: config.toolEnv,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false
    })
    const timer = setTimeout(() => {
      child.kill()
      resolve(false)
    }, 5_000)
    child.on('close', (code) => {
      clearTimeout(timer)
      resolve(code === 0)
    })
    child.on('error', () => {
      clearTimeout(timer)
      resolve(false)
    })
    child.stdin?.end()
  })
}
