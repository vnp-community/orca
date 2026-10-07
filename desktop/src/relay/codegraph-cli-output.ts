import { classifyCodeGraphStdout } from './codeintel-tool-output-classification'
import { CodeIntelError } from './codeintel-errors'

export function parseCodeGraphStatus(stdout: string) {
  const result = classifyCodeGraphStdout(stdout)
  if (typeof result.value !== 'object' || result.value === null || Array.isArray(result.value)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Expected object from CodeGraph status', { reason: 'unknown_shape' })
  }
  return result.value
}

export function parseCodeGraphQuery(stdout: string) {
  const result = classifyCodeGraphStdout(stdout)
  if (!Array.isArray(result.value)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Expected array from CodeGraph query', { reason: 'unknown_shape' })
  }
  return result.value
}

export function parseCodeGraphCallers(stdout: string) {
  const result = classifyCodeGraphStdout(stdout)
  if (!Array.isArray(result.value)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Expected array from CodeGraph callers', { reason: 'unknown_shape' })
  }
  return result.value
}

export function parseCodeGraphCallees(stdout: string) {
  const result = classifyCodeGraphStdout(stdout)
  if (!Array.isArray(result.value)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Expected array from CodeGraph callees', { reason: 'unknown_shape' })
  }
  return result.value
}

export function parseCodeGraphFiles(stdout: string, limit?: number) {
  const result = classifyCodeGraphStdout(stdout)
  if (!Array.isArray(result.value)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Expected array from CodeGraph files', { reason: 'unknown_shape' })
  }
  let files = result.value
  let truncated = false
  if (limit !== undefined && limit > 0 && files.length > limit) {
    files = files.slice(0, limit)
    truncated = true
  }
  return { files, truncated }
}

export function parseCodeGraphAffected(stdout: string) {
  const result = classifyCodeGraphStdout(stdout)
  if (typeof result.value !== 'object' || result.value === null || Array.isArray(result.value)) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Expected object from CodeGraph affected', { reason: 'unknown_shape' })
  }
  return {
    changedFiles: Array.isArray(result.value.changedFiles) ? result.value.changedFiles : [],
    affectedTests: Array.isArray(result.value.affectedTests) ? result.value.affectedTests : []
  }
}
