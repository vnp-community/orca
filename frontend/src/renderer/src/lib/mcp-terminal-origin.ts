import type { McpOrigin } from '../../../shared/mcp-types'
import { callRuntimeRpc, type RuntimeClientTarget } from '@/runtime/runtime-rpc-client'
import { parseRemoteRuntimePtyId } from '@/runtime/runtime-terminal-stream'

/** Key = terminal handle (== backend ptyId). */
export type OriginByHandle = Record<string, McpOrigin>

const MAX_CLIENT_NAME = 40
// Control chars and bidi overrides/isolates: clientName is self-declared by the MCP client.
// eslint-disable-next-line no-control-regex
const UNSAFE_CHARS = /[\u0000-\u001f\u007f-\u009f‎‏‪-‮⁦-⁩]/g

export function isMcpOrigin(value: unknown): value is McpOrigin {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  const o = value as Record<string, unknown>
  return (
    o.type === 'mcp' &&
    typeof o.clientName === 'string' &&
    typeof o.mcpSessionId === 'string' &&
    typeof o.userId === 'string'
  )
}

/** Plain-text, bounded client name safe to render in a badge. */
export function displayMcpClientName(name: string): string {
  const clean = name.replace(UNSAFE_CHARS, '').trim()
  if (!clean) {
    return 'MCP client'
  }
  return clean.length > MAX_CLIENT_NAME ? `${clean.slice(0, MAX_CLIENT_NAME - 1)}…` : clean
}

function entriesOf(raw: unknown): unknown[] {
  if (Array.isArray(raw)) {
    return raw
  }
  if (typeof raw === 'object' && raw !== null) {
    const terminals = (raw as { terminals?: unknown }).terminals
    return Array.isArray(terminals) ? terminals : []
  }
  return []
}

function handleOf(entry: Record<string, unknown>): string | null {
  const handle = typeof entry.handle === 'string' && entry.handle ? entry.handle : null
  const ptyId = typeof entry.ptyId === 'string' && entry.ptyId ? entry.ptyId : null
  return handle ?? ptyId
}

/**
 * Accepts both terminal.list shapes: the bare array backend-go returns and the
 * `{ terminals }` runtime shape. Entries without a valid origin are skipped.
 */
export function normalizeTerminalListOrigins(raw: unknown): OriginByHandle {
  const out: OriginByHandle = {}
  for (const entry of entriesOf(raw)) {
    if (typeof entry !== 'object' || entry === null) {
      continue
    }
    const record = entry as Record<string, unknown>
    const handle = handleOf(record)
    if (handle && isMcpOrigin(record.origin)) {
      out[handle] = record.origin
    }
  }
  return out
}

/** `remote:<env>@@<handle>` -> `<handle>`; other ids are already handles. */
export function ptyIdToOriginKey(ptyId: string): string {
  return parseRemoteRuntimePtyId(ptyId)?.handle ?? ptyId
}

export async function fetchMcpTerminalOrigins(
  target: RuntimeClientTarget
): Promise<OriginByHandle> {
  return normalizeTerminalListOrigins(
    await callRuntimeRpc<unknown>(target, 'terminal.list', {}, { timeoutMs: 8000 })
  )
}

/** Existing terminal.stop / terminal.close channels; same permission as the UI. */
export async function stopMcpOriginTerminal(
  target: RuntimeClientTarget,
  handle: string,
  force = false
): Promise<void> {
  await callRuntimeRpc<unknown>(target, force ? 'terminal.close' : 'terminal.stop', {
    terminal: handle
  })
}
