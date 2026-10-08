import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { reviewEventSchemas } from './review-telemetry-events'

const SRC = join(import.meta.dirname, '..')
const read = (rel: string): string => readFileSync(join(SRC, rel), 'utf-8')

describe('review telemetry privacy', () => {
  it('schemas contain only enums and booleans (no free-form strings or numbers)', () => {
    const source = read('shared/review-telemetry-events.ts')
    expect(source).not.toMatch(/z\.string\(|z\.number\(|z\.any\(|z\.unknown\(|z\.record\(/)
  })

  it('every object schema is strict (extra keys rejected)', () => {
    const source = read('shared/review-telemetry-events.ts')
    const objects = source.match(/z\s*\.object\(/g)?.length ?? 0
    const stricts = source.match(/\.strict\(\)/g)?.length ?? 0
    expect(objects).toBe(Object.keys(reviewEventSchemas).length)
    expect(stricts).toBe(objects)
  })

  it('the wrapper module imports no identifying types and does not track from components', () => {
    const wrappers = read('renderer/src/lib/review-telemetry.ts')
    const imports = [...wrappers.matchAll(/from '([^']+)'/g)].map((m) => m[1])
    expect(imports.sort()).toEqual(
      ['../../../shared/review-telemetry-events', '../../../shared/telemetry-events', './telemetry'].sort()
    )
  })

  it('components never call track() for review events directly', () => {
    const files = [
      'renderer/src/components/review-map/report/ReviewReportMenu.tsx',
      'renderer/src/components/review-map/report/use-insert-review-report.ts',
      'renderer/src/components/review-map/ai-summary/use-review-ai-summary.ts',
      'renderer/src/components/right-sidebar/use-source-control-quality-gate.ts'
    ]
    for (const file of files) {
      expect(read(file), file).not.toMatch(/from '[^']*\/lib\/telemetry'|\btrack\(/)
    }
  })

  it('decision tracker holds state in memory only', () => {
    const tracker = read('renderer/src/lib/review-decision-tracker.ts')
    expect(tracker).not.toMatch(/localStorage|sessionStorage|indexedDB|writeFile/)
  })
})
