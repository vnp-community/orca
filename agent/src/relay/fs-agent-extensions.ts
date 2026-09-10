// src/relay/fs-agent-extensions.ts
// FS RPC handlers for Orca Dev Agent v5.0.
// Wraps existing fs-handler-*.ts modules for agent RPC methods:
//   fs.readDir, fs.readFile, fs.stat, fs.glob
// Sibling RPC-handler groups split out of this file (max-lines ratchet):
// fs.grep/preflight.check -> fs-agent-search-extensions.ts,
// fs.writeFile/mkdir/rmdir -> fs-agent-write-extensions.ts,
// shell.eval/exec/execStream -> shell-agent-extensions.ts,
// fs.watch/unwatch -> fs-agent-watch-extensions.ts.

import { readdir, stat } from 'node:fs/promises'
import { join, isAbsolute } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { readRelayFileContent } from './fs-handler-file-read'
import { createTracer } from '../shared/trace'

const fsTracer = createTracer('agent:fs')

// ─── fs.readDir ───────────────────────────────────────────────────────────────

type FileTreeNode = {
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
  const depth = typeof params.depth === 'number' ? Math.min(params.depth, 5) : 1
  const span = fsTracer.start({ method: 'fs.readDir', path: rawPath || '(empty)', depth })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.readDir' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    const st = await stat(absPath)
    if (!st.isDirectory()) {
      span.fail('not a directory', { path: absPath })
      return {
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.InvalidParams, message: `Not a directory: ${absPath}` }
      }
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
        size: entry.isFile() ? (await stat(fullPath)).size : undefined
      }
      if (entry.isDirectory() && currentDepth < maxDepth) {
        node.children = await readDirRecursive(fullPath, maxDepth, currentDepth + 1)
      }
      return node
    })
  )
  // Sort: directories first, then files; alphabetically within each group
  return nodes.sort((a, b) => {
    if (a.type !== b.type) {
      return a.type === 'directory' ? -1 : 1
    }
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
  const span = fsTracer.start({ method: 'fs.readFile', path: rawPath || '(empty)' })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.readFile' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    // Reuse existing readRelayFileContent — handles size limits and binary detection
    const result = await readRelayFileContent(absPath)
    span.ok({ path: absPath, bytes: result.content.length, binary: result.isBinary })
    return {
      jsonrpc: '2.0',
      id,
      result: {
        content: result.content,
        encoding: result.isBinary ? 'base64' : 'utf-8',
        isBinary: result.isBinary,
        path: absPath
      }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes('File too large') || msg.includes('MAX_TEXT_FILE_SIZE')) {
      span.fail('file too large', { path: absPath })
      return {
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.InvalidParams, message: 'FILE_TOO_LARGE' }
      }
    }
    if (msg.includes('ENOENT') || msg.includes('not found')) {
      span.fail('file not found', { path: absPath })
      return {
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.PathNotFound, message: `File not found: ${absPath}` }
      }
    }
    span.fail(err, { path: absPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.stat ──────────────────────────────────────────────────────────────────

export async function handleFsStat(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  if (!rawPath) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  try {
    const st = await stat(absPath)
    return {
      jsonrpc: '2.0',
      id,
      result: {
        path: absPath,
        size: st.size,
        mtime: st.mtime.toISOString(),
        isDir: st.isDirectory(),
        isFile: st.isFile(),
        isLink: st.isSymbolicLink(),
        mode: st.mode.toString(8)
      }
    }
  } catch (err: unknown) {
    const nodeErr = err as NodeJS.ErrnoException
    if (nodeErr.code === 'ENOENT') {
      // Why PathNotFound, not the generic ServerError: callers (e.g. desktop's
      // fs:pathExists handler) need a stable, non-message-based way to tell
      // "doesn't exist" apart from a real server failure — ServerError made
      // every existence check over a Dev Server connection indistinguishable
      // from an actual error (found live: "New Markdown" looping through all
      // 100 untitled-N.md candidates, each one misread as "already exists").
      return {
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.PathNotFound, message: `Not found: ${absPath}` }
      }
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
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const pattern = typeof params.pattern === 'string' ? params.pattern : ''
  const cwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const ignore = Array.isArray(params.ignore)
    ? (params.ignore as unknown[]).map(String)
    : ['node_modules', '.git', 'dist', 'out']
  const MAX_RESULTS = 200

  if (!pattern) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: pattern' }
    }
  }

  // Extract filename pattern (last segment of glob pattern)
  const filePattern = pattern.split('/').pop() ?? pattern

  const ignoreArgs = ignore.flatMap((p: string) => [
    '-not',
    '-path',
    `*/${p}/*`,
    '-not',
    '-name',
    p
  ])

  const results = await new Promise<string[]>((resolve, reject) => {
    const child = spawn(
      'find',
      [cwd, '-maxdepth', '10', ...ignoreArgs, '-name', filePattern, '-type', 'f'],
      { shell: false }
    )

    const lines: string[] = []

    child.stdout?.on('data', (chunk: Buffer) => {
      const newLines = chunk
        .toString()
        .split('\n')
        .filter((l: string) => l.trim())
      lines.push(...newLines)
      if (lines.length > MAX_RESULTS) {
        child.kill('SIGTERM')
      }
    })

    child.on('close', () => resolve(lines.slice(0, MAX_RESULTS)))
    child.on('error', reject)
  })

  const relativePaths = results.map((p: string) =>
    p.startsWith(`${cwd}/`) ? p.slice(cwd.length + 1) : p
  )

  return {
    jsonrpc: '2.0',
    id,
    result: { paths: relativePaths, cwd, total: relativePaths.length }
  }
}
