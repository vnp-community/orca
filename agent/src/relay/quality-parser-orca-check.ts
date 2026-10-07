import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'

async function readRaw(input: QualityParserInput): Promise<string | null> {
  let raw = ''
  try {
    raw = await fs.readFile(input.stdoutPath, 'utf8')
  } catch (e: any) {
    return null
  }
  return raw
}

export const maxLinesRatchetParser = {
  key: 'orca-check-max-lines',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    const raw = await readRaw(input)
    if (raw === null) {
      return { findings: [], failure: { kind: 'env', envReason: 'STDOUT_UNREADABLE' }, stats: { scannedFiles: 0 } }
    }

    const lines = raw.split(/\r?\n/)
    const findings: RawQualityFinding[] = []

    for (const line of lines) {
      const matchNew = line.match(/^::error::New max-lines bypass not allowed:\s+(.*)$/)
      const matchStale = line.match(/^::error::Stale max-lines baseline entry \(prune it\):\s+(.*)$/)

      if (matchNew || matchStale) {
        const kind = matchNew ? 'new-bypass' : 'stale-baseline'
        const severity = matchNew ? 'error' : 'warning'
        const entry = (matchNew ? matchNew[1] : matchStale![1]).trim()

        let file = entry
        if (entry.startsWith('inline ')) {
          file = entry.substring('inline '.length).trim()
        } else if (entry === 'mobile-config') {
          file = 'mobile/.oxlintrc.json'
        }

        const mapped = toRepoRelative(file, input.cwd, input.repoRoot, input.platform)

        findings.push({
          ruleId: `orca-check/max-lines-ratchet/${kind}`,
          message: matchNew ? 'New max-lines bypass not allowed' : 'Stale max-lines baseline entry (prune it)',
          file: mapped.outside ? '' : mapped.file,
          line: 1,
          column: 1,
          severity,
          category: 'convention',
          anchorOverride: entry
        })
      }
    }

    return { findings, failure: null, stats: { scannedFiles: 0 } }
  }
}

export const styledScrollbarsParser = {
  key: 'orca-check-styled-scrollbars',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    const raw = await readRaw(input)
    if (raw === null) {
      return { findings: [], failure: { kind: 'env', envReason: 'STDOUT_UNREADABLE' }, stats: { scannedFiles: 0 } }
    }

    const lines = raw.split(/\r?\n/)
    const findings: RawQualityFinding[] = []

    // Skip 4 title lines. We can just skip lines until we see the format
    let i = 0
    while (i < lines.length && i < 4 && !lines[i].includes(':')) {
      i++ // wait, this might be fragile. Let's just match the format everywhere.
    }

    for (const line of lines) {
      const match = line.match(/^([^:]+):(\d+):(\d+)\s+(.*)$/)
      if (match) {
        // Path concatenated with desktop/
        const rawPath = `desktop/${match[1].trim()}`
        const mapped = toRepoRelative(rawPath, input.cwd, input.repoRoot, input.platform)

        findings.push({
          ruleId: 'orca-check/styled-scrollbars/unstyled',
          message: match[4].trim(),
          file: mapped.outside ? '' : mapped.file,
          line: parseInt(match[2], 10) || 1,
          column: parseInt(match[3], 10) || 1,
          severity: 'error',
          category: 'convention'
        })
      }
    }

    return { findings, failure: null, stats: { scannedFiles: 0 } }
  }
}

export const reliabilityGatesParser = {
  key: 'orca-check-reliability-gates',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    const raw = await readRaw(input)
    if (raw === null) {
      return { findings: [], failure: { kind: 'env', envReason: 'STDOUT_UNREADABLE' }, stats: { scannedFiles: 0 } }
    }

    const lines = raw.split(/\r?\n/)
    const findings: RawQualityFinding[] = []

    for (const line of lines) {
      const match = line.match(/^\-\s+([^:]+):\s+(.*)$/)
      if (match) {
        const mapped = toRepoRelative('desktop/config/reliability-gates.jsonc', input.cwd, input.repoRoot, input.platform)
        findings.push({
          ruleId: 'orca-check/reliability-gates/invalid-manifest',
          message: match[2].trim(),
          file: mapped.outside ? '' : mapped.file,
          line: 1,
          column: 1,
          severity: 'error',
          category: 'convention',
          anchorOverride: match[1].trim() // gateId
        })
      }
    }

    return { findings, failure: null, stats: { scannedFiles: 0 } }
  }
}
