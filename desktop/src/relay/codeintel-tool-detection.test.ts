import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import {
  hasCodeIntelBinary,
  detectCodeIntelTools,
  resetCodeIntelToolDetectionCache
} from './codeintel-tool-detection'
import { AgentConfig } from './agent-config'

describe('codeintel-tool-detection', () => {
  const tmpDir = os.tmpdir()
  let mockConfig: AgentConfig
  let binDir: string

  const originalPlatform = process.platform

  beforeEach(() => {
    resetCodeIntelToolDetectionCache()
    binDir = fs.mkdtempSync(path.join(tmpDir, 'orca-bin-'))
    mockConfig = {
      toolPath: binDir
    } as any
    Object.defineProperty(process, 'platform', { value: 'linux' })
  })

  afterEach(() => {
    try { fs.rmSync(binDir, { recursive: true, force: true }) } catch {}
    Object.defineProperty(process, 'platform', { value: originalPlatform })
  })

  it('hasCodeIntelBinary returns true if binary exists and is executable', async () => {
    const binPath = path.join(binDir, 'gitnexus')
    fs.writeFileSync(binPath, 'mock')
    fs.chmodSync(binPath, 0o755)

    expect(await hasCodeIntelBinary(mockConfig, 'gitnexus')).toBe(true)
    expect(await hasCodeIntelBinary(mockConfig, 'codegraph')).toBe(false)
  })

  it('detectCodeIntelTools returns unsupportedPlatform for win32', async () => {
    Object.defineProperty(process, 'platform', { value: 'win32' })
    const res = await detectCodeIntelTools(mockConfig)
    expect(res.unsupportedPlatform).toBe(true)
    expect(res.gitnexus.unsupportedPlatform).toBe(true)
  })

  it('detectCodeIntelTools returns supported for valid versions', async () => {
    const binPath = path.join(binDir, 'gitnexus')
    fs.writeFileSync(binPath, '#!/bin/sh\necho "gitnexus version 1.6.9"')
    fs.chmodSync(binPath, 0o755)

    const binPathCg = path.join(binDir, 'codegraph')
    fs.writeFileSync(binPathCg, '#!/bin/sh\necho "codegraph version 1.4.1"')
    fs.chmodSync(binPathCg, 0o755)

    const res = await detectCodeIntelTools(mockConfig)
    expect(res.gitnexus.available).toBe(true)
    expect(res.gitnexus.supported).toBe(true)
    expect(res.gitnexus.version).toBe('1.6.9')

    expect(res.codegraph.available).toBe(true)
    expect(res.codegraph.supported).toBe(true)
    expect(res.codegraph.version).toBe('1.4.1')
  })

  it('detectCodeIntelTools returns unsupported for old versions', async () => {
    const binPath = path.join(binDir, 'gitnexus')
    fs.writeFileSync(binPath, '#!/bin/sh\necho "gitnexus version 1.3.0"')
    fs.chmodSync(binPath, 0o755)

    const res = await detectCodeIntelTools(mockConfig)
    expect(res.gitnexus.available).toBe(true)
    expect(res.gitnexus.supported).toBe(false)
    expect(res.gitnexus.version).toBe('1.3.0')
  })

  it('caches the report', async () => {
    const binPath = path.join(binDir, 'gitnexus')
    fs.writeFileSync(binPath, '#!/bin/sh\necho "gitnexus version 1.6.9"')
    fs.chmodSync(binPath, 0o755)

    const res1 = await detectCodeIntelTools(mockConfig)
    fs.unlinkSync(binPath)
    const res2 = await detectCodeIntelTools(mockConfig)

    expect(res2).toBe(res1)
  })
})
