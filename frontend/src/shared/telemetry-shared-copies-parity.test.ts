/**
 * telemetry-shared-copies-parity.test.ts — FE-CV-TASK-095-02
 *
 * Verifies that all shared copies of telemetry-events.ts are byte-identical.
 * If any package is missing, the test reports the path but does not fail
 * (the package may legitimately not be checked out in CI).
 * However, if a package IS present and differs, the test fails.
 *
 * To sync copies: copy frontend/src/shared/telemetry-events.ts to all
 * other paths listed in SHARED_PATHS below and commit together.
 */

import { describe, it, expect } from 'vitest'
import * as fs from 'node:fs'
import * as path from 'node:path'

// Root of the monorepo — navigate up from the test file
const REPO_ROOT = path.resolve(__dirname, '../../../../../..')

const SOURCE_PATH = path.join(REPO_ROOT, 'frontend/src/shared/telemetry-events.ts')

/**
 * Other packages that must carry an identical copy.
 * These are the 5 non-frontend copies mentioned in FE-CV-SOL-095 §1.
 */
const SHARED_PATHS = [
  'desktop/src/shared/telemetry-events.ts',
  'backend/src/shared/telemetry-events.ts',
  'tests/vendor-shared/shared/telemetry-events.ts',
  'agent/src/shared/telemetry-events.ts',
  'mobile/src/vendor-shared/shared/telemetry-events.ts',
]

describe('telemetry-events.ts shared copies parity', () => {
  const sourceContent = (() => {
    try {
      return fs.readFileSync(SOURCE_PATH, 'utf-8')
    } catch {
      return null
    }
  })()

  it('source file exists', () => {
    expect(sourceContent, `Source not found: ${SOURCE_PATH}`).not.toBeNull()
  })

  for (const relPath of SHARED_PATHS) {
    const absPath = path.join(REPO_ROOT, relPath)

    it(`${relPath} matches source (or is absent from checkout)`, () => {
      if (!fs.existsSync(absPath)) {
        // Package not checked out — skip but report
        console.warn(`[parity] Skipped (not present): ${absPath}`)
        return
      }

      const copyContent = fs.readFileSync(absPath, 'utf-8')
      expect(copyContent, `${relPath} differs from frontend/src/shared/telemetry-events.ts`).toBe(
        sourceContent
      )
    })
  }
})
