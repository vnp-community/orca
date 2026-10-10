import { chmodSync, mkdirSync, mkdtempSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { ensureNodePtySpawnHelperExecutable } from './node-pty-spawn-helper-permissions'

describe.skipIf(process.platform === 'win32')('ensureNodePtySpawnHelperExecutable', () => {
  let dir: string
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'node-pty-helper-'))
  })
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true })
  })

  function writeHelper(relative: string, mode: number): string {
    const path = join(dir, relative)
    mkdirSync(join(path, '..'), { recursive: true })
    writeFileSync(path, '#!/bin/sh\n')
    chmodSync(path, mode)
    return path
  }

  it('restores the exec bit on a prebuilt helper that lost it', () => {
    const helper = writeHelper('prebuilds/darwin-arm64/spawn-helper', 0o644)
    const repaired = ensureNodePtySpawnHelperExecutable({
      platform: 'darwin',
      arch: 'arm64',
      nodePtyDir: dir
    })
    expect(repaired).toEqual([helper])
    expect(statSync(helper).mode & 0o111).toBe(0o111)
  })

  it('leaves an already executable helper and other architectures alone', () => {
    const good = writeHelper('prebuilds/darwin-arm64/spawn-helper', 0o755)
    const other = writeHelper('prebuilds/darwin-x64/spawn-helper', 0o644)
    const repaired = ensureNodePtySpawnHelperExecutable({
      platform: 'darwin',
      arch: 'arm64',
      nodePtyDir: dir
    })
    expect(repaired).toEqual([])
    expect(statSync(good).mode & 0o777).toBe(0o755)
    expect(statSync(other).mode & 0o777).toBe(0o644)
  })

  it('repairs a locally built helper too', () => {
    const helper = writeHelper('build/Release/spawn-helper', 0o600)
    expect(
      ensureNodePtySpawnHelperExecutable({ platform: 'linux', arch: 'x64', nodePtyDir: dir })
    ).toEqual([helper])
  })

  it('does nothing on Windows or when node-pty is not installed', () => {
    writeHelper('prebuilds/win32-x64/spawn-helper', 0o644)
    expect(
      ensureNodePtySpawnHelperExecutable({ platform: 'win32', arch: 'x64', nodePtyDir: dir })
    ).toEqual([])
    expect(ensureNodePtySpawnHelperExecutable({ nodePtyDir: null })).toEqual([])
  })

  it('does not throw when the install is missing the helper entirely', () => {
    expect(
      ensureNodePtySpawnHelperExecutable({ platform: 'darwin', arch: 'arm64', nodePtyDir: dir })
    ).toEqual([])
  })
})
