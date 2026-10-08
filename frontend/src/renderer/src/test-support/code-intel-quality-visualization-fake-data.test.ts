import { describe, expect, it } from 'vitest'
import { createFakeCodeIntelBackend } from './code-intel-fake-backend'
import {
  buildCoverageReport,
  buildStructureGraph,
  registerQualityVisualizationScenario
} from './code-intel-quality-visualization-fake-data'

const SELECTOR = { projectId: 'project-1', worktreeId: 'worktree-1' }

function backend() {
  const fake = createFakeCodeIntelBackend()
  fake.setSettings({ codeIntelEnabled: true, qualityGateEnabled: true })
  return fake
}

describe('quality visualization fake data', () => {
  it('serves ready trend, coverage, hotspot and structure data', async () => {
    const fake = backend()
    registerQualityVisualizationScenario(fake, {
      trend: { state: 'ready', points: 50 },
      coverage: { state: 'ready', kind: 'estimated' },
      hotspot: { state: 'ready' },
      structure: { state: 'ready', kind: 'large' }
    })
    const trend = (await fake.call('codeIntel.quality.trend', { ...SELECTOR })) as {
      points: unknown[]
    }
    expect(trend.points).toHaveLength(50)
    const coverage = (await fake.call('codeIntel.quality.coverage', { ...SELECTOR })) as {
      report: { source: string }
    }
    expect(coverage.report.source).toBe('estimated')
    const hotspots = (await fake.call('codeIntel.findings', {
      ...SELECTOR,
      rules: ['hotspot.file']
    })) as { findings: unknown[] }
    expect(hotspots.findings.length).toBeGreaterThan(0)
    const other = (await fake.call('codeIntel.findings', { ...SELECTOR })) as {
      findings: unknown[]
    }
    expect(other.findings).toEqual([])
    const structure = (await fake.call('codeIntel.structure', { ...SELECTOR, depth: 2 })) as {
      truncated: boolean
      totalCount: number
    }
    expect(structure).toMatchObject({ truncated: true, totalCount: 400 })
  })

  it('serves empty states, including report:null with a reason', async () => {
    const fake = backend()
    registerQualityVisualizationScenario(fake, {
      trend: { state: 'empty' },
      coverage: { state: 'empty' },
      hotspot: { state: 'empty' },
      structure: { state: 'empty' }
    })
    expect(await fake.call('codeIntel.quality.coverage', { ...SELECTOR })).toMatchObject({
      report: null,
      reason: expect.any(String)
    })
    expect(await fake.call('codeIntel.quality.trend', { ...SELECTOR })).toMatchObject({
      points: [],
      totalCount: 0
    })
  })

  it('fails with TIMEOUT {inProgress}, OUTPUT_TOO_LARGE and generic errors', async () => {
    const fake = backend()
    registerQualityVisualizationScenario(fake, {
      trend: { state: 'error', failure: 'timeout-in-progress' },
      coverage: { state: 'error', failure: 'output-too-large' },
      structure: { state: 'error' }
    })
    await expect(fake.call('codeIntel.quality.trend', { ...SELECTOR })).rejects.toThrow(
      /CODEINTEL_TIMEOUT.*"inProgress":true/
    )
    await expect(fake.call('codeIntel.quality.coverage', { ...SELECTOR })).rejects.toThrow(
      /CODEINTEL_OUTPUT_TOO_LARGE/
    )
    await expect(fake.call('codeIntel.structure', { ...SELECTOR, depth: 2 })).rejects.toThrow(
      /CODEINTEL_TOOL_FAILED/
    )
  })

  it('stale answers once and then fails, loading never settles', async () => {
    const fake = backend()
    registerQualityVisualizationScenario(fake, {
      trend: { state: 'stale' },
      coverage: { state: 'loading' }
    })
    await expect(fake.call('codeIntel.quality.trend', { ...SELECTOR })).resolves.toBeDefined()
    await expect(fake.call('codeIntel.quality.trend', { ...SELECTOR })).rejects.toThrow()
    const pending = fake.call('codeIntel.quality.coverage', { ...SELECTOR })
    const settled = await Promise.race([
      pending.then(() => 'settled'),
      new Promise((r) => setTimeout(() => r('pending'), 20))
    ])
    expect(settled).toBe('pending')
  })

  it('builds coverage in the D5 unit (ratio 0..1) and a cyclic graph', () => {
    const report = buildCoverageReport()
    expect(report.diff?.diffCoverage).toBeLessThanOrEqual(1)
    expect(report.files.every((f) => f.pct >= 0 && f.pct <= 1)).toBe(true)
    expect(
      buildStructureGraph('cycle').edges.some((e) => e.from === 'src/mod-3' && e.to === 'src/mod-1')
    ).toBe(true)
  })
})
