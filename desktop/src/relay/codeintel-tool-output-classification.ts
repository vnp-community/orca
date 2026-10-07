import { CodeIntelError } from './codeintel-errors'
import { tailForStderr } from './codeintel-secret-redaction'

export type GitNexusOutput = { kind: 'json'; value: any } | { notFound: true }

export function classifyGitNexusStdout(text: string, log?: { error: (msg: string) => void }): GitNexusOutput {
  if (!text.trim()) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Empty output from GitNexus', { reason: 'truncated_stdout' })
  }
  let parsed: any
  try {
    parsed = JSON.parse(text)
  } catch (err) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Invalid JSON from GitNexus', { reason: 'truncated_stdout', stdoutTail: tailForStderr(text) })
  }

  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed) && typeof parsed.error === 'string') {
    const errStr = parsed.error.toLowerCase()
    if (errStr.includes('not found') || errStr.includes('does not exist') || errStr.includes('ambiguous')) {
      return { notFound: true }
    }
    if (errStr.includes('write operations') || errStr.includes('create')) {
      if (log) log.error(`GitNexus blocked write operation: ${parsed.error}`)
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', parsed.error, { reason: 'write_blocked' })
    }
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', parsed.error)
  }

  if (parsed && typeof parsed === 'object') {
    return { kind: 'json', value: parsed }
  }

  throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Unknown shape from GitNexus', { reason: 'unknown_shape' })
}

export type CodeGraphOutput = { kind: 'json'; value: any }

export function classifyCodeGraphStdout(text: string): CodeGraphOutput {
  const clean = text.replace(/\u001b\[[0-9;]*m/g, '').trim()
  if (!clean.startsWith('[') && !clean.startsWith('{')) {
    const lower = clean.toLowerCase()
    if (lower.includes('not found')) {
      throw new CodeIntelError('CODEINTEL_SYMBOL_NOT_FOUND', clean)
    }
    if (lower.includes('not initialized')) {
      throw new CodeIntelError('CODEINTEL_INDEX_MISSING', clean)
    }
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', clean, { reason: 'unknown_shape' })
  }

  try {
    const parsed = JSON.parse(clean)
    return { kind: 'json', value: parsed }
  } catch (err) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Invalid JSON from CodeGraph', { reason: 'truncated_stdout', stdoutTail: tailForStderr(text) })
  }
}
