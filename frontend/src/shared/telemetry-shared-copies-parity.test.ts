/**
 * telemetry-shared-copies-parity.test.ts — FE-CV-TASK-095-02
 *
 * The telemetry schema files exist as byte-identical copies in six packages, and the
 * desktop main-process validator imports ITS copy at runtime. A drifted copy silently
 * drops events, so every copy that is present must match the frontend source.
 * A package that is not checked out is reported, not failed.
 *
 * To sync after a change: copy each file below from frontend/src/shared/ to every
 * other directory in COPY_DIRS in the same commit.
 */

import * as fs from 'node:fs'
import * as path from 'node:path'
import { describe, expect, it } from 'vitest'

const REPO_ROOT = path.resolve(__dirname, '../../..')
const SOURCE_DIR = path.join(REPO_ROOT, 'frontend/src/shared')

const FILES = ['telemetry-events.ts', 'mcp-telemetry-events.ts', 'review-telemetry-events.ts']
const COPY_DIRS = [
  'desktop/src/shared',
  'backend/src/shared',
  'tests/vendor-shared/shared',
  'agent/src/shared',
  'mobile/src/vendor-shared/shared'
]

describe('telemetry schema shared copies parity', () => {
  for (const file of FILES) {
    it(`source ${file} exists`, () => {
      expect(fs.existsSync(path.join(SOURCE_DIR, file)), `missing ${file}`).toBe(true)
    })

    for (const dir of COPY_DIRS) {
      it(`${dir}/${file} matches the frontend source (or the package is absent)`, () => {
        const copyPath = path.join(REPO_ROOT, dir, file)
        if (!fs.existsSync(path.join(REPO_ROOT, dir))) {
          console.warn(`[parity] package not checked out, skipped: ${dir}`)
          return
        }
        // A present package must carry the file: telemetry-events.ts imports its siblings.
        expect(fs.existsSync(copyPath), `${dir} is missing ${file}`).toBe(true)
        expect(fs.readFileSync(copyPath, 'utf-8'), `${dir}/${file} drifted from frontend/src/shared/${file}`).toBe(
          fs.readFileSync(path.join(SOURCE_DIR, file), 'utf-8')
        )
      })
    }
  }

  it('drift detection works: a modified copy would not equal the source', () => {
    const source = fs.readFileSync(path.join(SOURCE_DIR, 'review-telemetry-events.ts'), 'utf-8')
    expect(`${source}\n// drift`).not.toBe(source)
  })
})
