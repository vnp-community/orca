export interface RuntimeSwitchWarning {
  var: string
  code: string
}

export interface RuntimeSwitches {
  readonly codeintelDisabled: boolean
  readonly reindexDisabled: boolean
  readonly qualityDisabled: boolean
  readonly warnings: ReadonlyArray<RuntimeSwitchWarning>
}

const TRUTHY_VALUES = new Set(['1', 'true', 'yes', 'on'])
const FALSY_VALUES = new Set(['0', 'false', 'no', 'off'])

export function readRuntimeSwitches(env: NodeJS.ProcessEnv = process.env): RuntimeSwitches {
  const warnings: RuntimeSwitchWarning[] = []

  // 1. ORCA_CODEINTEL_DISABLED
  let codeintelDisabled = false
  const rawDisabled = env['ORCA_CODEINTEL_DISABLED']
  if (rawDisabled !== undefined) {
    const val = rawDisabled.trim().toLowerCase()
    if (val === '') {
      codeintelDisabled = false
    } else if (TRUTHY_VALUES.has(val)) {
      codeintelDisabled = true
    } else if (FALSY_VALUES.has(val)) {
      codeintelDisabled = false
    } else {
      // Fail closed for unknown value
      codeintelDisabled = true
      warnings.push({ var: 'ORCA_CODEINTEL_DISABLED', code: 'invalid_value' })
    }
  }

  // 2. ORCA_CODEINTEL_REINDEX
  let reindexDisabled = codeintelDisabled
  const rawReindex = env['ORCA_CODEINTEL_REINDEX']
  if (rawReindex !== undefined) {
    const val = rawReindex.trim().toLowerCase()
    if (val === 'off') {
      reindexDisabled = true
    } else if (val === 'on' || val === '' || val === '1' || val === '0' || val === 'true' || val === 'false') {
      if (!codeintelDisabled) reindexDisabled = false
    } else {
      warnings.push({ var: 'ORCA_CODEINTEL_REINDEX', code: 'invalid_value' })
    }
  }

  // 3. ORCA_QUALITY_RUN
  let qualityDisabled = codeintelDisabled
  const rawQuality = env['ORCA_QUALITY_RUN']
  if (rawQuality !== undefined) {
    const val = rawQuality.trim().toLowerCase()
    if (val === 'off') {
      qualityDisabled = true
    } else if (val === 'on' || val === '' || val === '1' || val === '0' || val === 'true' || val === 'false') {
      if (!codeintelDisabled) qualityDisabled = false
    } else {
      warnings.push({ var: 'ORCA_QUALITY_RUN', code: 'invalid_value' })
    }
  }

  // If codeintelDisabled is true, cascade disable to reindex and quality
  if (codeintelDisabled) {
    reindexDisabled = true
    qualityDisabled = true
  }

  return Object.freeze({
    codeintelDisabled,
    reindexDisabled,
    qualityDisabled,
    warnings: Object.freeze(warnings)
  })
}

let hasLoggedDisabledWarning = false

export function logCodeIntelDisabledOnce(log: { warn: (msg: string) => void }, switches?: RuntimeSwitches) {
  const sw = switches ?? readRuntimeSwitches()
  if (sw.codeintelDisabled && !hasLoggedDisabledWarning) {
    hasLoggedDisabledWarning = true
    log.warn('codeintel disabled by ORCA_CODEINTEL_DISABLED')
  }
}

export function resetDisabledLogState() {
  hasLoggedDisabledWarning = false
}
