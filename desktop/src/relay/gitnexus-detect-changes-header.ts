export interface GitNexusRiskHint {
  source: 'gitnexus detect-changes'
  level: string
  files: number
  symbols: number
  processes: number
}

export interface CrossCheckComparison {
  riskHint: GitNexusRiskHint | null
  warnings: string[]
}

const CHANGES_REGEX = /^Changes:\s+(\d+)\s+files?,\s+(\d+)\s+symbols?$/i
const PROCESSES_REGEX = /^Affected processes:\s+(\d+)$/i
const RISK_REGEX = /^Risk level:\s+(\w+)$/i

export function parseGitNexusDetectChangesHeader(output: string): GitNexusRiskHint | null {
  if (!output) return null

  const lines = output
    .split('\n')
    .map(l => l.trim())
    .filter(l => l.length > 0)

  if (lines.length < 3) return null

  const m1 = lines[0].match(CHANGES_REGEX)
  const m2 = lines[1].match(PROCESSES_REGEX)
  const m3 = lines[2].match(RISK_REGEX)

  if (!m1 || !m2 || !m3) {
    return null
  }

  const files = parseInt(m1[1], 10)
  const symbols = parseInt(m1[2], 10)
  const processes = parseInt(m2[1], 10)
  const level = m3[1].toUpperCase()

  return {
    source: 'gitnexus detect-changes',
    level,
    files,
    symbols,
    processes
  }
}

export function evaluateCrossCheck(
  hint: GitNexusRiskHint | null,
  agentMetrics: { files: number; symbols: number }
): CrossCheckComparison {
  const warnings: string[] = []
  if (!hint) {
    return { riskHint: null, warnings }
  }

  // Check mismatch > 20%
  const isMismatch = (agentVal: number, cliVal: number): boolean => {
    const maxVal = Math.max(agentVal, cliVal)
    if (maxVal === 0) return false
    const diff = Math.abs(agentVal - cliVal)
    return diff / maxVal > 0.2
  }

  if (
    isMismatch(agentMetrics.files, hint.files) ||
    isMismatch(agentMetrics.symbols, hint.symbols)
  ) {
    warnings.push('crosscheck_mismatch')
  }

  return {
    riskHint: hint,
    warnings
  }
}
