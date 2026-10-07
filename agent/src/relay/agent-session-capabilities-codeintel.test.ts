import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { buildCapabilities, STATIC_CAPABILITIES_FALLBACK } from './agent-session-capabilities'
import { AgentConfig } from './agent-config'

vi.mock('./agent-session-capabilities', async (importOriginal) => {
  const actual = await importOriginal<any>()
  return {
    ...actual,
    checkGitAvailable: vi.fn().mockResolvedValue(true),
    checkPtyAvailable: vi.fn().mockResolvedValue(true)
  }
})

describe('agent-session-capabilities codeintel', () => {
  const tmpDir = os.tmpdir()
  let mockConfig: AgentConfig
  let binDir: string
  const originalPlatform = process.platform

  beforeEach(() => {
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

  it('includes codeintel capabilities when binaries exist', async () => {
    const gitnexus = path.join(binDir, 'gitnexus')
    const codegraph = path.join(binDir, 'codegraph')
    fs.writeFileSync(gitnexus, 'mock')
    fs.chmodSync(gitnexus, 0o755)
    fs.writeFileSync(codegraph, 'mock')
    fs.chmodSync(codegraph, 0o755)

    const log = { info: vi.fn() } as any
    const caps = await buildCapabilities(mockConfig, log)

    expect(caps).toContain('codeintel')
    expect(caps).toContain('codeintel.gitnexus')
    expect(caps).toContain('codeintel.codegraph')
    expect(caps).toContain('fs')
    expect(caps).toContain('git')
    expect(caps).toContain('pty')
  })

  it('includes only gitnexus capability when only it exists', async () => {
    const gitnexus = path.join(binDir, 'gitnexus')
    fs.writeFileSync(gitnexus, 'mock')
    fs.chmodSync(gitnexus, 0o755)

    const log = { info: vi.fn() } as any
    const caps = await buildCapabilities(mockConfig, log)

    expect(caps).toContain('codeintel')
    expect(caps).toContain('codeintel.gitnexus')
    expect(caps).not.toContain('codeintel.codegraph')
  })

  it('does not include codeintel capabilities when binaries do not exist', async () => {
    const log = { info: vi.fn() } as any
    const caps = await buildCapabilities(mockConfig, log)

    expect(caps).not.toContain('codeintel')
    expect(caps).not.toContain('codeintel.gitnexus')
    expect(caps).not.toContain('codeintel.codegraph')
  })

  it('STATIC_CAPABILITIES_FALLBACK does not contain codeintel', () => {
    expect((STATIC_CAPABILITIES_FALLBACK as any)).not.toContain('codeintel')
    expect((STATIC_CAPABILITIES_FALLBACK as any)).not.toContain('codeintel.gitnexus')
    expect((STATIC_CAPABILITIES_FALLBACK as any)).not.toContain('codeintel.codegraph')
  })
})
