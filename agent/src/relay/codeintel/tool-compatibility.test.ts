import { describe, it, expect, vi } from 'vitest'
import fs from 'fs'
import path from 'path'
import {
  classifyToolCompatibility,
  SUPPORTED_TOOL_VERSIONS,
  SUPPORTED_SCHEMA_MARKERS
} from '../codeintel-tool-detection'
import { CodeIntelError } from '../codeintel-errors'

export function verifyEverySupportedVersionHasFixtures(
  supportedVersions: Record<'gitnexus' | 'codegraph', string[]>,
  fixturesRoot = path.join(__dirname, '__fixtures__')
) {
  for (const [tool, versions] of Object.entries(supportedVersions) as Array<['gitnexus' | 'codegraph', string[]]>) {
    for (const ver of versions) {
      const dir = path.join(fixturesRoot, tool, ver)
      if (!fs.existsSync(dir)) {
        throw new Error(`Missing fixture directory for ${tool}@${ver}: ${dir}`)
      }
      const manifestPath = path.join(dir, 'MANIFEST.json')
      if (!fs.existsSync(manifestPath)) {
        throw new Error(`Missing MANIFEST.json for ${tool}@${ver}: ${manifestPath}`)
      }
      const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
      if (manifest.tool !== tool || manifest.version !== ver) {
        throw new Error(`Manifest tool/version mismatch in ${manifestPath}`)
      }
    }
  }
}

describe('tool-compatibility', () => {
  it('classifies versions in the table correctly', () => {
    // 1.6.9 verified
    expect(classifyToolCompatibility('gitnexus', '1.6.9')).toEqual({
      state: 'verified',
      supported: true,
      warnings: [],
      reindexRecommended: false
    })

    // 1.6.10 and 1.7.0 untested
    const r1 = classifyToolCompatibility('gitnexus', '1.6.10')
    expect(r1.state).toBe('untested')
    expect(r1.supported).toBe(true)
    expect(r1.warnings).toContain('tool_version_untested')

    const r2 = classifyToolCompatibility('gitnexus', '1.7.0')
    expect(r2.state).toBe('untested')
    expect(r2.supported).toBe(true)
    expect(r2.warnings).toContain('tool_version_untested')

    // 2.0.0 incompatible
    expect(classifyToolCompatibility('gitnexus', '2.0.0').state).toBe('incompatible')
    expect(classifyToolCompatibility('gitnexus', '2.0.0').supported).toBe(false)

    // 1.5.9 incompatible
    expect(classifyToolCompatibility('gitnexus', '1.5.9').state).toBe('incompatible')
    expect(classifyToolCompatibility('gitnexus', '1.5.9').supported).toBe(false)

    // unknown marker -> incompatible
    expect(classifyToolCompatibility('gitnexus', '1.6.9', { schemaVersion: 99 }).state).toBe('incompatible')

    // reindexRecommended retains state and adds warning
    const r3 = classifyToolCompatibility('gitnexus', '1.6.9', { schemaVersion: 5, reindexRecommended: true })
    expect(r3.state).toBe('verified')
    expect(r3.reindexRecommended).toBe(true)
    expect(r3.warnings).toContain('index_built_with_old_extraction')
  })

  it('TestEverySupportedVersionHasFixtures: passes for real supported versions', () => {
    expect(() => verifyEverySupportedVersionHasFixtures(SUPPORTED_TOOL_VERSIONS)).not.toThrow()
  })

  it('TestEverySupportedVersionHasFixtures: fails when injecting an unsupported fake version', () => {
    const fakeVersions = {
      ...SUPPORTED_TOOL_VERSIONS,
      gitnexus: [...SUPPORTED_TOOL_VERSIONS.gitnexus, '9.9.9']
    }
    expect(() => verifyEverySupportedVersionHasFixtures(fakeVersions)).toThrow(/Missing fixture directory/)
  })

  it('shape guard: format drift error does not leak raw output', () => {
    const rawOutput = 'SECRET_OUTPUT_PAYLOAD_HERE'
    const error = new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Format drift occurred', {
      tool: 'gitnexus',
      version: '1.6.9',
      command: 'context',
      reason: 'format_drift'
    })

    expect(error.message).not.toContain(rawOutput)
    expect(error.data.tool).toBe('gitnexus')
    expect(error.data.version).toBe('1.6.9')
    expect(error.data.command).toBe('context')
    expect(error.data.reason).toBe('format_drift')
  })
})
