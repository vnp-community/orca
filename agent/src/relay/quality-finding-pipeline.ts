import { RawQualityFinding } from './quality-parser-types'
import {
  normalizeAnchor,
  normalizeMessage,
  assignOccurrences,
  computeFingerprint,
  FingerprintItem
} from './quality-finding-fingerprint'
import {
  MAX_FINDINGS_PER_STEP,
  MESSAGE_MAX_BYTES,
  FIX_HINT_MAX_BYTES
} from './quality-finding-limits'

export type QualityFinding = Omit<RawQualityFinding, 'anchorOverride' | 'testAnchor' | 'printedPath' | 'evidence'> & {
  fingerprint: string
  fpVersion: number
  tool: string
  toolVersion: string
  stepId: string
  inScope: boolean
  fixHint?: string
}

export type PipelineContext = {
  stepId: string
  tool: string
  toolVersion: string
  repoRoot: string
  cwd: string
  home?: string
  tmp?: string
  scopeFiles: string[] | null // null means all files are in scope
  readSourceLine: (file: string, line: number) => Promise<string | null>
  remainingRunBudget: number
}

function truncateStringUTF8(str: string, maxBytes: number): string {
  if (!str) return str
  let buf = Buffer.from(str, 'utf8')
  if (buf.length <= maxBytes) return str
  // Truncate and ensure valid utf8
  buf = buf.subarray(0, maxBytes - 3)
  // Ensure we don't cut inside a UTF-8 multi-byte sequence
  // If the last byte is a continuation byte (10xxxxxx), remove it until we hit the start byte
  while (buf.length > 0 && (buf[buf.length - 1] & 0xc0) === 0x80) {
    buf = buf.subarray(0, buf.length - 1)
  }
  // Also remove the start byte of an incomplete sequence
  if (buf.length > 0) {
    const lastByte = buf[buf.length - 1]
    if ((lastByte & 0xc0) === 0xc0) {
      buf = buf.subarray(0, buf.length - 1)
    }
  }
  return buf.toString('utf8') + '...'
}

function isInScope(file: string, scopeFiles: string[] | null): boolean {
  if (!file) return false
  if (scopeFiles === null) return true
  const lower = file.toLowerCase()
  return scopeFiles.some(s => s.toLowerCase() === lower)
}

const SEVERITY_WEIGHT = { error: 0, warning: 1, info: 2 }

export async function runFindingPipeline(
  rawFindings: RawQualityFinding[],
  ctx: PipelineContext
) {
  const counts = { error: 0, warning: 0, info: 0, total: 0, outsideScope: 0 }
  let outsideRepoCount = 0

  const items: FingerprintItem[] = []

  for (const raw of rawFindings) {
    if (raw.file === '') {
      outsideRepoCount++
      continue // Wait, spec says: `file` empty -> outsideRepoCount. But we still process it? No, usually we skip it. Let's include it but with empty file, or skip?
      // "ngoài repo → file:"" + outsideRepoCount." -> maybe just skip and increment count.
    }

    const inScope = isInScope(raw.file, ctx.scopeFiles)
    
    // Increment pre-trim counts
    counts.total++
    counts[raw.severity]++
    if (!inScope) {
      counts.outsideScope++
    }

    const normMsg = truncateStringUTF8(normalizeMessage(raw.message, ctx), MESSAGE_MAX_BYTES)
    let fixHint = raw.evidence?.fixHint ? truncateStringUTF8(normalizeMessage(raw.evidence.fixHint, ctx), FIX_HINT_MAX_BYTES) : undefined

    let anchor: string
    if (raw.anchorOverride !== undefined) {
      anchor = raw.anchorOverride || ''
    } else if (raw.testAnchor) {
      anchor = raw.testAnchor
    } else {
      const sourceLine = await ctx.readSourceLine(raw.file, raw.line)
      anchor = normalizeAnchor(sourceLine)
    }

    items.push({
      ...raw,
      tool: ctx.tool,
      anchor,
      normMessage: normMsg,
      fixHint
    } as FingerprintItem & { fixHint?: string })
  }

  const withOccurrences = assignOccurrences(items)
  
  let findings: QualityFinding[] = withOccurrences.map(item => {
    const { fingerprint, fpVersion } = computeFingerprint(item, item.occurrence)
    const inScope = isInScope(item.file, ctx.scopeFiles)
    
    return {
      fingerprint,
      fpVersion,
      ruleId: item.ruleId,
      severity: item.severity,
      category: item.category,
      file: item.file,
      line: item.line,
      column: item.column,
      endLine: item.endLine,
      endColumn: item.endColumn,
      message: item.normMessage, // Use normalized message
      tool: ctx.tool,
      toolVersion: ctx.toolVersion,
      stepId: ctx.stepId,
      inScope,
      fixHint: (item as any).fixHint
    }
  })

  // Sort: error > warning > info, inScope > outOfScope, file, line, column, ruleId
  findings.sort((a, b) => {
    if (SEVERITY_WEIGHT[a.severity] !== SEVERITY_WEIGHT[b.severity]) {
      return SEVERITY_WEIGHT[a.severity] - SEVERITY_WEIGHT[b.severity]
    }
    if (a.inScope !== b.inScope) {
      return a.inScope ? -1 : 1
    }
    if (a.file !== b.file) {
      return a.file.localeCompare(b.file)
    }
    if (a.line !== b.line) return a.line - b.line
    if (a.column !== b.column) return a.column - b.column
    return a.ruleId.localeCompare(b.ruleId)
  })

  let truncated = false
  const budget = Math.min(MAX_FINDINGS_PER_STEP, ctx.remainingRunBudget)
  if (findings.length > budget) {
    findings = findings.slice(0, budget)
    truncated = true
  }

  return { findings, counts, truncated, outsideRepoCount }
}

export function inferStepStatus(opts: {
  exitCode: number
  hasFindings: boolean
  failureKind: string | null
  envReason?: string
  skipDriftGuard?: boolean
  findingsExitCodes: number[]
}) {
  if (opts.failureKind === 'env') {
    return { status: 'env_not_ready', failureKind: 'env', envReason: opts.envReason }
  }
  if (opts.failureKind) {
    return { status: 'failed', failureKind: opts.failureKind }
  }

  const inFindingsExit = opts.findingsExitCodes.includes(opts.exitCode)
  const isOk = opts.exitCode === 0

  if (inFindingsExit && !opts.hasFindings && !opts.skipDriftGuard && opts.exitCode !== 0) {
    return { status: 'failed', failureKind: 'format_drift' }
  }

  if (inFindingsExit && opts.hasFindings) {
    return { status: 'findings' }
  }

  if (isOk) {
    // If ok but has findings, and format drift didn't trigger, it could just be findings.
    // However, some tools exit 0 but emit findings (like go vet). 
    if (opts.hasFindings) return { status: 'findings' }
    return { status: 'passed' }
  }

  // Not 0, not in findings exit codes -> unexpected
  return { status: 'failed', failureKind: 'exit_unexpected' }
}
