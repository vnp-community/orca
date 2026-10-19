import { mkdtempSync, writeFileSync } from 'node:fs'
import { homedir, tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { resolveUsablePtyCwd, validatePtyCwd } from './pty-handler'
import { usableCwd } from './pty-agent-bridge'

// BUG-QA-01: a terminal asked to start in a directory must say when it could not, instead of silently opening $HOME.
describe('pty cwd', () => {
  const dir = mkdtempSync(join(tmpdir(), 'pty-cwd-'))
  const file = join(dir, 'a-file')
  writeFileSync(file, 'x')

  it('uses a directory that exists', () => {
    expect(resolveUsablePtyCwd(dir)).toBe(dir)
    expect(usableCwd(dir)).toBe(dir)
    expect(validatePtyCwd(dir)).toBe(dir)
  })

  it.each([
    ['empty', ''],
    ['missing', join(dir, 'nope')],
    ['a file', file],
    ['a NUL byte', `${dir}\0/x`]
  ])('cannot use %s: reports null and the home directory is opened', (_name, bad) => {
    expect(resolveUsablePtyCwd(bad)).toBeNull()
    expect(usableCwd(bad)).toBeNull()
    expect(validatePtyCwd(bad)).toBe(homedir())
  })
})
