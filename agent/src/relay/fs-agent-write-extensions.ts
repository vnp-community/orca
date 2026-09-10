// src/relay/fs-agent-write-extensions.ts
// Mutating FS RPC handlers for Orca Dev Agent v5.0: fs.writeFile, fs.mkdir, fs.rmdir.
// Split out of fs-agent-extensions.ts to stay under the repo's 300-line max-lines ratchet.

import { writeFile, mkdir, rmdir as fsRmdir, rm } from 'node:fs/promises'
import { join, isAbsolute, resolve as resolvePath, dirname } from 'node:path'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const fsTracer = createTracer('agent:fs')

// ─── fs.writeFile ─────────────────────────────────────────────────────────────

/**
 * Write file on dev server.
 * SECURITY CRITICAL: path must be within config.workDir (projectRoot).
 * Max write: 10MB. Creates parent directories automatically.
 */
export async function handleFsWriteFile(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const content = typeof params.content === 'string' ? params.content : ''
  const encoding =
    typeof params.encoding === 'string' ? (params.encoding as BufferEncoding) : 'utf-8'

  if (!rawPath) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)
  const resolvedPath = resolvePath(absPath)
  const resolvedWork = resolvePath(config.workDir)
  const span = fsTracer.start({ method: 'fs.writeFile', path: rawPath })

  // SecureFs: must be within workDir
  if (!resolvedPath.startsWith(`${resolvedWork}/`) && resolvedPath !== resolvedWork) {
    span.fail('path outside project root', { path: rawPath })
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `Path outside project root: ${rawPath}`
      }
    }
  }

  // Size limit: 10MB
  const MAX_WRITE = 10 * 1024 * 1024
  const byteLen = Buffer.byteLength(content, encoding)
  if (byteLen > MAX_WRITE) {
    span.fail('content too large', { path: rawPath, bytes: byteLen })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Content too large: max 10MB' }
    }
  }

  try {
    await mkdir(dirname(resolvedPath), { recursive: true })
    await writeFile(resolvedPath, content, { encoding })
    span.ok({ path: resolvedPath, bytes: byteLen })
    return {
      jsonrpc: '2.0',
      id,
      result: { ok: true, path: resolvedPath, bytes: byteLen }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { path: resolvedPath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── fs.mkdir ─────────────────────────────────────────────────────────────────
// Creates a directory (and parents) on the agent filesystem.

export async function handleFsMkdir(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const span = fsTracer.start({ method: 'fs.mkdir', path: rawPath || '(empty)' })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.mkdir' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
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
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const recursive = params.recursive === true
  const span = fsTracer.start({ method: 'fs.rmdir', path: rawPath || '(empty)', recursive })

  if (!rawPath) {
    span.fail('missing param: path', { method: 'fs.rmdir' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }

  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  // Safety: refuse to remove workDir itself or any parent
  if (config.workDir.startsWith(absPath) || absPath === '/') {
    span.fail('refusing to remove protected path', { path: absPath })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Refusing to remove protected path' }
    }
  }

  try {
    await (recursive ? rm(absPath, { recursive: true, force: true }) : fsRmdir(absPath))
    span.ok({ path: absPath, recursive })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { path: absPath, recursive })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
