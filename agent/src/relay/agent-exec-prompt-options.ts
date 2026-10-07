// src/relay/agent-exec-prompt-options.ts
// Parses and validates the new execPrompt options introduced in CR-REQ-033:
// accessMode, workspaceKind, reportChanges, resultBlock, maxOutputBytes.
// Intentionally does NOT import from agent-rpc-dispatch.ts to avoid a
// circular dependency (dispatcher dynamically imports this module).

import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ── Constants ──────────────────────────────────────────────────────────────────

export const DEFAULT_MAX_OUTPUT_BYTES = 4 * 1024 * 1024   // 4 MiB
export const MAX_OUTPUT_BYTES_CEILING = 12 * 1024 * 1024  // 12 MiB
export const MIN_OUTPUT_BYTES = 64 * 1024                  // 64 KiB
export const RESULT_NONCE_PATTERN = /^[A-Za-z0-9]{16,64}$/

// ── Types ──────────────────────────────────────────────────────────────────────

export type ExecPromptErrorCode =
  | 'INVALID_ACCESS_MODE'
  | 'INVALID_WORKSPACE_KIND'
  | 'INVALID_REPORT_CHANGES'
  | 'INVALID_RESULT_BLOCK'
  | 'INVALID_RESULT_BLOCK_NONCE'
  | 'INVALID_MAX_OUTPUT_BYTES'

export type ExecPromptOptions = {
  accessMode: 'write' | 'readonly'
  workspaceKind: 'worktree' | 'repo_root' | 'scratch'
  /** Tracks whether each option was sent explicitly by the caller. */
  explicit: { accessMode: boolean; workspaceKind: boolean }
  reportChanges: boolean
  resultBlockNonce: string | null
  maxOutputBytes: number
}

type ParseError = { code: ExecPromptErrorCode; message: string }
type ParseResult<T> =
  | { ok: true; value: T }
  | { ok: false; error: ParseError }

// ── Parser ─────────────────────────────────────────────────────────────────────

export function parseExecPromptOptions(
  params: Record<string, unknown>
): ParseResult<ExecPromptOptions> {
  // accessMode
  const hasAccessMode = 'accessMode' in params
  const rawAccessMode = params['accessMode']
  let accessMode: 'write' | 'readonly' = 'write'
  if (hasAccessMode) {
    if (rawAccessMode !== 'write' && rawAccessMode !== 'readonly') {
      return {
        ok: false,
        error: {
          code: 'INVALID_ACCESS_MODE',
          message: `accessMode must be "write" or "readonly", got ${JSON.stringify(rawAccessMode)}`
        }
      }
    }
    accessMode = rawAccessMode
  }

  // workspaceKind
  const hasWorkspaceKind = 'workspaceKind' in params
  const rawWorkspaceKind = params['workspaceKind']
  let workspaceKind: 'worktree' | 'repo_root' | 'scratch' = 'worktree'
  if (hasWorkspaceKind) {
    if (
      rawWorkspaceKind !== 'worktree' &&
      rawWorkspaceKind !== 'repo_root' &&
      rawWorkspaceKind !== 'scratch'
    ) {
      return {
        ok: false,
        error: {
          code: 'INVALID_WORKSPACE_KIND',
          message: `workspaceKind must be "worktree", "repo_root", or "scratch", got ${JSON.stringify(rawWorkspaceKind)}`
        }
      }
    }
    workspaceKind = rawWorkspaceKind
  }

  // reportChanges
  const hasReportChanges = 'reportChanges' in params
  const rawReportChanges = params['reportChanges']
  let reportChanges = false
  if (hasReportChanges) {
    if (typeof rawReportChanges !== 'boolean') {
      return {
        ok: false,
        error: {
          code: 'INVALID_REPORT_CHANGES',
          message: `reportChanges must be a boolean, got ${typeof rawReportChanges}`
        }
      }
    }
    reportChanges = rawReportChanges
  }

  // resultBlock
  const hasResultBlock = 'resultBlock' in params
  const rawResultBlock = params['resultBlock']
  let resultBlockNonce: string | null = null
  if (hasResultBlock) {
    if (
      typeof rawResultBlock !== 'object' ||
      rawResultBlock === null ||
      Array.isArray(rawResultBlock)
    ) {
      return {
        ok: false,
        error: {
          code: 'INVALID_RESULT_BLOCK',
          message: `resultBlock must be an object (not null or array), got ${Array.isArray(rawResultBlock) ? 'array' : typeof rawResultBlock}`
        }
      }
    }
    const nonce = (rawResultBlock as Record<string, unknown>)['nonce']
    if (typeof nonce !== 'string' || !RESULT_NONCE_PATTERN.test(nonce)) {
      return {
        ok: false,
        error: {
          code: 'INVALID_RESULT_BLOCK_NONCE',
          message: `resultBlock.nonce must match /^[A-Za-z0-9]{16,64}$/, got ${JSON.stringify(nonce)}`
        }
      }
    }
    resultBlockNonce = nonce
  }

  // maxOutputBytes
  const hasMaxOutputBytes = 'maxOutputBytes' in params
  const rawMaxOutputBytes = params['maxOutputBytes']
  let maxOutputBytes = DEFAULT_MAX_OUTPUT_BYTES
  if (hasMaxOutputBytes) {
    if (
      typeof rawMaxOutputBytes !== 'number' ||
      !isFinite(rawMaxOutputBytes) ||
      !Number.isInteger(rawMaxOutputBytes)
    ) {
      return {
        ok: false,
        error: {
          code: 'INVALID_MAX_OUTPUT_BYTES',
          message: `maxOutputBytes must be a finite integer, got ${JSON.stringify(rawMaxOutputBytes)}`
        }
      }
    }
    if (rawMaxOutputBytes < MIN_OUTPUT_BYTES) {
      return {
        ok: false,
        error: {
          code: 'INVALID_MAX_OUTPUT_BYTES',
          message: `maxOutputBytes must be >= ${MIN_OUTPUT_BYTES} (64 KiB), got ${rawMaxOutputBytes}`
        }
      }
    }
    // Clamp at ceiling rather than reject — caller may use a larger value
    maxOutputBytes = Math.min(rawMaxOutputBytes, MAX_OUTPUT_BYTES_CEILING)
  }

  return {
    ok: true,
    value: {
      accessMode,
      workspaceKind,
      explicit: { accessMode: hasAccessMode, workspaceKind: hasWorkspaceKind },
      reportChanges,
      resultBlockNonce,
      maxOutputBytes
    }
  }
}

// ── Error response builder ─────────────────────────────────────────────────────

export function toExecPromptErrorResponse(
  method: 'agent.execPrompt' | 'agent.execPromptStream',
  id: string | number | null,
  error: ParseError
): object {
  return {
    jsonrpc: '2.0',
    id,
    error: {
      code: AgentErrorCode.InvalidParams,
      message: `${method}: ${error.message} (${error.code})`,
      data: { reason: error.code }
    }
  }
}
