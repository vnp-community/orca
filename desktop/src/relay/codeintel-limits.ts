export interface CodeIntelLimits {
  toolTimeoutMs: number
  methodTimeoutMs: number
  detectTimeoutMs: number
  toolMaxOutputBytes: number
  resultMaxBytes: number
  maxConcurrentTools: number
  queueMax: number
  queueWaitMs: number
  killGraceMs: number
}

export const CODEINTEL_DEFAULT_LIMITS: CodeIntelLimits = {
  toolTimeoutMs: 20000,
  methodTimeoutMs: 25000,
  detectTimeoutMs: 55000,
  toolMaxOutputBytes: 16 * 1024 * 1024, // 16 MiB
  resultMaxBytes: 8 * 1024 * 1024, // 8 MiB
  maxConcurrentTools: 3,
  queueMax: 16,
  queueWaitMs: 10000,
  killGraceMs: 5000,
}

export function readCodeIntelLimits(env: NodeJS.ProcessEnv, log: { warn: (msg: string) => void }): CodeIntelLimits {
  const limits = { ...CODEINTEL_DEFAULT_LIMITS }

  const parseNum = (key: string, min: number, max: number): number | undefined => {
    if (env[key] !== undefined) {
      const val = Number(env[key])
      if (Number.isInteger(val) && val >= min && val <= max) {
        return val
      }
      log.warn(`Invalid value for ${key}: ${env[key]}. Using default.`)
    }
    return undefined
  }

  const toolTimeoutMs = parseNum('ORCA_CODEINTEL_TOOL_TIMEOUT_MS', 1, 300000)
  if (toolTimeoutMs) limits.toolTimeoutMs = toolTimeoutMs

  const methodTimeoutMs = parseNum('ORCA_CODEINTEL_METHOD_TIMEOUT_MS', 1, 300000)
  if (methodTimeoutMs) {
    if (methodTimeoutMs <= limits.toolTimeoutMs) {
      log.warn(`ORCA_CODEINTEL_METHOD_TIMEOUT_MS (${methodTimeoutMs}) must be > toolTimeoutMs (${limits.toolTimeoutMs}). Using default.`)
    } else {
      limits.methodTimeoutMs = methodTimeoutMs
    }
  }

  const detectTimeoutMs = parseNum('ORCA_CODEINTEL_DETECT_TIMEOUT_MS', 1, 300000)
  if (detectTimeoutMs) limits.detectTimeoutMs = detectTimeoutMs

  const toolMaxOutputBytes = parseNum('ORCA_CODEINTEL_TOOL_MAX_OUTPUT_BYTES', 1, 100 * 1024 * 1024)
  if (toolMaxOutputBytes) limits.toolMaxOutputBytes = toolMaxOutputBytes

  const resultMaxBytes = parseNum('ORCA_CODEINTEL_RESULT_MAX_BYTES', 1, 100 * 1024 * 1024)
  if (resultMaxBytes) limits.resultMaxBytes = resultMaxBytes

  const maxConcurrentTools = parseNum('ORCA_CODEINTEL_MAX_CONCURRENT_TOOLS', 1, 100)
  if (maxConcurrentTools) limits.maxConcurrentTools = maxConcurrentTools

  const queueMax = parseNum('ORCA_CODEINTEL_QUEUE_MAX', 1, 1000)
  if (queueMax) limits.queueMax = queueMax

  const queueWaitMs = parseNum('ORCA_CODEINTEL_QUEUE_WAIT_MS', 1, 60000)
  if (queueWaitMs) limits.queueWaitMs = queueWaitMs

  return Object.freeze(limits)
}
