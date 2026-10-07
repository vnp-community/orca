import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import { checkCodeIntelRelayParity, isCodeIntelCoreFile } from './check-codeintel-relay-parity.mjs'

describe('check-codeintel-relay-parity', () => {
  let tmpAgent
  let tmpDesktop

  beforeEach(() => {
    tmpAgent = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-parity-'))
    tmpDesktop = fs.mkdtempSync(path.join(os.tmpdir(), 'desktop-parity-'))
  })

  afterEach(() => {
    fs.rmSync(tmpAgent, { recursive: true, force: true })
    fs.rmSync(tmpDesktop, { recursive: true, force: true })
  })

  it('filters core files correctly', () => {
    expect(isCodeIntelCoreFile('codeintel-errors.ts')).toBe(true)
    expect(isCodeIntelCoreFile('gitnexus-query.ts')).toBe(true)
    expect(isCodeIntelCoreFile('codegraph-reader.ts')).toBe(true)
    expect(isCodeIntelCoreFile('codeintel-errors.test.ts')).toBe(false)
    expect(isCodeIntelCoreFile('agent-rpc-dispatch-codeintel.ts')).toBe(false)
    expect(isCodeIntelCoreFile('other.ts')).toBe(false)
  })

  it('passes when files are identical across both directories', () => {
    fs.writeFileSync(path.join(tmpAgent, 'codeintel-foo.ts'), 'export const x = 1\n')
    fs.writeFileSync(path.join(tmpDesktop, 'codeintel-foo.ts'), 'export const x = 1\n')

    const res = checkCodeIntelRelayParity(tmpAgent, tmpDesktop)
    expect(res.checkedCount).toBe(1)
    expect(res.mismatches).toHaveLength(0)
  })

  it('fails when a core file is missing in desktop relay', () => {
    fs.writeFileSync(path.join(tmpAgent, 'codeintel-bar.ts'), 'export const bar = 2\n')

    const res = checkCodeIntelRelayParity(tmpAgent, tmpDesktop)
    expect(res.mismatches).toHaveLength(1)
    expect(res.mismatches[0]).toEqual({
      file: 'codeintel-bar.ts',
      issue: 'missing in desktop relay'
    })
  })

  it('fails when content differs between agent and desktop relay', () => {
    fs.writeFileSync(path.join(tmpAgent, 'codeintel-baz.ts'), 'export const baz = 1\n')
    fs.writeFileSync(path.join(tmpDesktop, 'codeintel-baz.ts'), 'export const baz = 2\n')

    const res = checkCodeIntelRelayParity(tmpAgent, tmpDesktop)
    expect(res.mismatches).toHaveLength(1)
    expect(res.mismatches[0]).toEqual({
      file: 'codeintel-baz.ts',
      issue: 'content mismatch'
    })
  })
})
