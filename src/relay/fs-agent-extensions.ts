// src/relay/fs-agent-extensions.ts
// FS RPC handlers for Orca Dev Agent v5.0.
// Wraps existing fs-handler-*.ts modules for agent RPC methods:
//   fs.readDir, fs.readFile, fs.grep, preflight.check

import { readdir, stat, writeFile, mkdir, rmdir as fsRmdir, rm } from 'node:fs/promises'
import { join, isAbsolute, resolve as resolvePath, dirname } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { readRelayFileContent } from './fs-handler-file-read'
import { checkRgAvailable } from './fs-handler-utils'
import { createTracer } from '../shared/trace'

const fsTracer = createTracer('agent:fs')

// ─── fs.readDir ───────────────────────────────────────────────────────────────

interface FileTreeNode {
  path: string
  name: string
  type: 'file' | 'directory'
  size?: number
  children?: FileTreeNode[]
}

export async function handleFsReadDir(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const depth   = typeof params.depth === 'number' ? Math.min(params.depth, 5) : 1
  const span    = fsTracer.start({ method: 'fs.readDir', path: rawPath || '(empty)', depth })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.readDir' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    const st = await stat(absPath)
    if (!st.isDirectory()) {
      span.fail('not a directory', { path: absPath })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: `Not a directory: ${absPath}` } }
    }
    const entries = await readDirRecursive(absPath, depth, 1)
    span.ok({ path: absPath, entries: entries.length })
    return { jsonrpc: '2.0', id, result: { entries, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { path: absPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

async function readDirRecursive(
  dir: string,
  maxDepth: number,
  currentDepth: number
): Promise<FileTreeNode[]> {
  const entries = await readdir(dir, { withFileTypes: true })
  const nodes = await Promise.all(
    entries.map(async (entry): Promise<FileTreeNode> => {
      const fullPath = join(dir, entry.name)
      const node: FileTreeNode = {
        path: fullPath,
        name: entry.name,
        type: entry.isDirectory() ? 'directory' : 'file',
        size: entry.isFile() ? (await stat(fullPath)).size : undefined,
      }
      if (entry.isDirectory() && currentDepth < maxDepth) {
        node.children = await readDirRecursive(fullPath, maxDepth, currentDepth + 1)
      }
      return node
    })
  )
  // Sort: directories first, then files; alphabetically within each group
  return nodes.sort((a, b) => {
    if (a.type !== b.type) return a.type === 'directory' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

// ─── fs.readFile ─────────────────────────────────────────────────────────────

export async function handleFsReadFile(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const span    = fsTracer.start({ method: 'fs.readFile', path: rawPath || '(empty)' })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.readFile' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    // Reuse existing readRelayFileContent — handles size limits and binary detection
    const result = await readRelayFileContent(absPath)
    span.ok({ path: absPath, bytes: result.content.length, binary: result.isBinary })
    return {
      jsonrpc: '2.0', id,
      result: {
        content:  result.content,
        encoding: result.isBinary ? 'base64' : 'utf-8',
        isBinary: result.isBinary,
        path:     absPath,
      },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes('File too large') || msg.includes('MAX_TEXT_FILE_SIZE')) {
      span.fail('file too large', { path: absPath })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'FILE_TOO_LARGE' } }
    }
    if (msg.includes('ENOENT') || msg.includes('not found')) {
      span.fail('file not found', { path: absPath })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `File not found: ${absPath}` } }
    }
    span.fail(err, { path: absPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.grep ─────────────────────────────────────────────────────────────────

interface GrepMatch {
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
  const root       = typeof params.root === 'string' && params.root ? params.root : config.workDir
  const pattern    = typeof params.pattern === 'string' ? params.pattern : ''
  const maxResults = typeof params.maxResults === 'number' ? Math.min(params.maxResults, 200) : 50
  const span       = fsTracer.start({ method: 'fs.grep', pattern: pattern || '(empty)', root })

  if (!pattern) {
    span.fail('missing param: pattern', { method: 'fs.grep' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: pattern' } }
  }

  const absRoot = isAbsolute(root) ? root : join(config.workDir, root)
  const rgAvailable = await checkRgAvailable()

  try {
    const matches = rgAvailable
      ? await grepWithRg(pattern, absRoot, maxResults, config)
      : await grepFallback(pattern, absRoot, maxResults, config)

    span.ok({ pattern, matches: matches.length, truncated: matches.length >= maxResults, tool: rgAvailable ? 'rg' : 'grep' })
    return {
      jsonrpc: '2.0', id,
      result: { matches, total: matches.length, truncated: matches.length >= maxResults },
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
    child.stdout?.on('data', (c: Buffer) => { output += c.toString() })
    child.on('close', (code) => {
      // rg exits 0 on match, 1 on no match — both are success
      if (code !== 0 && code !== 1) { reject(new Error(`rg exited with code ${code}`)); return }
      const matches: GrepMatch[] = []
      for (const line of output.split('\n')) {
        if (!line.trim()) continue
        try {
          const obj = JSON.parse(line) as {
            type: string
            data: { path: { text: string }; line_number: number; lines: { text: string } }
          }
          if (obj.type === 'match') {
            matches.push({
              file: obj.data.path.text,
              line: obj.data.line_number,
              text: obj.data.lines.text.trimEnd(),
            })
            if (matches.length >= maxResults) break
          }
        } catch { /* skip non-JSON lines */ }
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
      ['-r', '-n', '-i',
       '--include=*.ts', '--include=*.tsx', '--include=*.js', '--include=*.go',
       '--include=*.py', '--include=*.md',
       pattern, root],
      { env: config.toolEnv, stdio: ['pipe', 'pipe', 'pipe'], shell: false }
    )
    let output = ''
    child.stdout?.on('data', (c: Buffer) => { output += c.toString() })
    child.on('close', (code) => {
      if (code !== 0 && code !== 1) { reject(new Error(`grep exited with code ${code}`)); return }
      const matches: GrepMatch[] = []
      for (const raw of output.split('\n')) {
        const m = raw.match(/^(.+?):(\d+):(.*)$/)
        if (m) {
          matches.push({ file: m[1], line: parseInt(m[2], 10), text: m[3] })
          if (matches.length >= maxResults) break
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

  await Promise.all(services.map(async (service) => {
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
  }))

  return { jsonrpc: '2.0', id, result: results }
}

function checkBinaryAvailable(binary: string, config: AgentConfig): Promise<boolean> {
  return new Promise((resolve) => {
    const child = spawn(binary, ['--version'], {
      env:   config.toolEnv,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,
    })
    const timer = setTimeout(() => { child.kill(); resolve(false) }, 5_000)
    child.on('close', (code) => { clearTimeout(timer); resolve(code === 0) })
    child.on('error', ()     => { clearTimeout(timer); resolve(false) })
    child.stdin?.end()
  })
}

// ─── fs.stat ──────────────────────────────────────────────────────────────────

export async function handleFsStat(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    const st = await stat(absPath)
    return {
      jsonrpc: '2.0', id,
      result: {
        path:   absPath,
        size:   st.size,
        mtime:  st.mtime.toISOString(),
        isDir:  st.isDirectory(),
        isFile: st.isFile(),
        isLink: st.isSymbolicLink(),
        mode:   st.mode.toString(8),
      },
    }
  } catch (err: unknown) {
    const nodeErr = err as NodeJS.ErrnoException
    if (nodeErr.code === 'ENOENT') {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Not found: ${absPath}` } }
    }
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.glob ──────────────────────────────────────────────────────────────────

/**
 * Glob files on dev server using `find` CLI (always available on Linux/macOS).
 * shell: false — no shell injection possible.
 * MAX_RESULTS: 200 entries to prevent memory explosion.
 */
export async function handleFsGlob(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const pattern = typeof params.pattern === 'string' ? params.pattern : ''
  const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const ignore  = Array.isArray(params.ignore)
    ? (params.ignore as unknown[]).map(String)
    : ['node_modules', '.git', 'dist', 'out']
  const MAX_RESULTS = 200

  if (!pattern) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: pattern' } }
  }

  // Extract filename pattern (last segment of glob pattern)
  const filePattern = pattern.split('/').pop() ?? pattern

  const ignoreArgs = ignore.flatMap((p: string) => [
    '-not', '-path', `*/${p}/*`,
    '-not', '-name', p,
  ])

  const results = await new Promise<string[]>((resolve, reject) => {
    const child = spawn('find', [
      cwd,
      '-maxdepth', '10',
      ...ignoreArgs,
      '-name', filePattern,
      '-type', 'f',
    ], { shell: false })

    const lines: string[] = []

    child.stdout?.on('data', (chunk: Buffer) => {
      const newLines = chunk.toString().split('\n').filter((l: string) => l.trim())
      lines.push(...newLines)
      if (lines.length > MAX_RESULTS) child.kill('SIGTERM')
    })

    child.on('close', () => resolve(lines.slice(0, MAX_RESULTS)))
    child.on('error', reject)
  })

  const relativePaths = results.map((p: string) =>
    p.startsWith(cwd + '/') ? p.slice(cwd.length + 1) : p
  )

  return {
    jsonrpc: '2.0', id,
    result: { paths: relativePaths, cwd, total: relativePaths.length },
  }
}

// ─── fs.writeFile ─────────────────────────────────────────────────────────────

/**
 * Write file on dev server.
 * SECURITY CRITICAL: path must be within config.workDir (projectRoot).
 * Max write: 10MB. Creates parent directories automatically.
 */
export async function handleFsWriteFile(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath  = typeof params.path     === 'string' ? params.path    : ''
  const content  = typeof params.content  === 'string' ? params.content : ''
  const encoding = typeof params.encoding === 'string' ? params.encoding as BufferEncoding : 'utf-8'

  if (!rawPath) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath      = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)
  const resolvedPath = resolvePath(absPath)
  const resolvedWork = resolvePath(config.workDir)
  const span         = fsTracer.start({ method: 'fs.writeFile', path: rawPath })

  // SecureFs: must be within workDir
  if (!resolvedPath.startsWith(resolvedWork + '/') && resolvedPath !== resolvedWork) {
    span.fail('path outside project root', { path: rawPath })
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: `Path outside project root: ${rawPath}` },
    }
  }

  // Size limit: 10MB
  const MAX_WRITE = 10 * 1024 * 1024
  const byteLen = Buffer.byteLength(content, encoding)
  if (byteLen > MAX_WRITE) {
    span.fail('content too large', { path: rawPath, bytes: byteLen })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Content too large: max 10MB' } }
  }

  try {
    await mkdir(dirname(resolvedPath), { recursive: true })
    await writeFile(resolvedPath, content, { encoding })
    span.ok({ path: resolvedPath, bytes: byteLen })
    return {
      jsonrpc: '2.0', id,
      result: { ok: true, path: resolvedPath, bytes: byteLen },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { path: resolvedPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── shell.eval ───────────────────────────────────────────────────────────────
// Executes a short shell command and returns stdout + stderr.
// Used internally by devServer.browseDir to resolve '~' → real home directory.
// SECURITY: only reachable via the authenticated agent relay, never from browser.

export async function handleShellEval(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig
): Promise<object> {
  const command   = typeof params.command === 'string' ? params.command : ''
  const timeoutMs = typeof params.timeout === 'number' ? Math.min(params.timeout, 10_000) : 5_000
  const span      = fsTracer.start({ method: 'shell.eval', cmd: command.slice(0, 80) })

  if (!command) {
    span.fail('missing param: command', { method: 'shell.eval' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: command' } }
  }

  return new Promise((resolve) => {
    let stdout = ''
    let stderr = ''
    const child = spawn('sh', ['-c', command], { env: process.env })
    const timer = setTimeout(() => {
      child.kill()
      span.fail('timed out', { cmd: command.slice(0, 80), timeoutMs })
      resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: 'shell.eval timed out' } })
    }, timeoutMs)

    child.stdout.on('data', (d: Buffer) => { stdout += d.toString() })
    child.stderr.on('data', (d: Buffer) => { stderr += d.toString() })
    child.on('close', (code) => {
      clearTimeout(timer)
      span.ok({ exitCode: code ?? 0, stdoutLen: stdout.length })
      resolve({ jsonrpc: '2.0', id, result: { stdout, stderr, exitCode: code ?? 0 } })
    })
    child.on('error', (err) => {
      clearTimeout(timer)
      span.fail(err, { cmd: command.slice(0, 80) })
      resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: err.message } })
    })
  })
}

// ─── fs.mkdir ─────────────────────────────────────────────────────────────────
// Creates a directory (and parents) on the agent filesystem.

export async function handleFsMkdir(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const span    = fsTracer.start({ method: 'fs.mkdir', path: rawPath || '(empty)' })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.mkdir' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    await mkdir(absPath, { recursive: true })
    span.ok({ path: absPath })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { path: absPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.rmdir ─────────────────────────────────────────────────────────────────
// Removes a directory from the agent filesystem.
// Refuses to remove non-empty directories (safe default).

export async function handleFsRmdir(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath   = typeof params.path === 'string' ? params.path : ''
  const recursive = params.recursive === true
  const span      = fsTracer.start({ method: 'fs.rmdir', path: rawPath || '(empty)', recursive })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.rmdir' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  // Safety: refuse to remove workDir itself or any parent
  if (config.workDir.startsWith(absPath) || absPath === '/') {
    span.fail('refusing to remove protected path', { path: absPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Refusing to remove protected path' } }
  }

  try {
    if (recursive) {
      await rm(absPath, { recursive: true, force: true })
    } else {
      await fsRmdir(absPath)
    }
    span.ok({ path: absPath, recursive })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { path: absPath, recursive })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
