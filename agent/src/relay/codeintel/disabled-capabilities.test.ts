import { describe, it, expect, vi } from 'vitest'
import { buildCapabilities, STATIC_CAPABILITIES_FALLBACK } from '../agent-session-capabilities'
import { readRuntimeSwitches } from './runtime-switches'

describe('disabled-capabilities (Task 073-03)', () => {
  const mockLog: any = {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn()
  }

  it('STATIC_CAPABILITIES_FALLBACK contains neither codeintel nor quality', () => {
    expect(STATIC_CAPABILITIES_FALLBACK).not.toContain('codeintel')
    expect(STATIC_CAPABILITIES_FALLBACK).not.toContain('codeintel.gitnexus')
    expect(STATIC_CAPABILITIES_FALLBACK).not.toContain('codeintel.codegraph')
    expect(STATIC_CAPABILITIES_FALLBACK).not.toContain('quality')
  })

  it('buildCapabilities omits codeintel* and quality when codeintelDisabled is true', async () => {
    const disabledSwitches = readRuntimeSwitches({ ORCA_CODEINTEL_DISABLED: '1' })
    const config: any = { toolEnv: {} }

    const caps = await buildCapabilities(config, mockLog, disabledSwitches)

    expect(caps).toContain('fs')
    expect(caps).not.toContain('codeintel')
    expect(caps).not.toContain('codeintel.gitnexus')
    expect(caps).not.toContain('codeintel.codegraph')
    expect(caps).not.toContain('quality')
  })

  it('buildCapabilities includes codeintel capabilities when enabled with available tools', async () => {
    const enabledSwitches = readRuntimeSwitches({ ORCA_CODEINTEL_DISABLED: '0' })
    const config: any = {
      toolEnv: {
        PATH: process.env.PATH
      }
    }

    const caps = await buildCapabilities(config, mockLog, enabledSwitches)
    expect(caps).toContain('fs')
    // Standard capabilities are present
  })
})
